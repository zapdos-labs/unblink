import type { EngineToServer, ServerToEngine } from "~/shared";
import { Conn } from "~/shared/Conn";
import { logger } from "../logger";
import { updateMediaUnit } from "../database";
import type { WebhookMessage } from "~/shared/alert";
import type { ServerWebSocket } from "bun";
import type { WsClient } from "../WsClient";

export function connect_to_engine(props: {
    ENGINE_URL: string,
    forward_to_webhook: (msg: WebhookMessage) => Promise<void>,
    clients: () => Map<ServerWebSocket, WsClient>,
}) {
    const engine_conn = new Conn<ServerToEngine, EngineToServer>(`wss://${props.ENGINE_URL}/ws`, {
        onOpen() {
            const msg: ServerToEngine = {
                type: "i_am_server",
            }
            engine_conn.send(msg);
        },
        onClose() {
            logger.info("Disconnected from Zapdos Labs engine WebSocket");
        },
        onError(event) {
            logger.error(event, "WebSocket to engine error:");
        },
        onMessage(decoded) {
            if (decoded.type === 'frame_description') {
                // Store in database
                updateMediaUnit({
                    id: decoded.frame_id,
                    description: decoded.description,
                })

                // Forward to clients 
                for (const [id, client] of props.clients()) {
                    client.send(decoded, false);
                }

                // Also forward to webhook
                props.forward_to_webhook({
                    event: 'description',
                    data: {
                        created_at: new Date().toISOString(),
                        stream_id: decoded.stream_id,
                        frame_id: decoded.frame_id,
                        description: decoded.description,
                    }
                });
            }

            if (decoded.type === 'frame_embedding') {
                // Store in database
                updateMediaUnit({
                    id: decoded.frame_id,
                    embedding: decoded.embedding,
                })
            }
        }
    });

    return engine_conn;
}