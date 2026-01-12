import asyncio
import requests
from datetime import datetime, timezone
from typing import Callable, Awaitable
from PIL import Image
from io import BytesIO


async def download_frame(http_url: str, worker_key: str, frame_uuid: str) -> bytes | None:
    """Download a frame from the relay, return JPEG bytes"""
    try:
        response = requests.get(
            f"{http_url}/worker/frames/{frame_uuid}",
            headers={"X-Worker-Key": worker_key},
            timeout=10
        )
        if response.status_code == 200:
            return response.content
        else:
            print(f"[frame_batch_event] Failed to download {frame_uuid}: {response.status_code}")
            return None
    except Exception as e:
        print(f"[frame_batch_event] Error downloading {frame_uuid}: {e}")
        return None


async def process_frame_batch_event(
    event: dict,
    http_url: str,
    worker_key: str,
    model,
    processor
) -> dict:
    """
    Process a frame batch event as video input for Qwen3-VL.

    Args:
        event: The frame_batch event dict
        http_url: Base URL for frame downloads
        worker_key: Worker authentication key
        model: Qwen3-VL model instance (required)
        processor: Qwen3-VL processor (required)

    Returns:
        Summary event dict
    """
    frames = event.get('frames', [])
    service_id = event.get('service_id')
    metadata = event.get('metadata', {})

    print(f"\n[frame_batch_event] Processing batch: {len(frames)} frames from {service_id}")

    # Download all frames and convert to PIL Images
    pil_images = []
    for frame_uuid in frames:
        jpeg_data = await download_frame(http_url, worker_key, frame_uuid)
        if jpeg_data:
            pil_image = Image.open(BytesIO(jpeg_data)).convert("RGB")
            pil_images.append(pil_image)

    if not pil_images:
        raise ValueError("No frames downloaded")

    from qwen_vl_utils import process_vision_info

    # Get timing info from metadata
    duration_seconds = metadata.get('duration_seconds', 0)
    start_time = metadata.get('start_time', 0)  # Offset in seconds from video start
    num_frames = len(pil_images)

    # Read fps from metadata (provided by relay)
    fps = metadata.get('fps', 2.0)

    # Build temporal context text
    if start_time > 0:
        end_time = start_time + duration_seconds
        temporal_context = f"This video segment spans from {start_time:.1f}s to {end_time:.1f}s. "
        print(f"[frame_batch_event] Using FPS={fps:.3f} for {num_frames} frames from {start_time:.1f}s to {end_time:.1f}s")
    else:
        temporal_context = ""
        print(f"[frame_batch_event] Using FPS={fps:.3f} for {num_frames} frames over {duration_seconds:.1f}s")

    # Construct video message (produces <|video_pad|> tokens)
    messages = [{
        "role": "user",
        "content": [
            {
                "type": "video",
                "video": pil_images,  # Pass all frames as video
                "raw_fps": fps,  # FPS from metadata
            },
            {
                "type": "text",
                "text": f"{temporal_context}Analyze this video sequence. Describe any events, activities, or changes that occur."
            }
        ]
    }]

    # Process vision inputs
    image_inputs, video_inputs = process_vision_info(messages)

    # Apply chat template
    text = processor.apply_chat_template(
        messages,
        tokenize=False,
        add_generation_prompt=True
    )

    # Tokenize
    inputs = processor(
        text=[text],
        images=image_inputs,
        videos=video_inputs,
        padding=True,
        return_tensors="pt"
    )

    # Move to device
    inputs = inputs.to(model.device)

    # Generate
    print(f"[frame_batch_event] Running inference on {len(pil_images)} frames...")
    generated_ids = model.generate(
        **inputs,
        max_new_tokens=512
    )

    # Decode output
    output_text = processor.batch_decode(
        generated_ids,
        skip_special_tokens=True,
        clean_up_tokenization_spaces=False
    )[0]

    summary = {
        "summary": f"Video analysis: {output_text}",
        "service_id": service_id,
        "frame_count": len(pil_images),
        "duration_seconds": metadata.get('duration_seconds', 0),
        "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    }

    print(f"[frame_batch_event] Summary: {summary}")
    return summary
