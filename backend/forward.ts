import type { ServerWebSocket } from "bun";
import { decode } from "cbor-x";
import type { ServerToWorkerObjectDetectionMessage, WorkerToServerMessage } from "~/shared";
import type { WsClient } from "./WsClient";

export const createForwardFunction = (opts: {
    clients: Map<ServerWebSocket, WsClient>,
    worker_object_detection: () => Worker,
}) => (msg: MessageEvent) => {
    // Broadcast to all clients
    const encoded = msg.data;
    const decoded = decode(encoded) as WorkerToServerMessage;

    if (decoded.type === 'codec' || decoded.type === 'frame' || decoded.type === 'object_detection') {
        // Forward to clients
        for (const [, client] of opts.clients) {
            client.send(decoded);
        }
    }

    if (decoded.type === 'frame_file') {
        const msg: ServerToWorkerObjectDetectionMessage = decoded;
        // Forward to object detection worker
        opts.worker_object_detection().postMessage(msg);
    }
}
