import type { ServerWebSocket } from "bun";
import { decode } from "cbor-x";
import type { EngineToServer, ServerToEngine, ServerToWorkerObjectDetectionMessage, WorkerToServerMessage } from "~/shared";
import type { WsClient } from "./WsClient";
import type { Conn } from "~/shared/Conn";
import fs from "fs/promises";
import { addMediaUnit, table_media_units } from "./database";

export const createForwardFunction = (opts: {
    clients: Map<ServerWebSocket, WsClient>,
    worker_object_detection: () => Worker,
    settings: () => Record<string, string>,
    engine_conn: () => Conn<ServerToEngine, EngineToServer>,
}) => {
    const state = {
        last_engine_sent: 0,
    }

    return async (msg: MessageEvent) => {
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
            (async () => {
                const now = Date.now();
                // Throttle engine forwarding to 1 frame every 5 seconds
                if (now - state.last_engine_sent < 5000) {
                    return;
                }
                state.last_engine_sent = now;

                // Store in database
                addMediaUnit({
                    id: decoded.frame_id,
                    type: 'frame',
                    at_time: new Date(),
                    description: null,
                    embedding: null,
                    media_id: decoded.stream_id,
                    path: decoded.path,
                })

                // Forward to AI engine for 
                // 1. Compute embedding  
                // 2. VLM inference
                const engine_conn = opts.engine_conn();

                // Read the frame binary from the file
                const frame = await fs.readFile(decoded.path);
                const msg: ServerToEngine = {
                    type: "frame_binary",
                    stream_id: decoded.stream_id,
                    frame_id: decoded.frame_id,
                    frame,
                }
                engine_conn.send(msg);
            })();

            // Forward to object detection worker if enabled
            const object_detection_enabled = opts.settings()['object_detection_enabled'] === 'true';
            if (object_detection_enabled) {
                const msg: ServerToWorkerObjectDetectionMessage = decoded;
                // Forward to object detection worker
                opts.worker_object_detection().postMessage(msg);
            }
        }
    }

}