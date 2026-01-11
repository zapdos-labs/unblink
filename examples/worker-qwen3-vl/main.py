import asyncio
import json
import websockets
import requests
from events.process_frame_batch_event import process_frame_batch_event


class CVWorker:
    def __init__(self, relay_url: str = "ws://localhost:7010/connect", http_url: str = "http://localhost:7010"):
        self.relay_url = relay_url
        self.http_url = http_url
        self.worker_id = None  # Assigned by relay
        self.worker_key = None
        self.ws = None

        # Model and processor (optional)
        self.model = None
        self.processor = None

    def load_model(self, model_name: str = "Qwen/Qwen3-VL-4B-Instruct"):
        """Load Qwen3-VL model for video inference"""
        print(f"[Worker] Loading model: {model_name}")

        from transformers import Qwen2VLForConditionalGeneration, AutoProcessor
        import torch

        self.processor = AutoProcessor.from_pretrained(model_name)
        self.model = Qwen2VLForConditionalGeneration.from_pretrained(
            model_name,
            torch_dtype=torch.bfloat16,
            device_map="auto"
        )
        print(f"[Worker] Model loaded on device: {self.model.device}")

    async def connect(self):
        print(f"[Worker] Connecting to {self.relay_url}")
        self.ws = await websockets.connect(self.relay_url)

    async def register(self):
        registration_msg = {
            "type": "register",
            "data": {}
        }
        await self.ws.send(json.dumps(registration_msg))
        response = await self.ws.recv()
        data = json.loads(response)
        if data.get("type") == "registered":
            self.worker_id = data["data"]["worker_id"]
            self.worker_key = data["data"]["key"]
            print(f"[Worker] Registered: {self.worker_id}")
            print(f"[Worker] Key: {self.worker_key[:16]}...")
        else:
            print(f"[Worker] Registered: {response}")

    async def listen(self):
        print(f"[Worker] Listening for events...")
        try:
            async for message in self.ws:
                data = json.loads(message)
                print(json.dumps(data, indent=2))

                event_type = data.get("type")
                if event_type == "frame_batch" and self.worker_key:
                    event = data.get("data")
                    if event:
                        # Pass model and processor to enable inference
                        summary = await process_frame_batch_event(
                            event,
                            self.http_url,
                            self.worker_key,
                            model=self.model,
                            processor=self.processor
                        )
                        await self.emit_event(summary)


        except websockets.exceptions.ConnectionClosed:
            print(f"[Worker] Connection closed")

    async def emit_event(self, event_data: dict):
        """Emit event back to relay via HTTP POST"""
        try:
            response = requests.post(
                f"{self.http_url}/events",
                json=event_data,
                headers={
                    "Content-Type": "application/json",
                    "X-Worker-Key": self.worker_key
                },
                timeout=10
            )
            if response.status_code == 200:
                print(f"[Worker] Event emitted: {event_data}")
            else:
                print(f"[Worker] Emit failed: {response.status_code}")
        except Exception as e:
            print(f"[Worker] Emit error: {e}")

    async def run(self):
        try:
            await self.connect()
            await self.register()
            await self.listen()
        except KeyboardInterrupt:
            print("\n[Worker] Shutting down...")
        finally:
            if self.ws:
                await self.ws.close()


def main():
    print("="*60)
    print("CV Worker - Qwen3-VL Video Processor")
    print("="*60 + "\n")

    worker = CVWorker()

    # Uncomment to enable model inference:
    # worker.load_model("Qwen/Qwen2-VL-2B-Instruct")

    asyncio.run(worker.run())


if __name__ == "__main__":
    main()
