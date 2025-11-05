import { decode, encode } from "cbor-x";
import { randomUUID } from "crypto";
import { RECORDINGS_DIR, RUNTIME_DIR } from "./backend/appdir";
import { table_media, table_media_units, table_settings, updateMediaUnit } from "./backend/database";
import { logger } from "./backend/logger";

import type { ServerWebSocket } from "bun";
import { WsClient } from "./backend/WsClient";
import { createForwardFunction } from "./backend/forward";
import { spawn_worker } from "./backend/worker_connect/shared";
import { start_stream_file, start_streams, stop_stream } from "./backend/worker_connect/worker_stream_connector";
import homepage from "./index.html";
import type { ClientToServerMessage, EngineToServer, RecordingsResponse, ServerToEngine } from "./shared";
import { Conn } from "./shared/Conn";


logger.info(`Using runtime directory: ${RUNTIME_DIR}`);

// Check version
const SUPPORTED_VERSION = "1.0.0";
const ENGINE_URL = process.env.ENGINE_URL || "api.zapdoslabs.com";
// Send /version request to engine
try {
    const version_response = await fetch(`https://${ENGINE_URL}/version`);
    if (version_response.ok) {
        const version_data = await version_response.json();
        if (version_data.version) {
            logger.info(`Engine version: ${version_data.version}`);
            if (version_data.version !== SUPPORTED_VERSION) {
                logger.warn(`Warning: Newer engine version available: ${version_data.version}. Supported version is ${SUPPORTED_VERSION}. Please consider updating the server.`);
                logger.warn(`Visit https://github.com/tri2820/unblink for update instructions.`);
            }
        }
    } else {
        logger.error(`Failed to fetch version from engine: ${version_response.status} ${version_response.statusText}`);
    }
} catch (error) {
    logger.error({ error }, "Error connecting to Zapdos Labs engine");
}


const clients = new Map<ServerWebSocket, WsClient>();

const engine_conn = new Conn<ServerToEngine, EngineToServer>(`wss://${ENGINE_URL}/ws`, {
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
            for (const [id, client] of clients) {
                client.send(decoded);
            }
        }

        if (decoded.type === 'frame_embedding') {
            console.log('Received embedding for frame', decoded.frame_id, 'embedding length:', decoded.embedding.length);
            // Store in database
            updateMediaUnit({
                id: decoded.frame_id,
                embedding: decoded.embedding,
            })
        }
    }
});

// Load settings into memory
let SETTINGS: Record<string, string> = {};
const settings_db = await table_settings.query().toArray();
for (const setting of settings_db) {
    SETTINGS[setting.key] = setting.value;
}
logger.info({ SETTINGS }, "Caches loaded");

// Create Bun server
const server = Bun.serve({
    port: 3000,
    routes: {
        "/": homepage,
        "/test": async (req) => {
            return new Response("Test endpoint working");
        },
        '/media/:id': {
            PUT: async ({ params, body }: { params: { id: string }, body: any }) => {
                const { id } = params;
                const data = await new Response(body).json();
                const { name, uri, labels, saveToDisk, saveDir } = data;
                if (!name || !uri) {
                    return new Response('Missing name or uri', { status: 400 });
                }
                const updated_at = new Date().toISOString();
                await table_media.mergeInsert("id")
                    .whenMatchedUpdateAll()
                    .execute([{ id, name, uri, labels: labels ?? [], updated_at, saveToDisk: saveToDisk ?? false, saveDir: saveDir ?? '' }]);
                return Response.json({ success: true });
            },
            DELETE: async ({ params }: { params: { id: string } }) => {
                const { id } = params;
                await table_media.delete(`id = '${id}'`);
                return Response.json({ success: true });
            }
        },
        '/media_units/media/:id': {
            GET: async () => {
                const media_units = await table_media_units.query().toArray();
                // @ts-ignore
                media_units.sort((a, b) => b.at_time.localeCompare(a.at_time));
                // mask out embedding for response
                for (const mu of media_units) {
                    mu.embedding = null;
                }
                return Response.json(media_units);
            },
        },
        '/media': {
            GET: async () => {
                const media = await table_media.query().toArray();
                // @ts-ignore
                media.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
                return Response.json(media);
            },
            POST: async (req: Request) => {
                const body = await req.json();
                const { name, uri, labels, saveToDisk, saveDir } = body;
                if (!name || !uri) {
                    return new Response('Missing name or uri', { status: 400 });
                }
                const id = randomUUID();
                const updated_at = new Date().toISOString();
                await table_media.add([{ id, name, uri, labels: labels ?? [], updated_at, saveToDisk: saveToDisk ?? false, saveDir: saveDir ?? '' }]);
                return Response.json({ success: true, id });
            },
        },
        '/recordings': {
            GET: async () => {
                try {
                    const recordingsByStream: RecordingsResponse = {};
                    const glob = new Bun.Glob("*/*.mkv");
                    for await (const file of glob.scan(RECORDINGS_DIR)) {
                        const parts = file.split("/");
                        if (parts.length < 2) {
                            continue;
                        }
                        const streamId = parts[0]!;
                        // from_1762122447803_ms.mkv
                        const file_name = parts[1]!;

                        const from_ms = file_name.match(/from_(\d+)_ms\.mkv/)?.[1];
                        const to_ms = file_name.match(/_to_(\d+)_ms\.mkv/)?.[1];

                        const fromDate = from_ms ? new Date(parseInt(from_ms)) : null;
                        const toDate = to_ms ? new Date(parseInt(to_ms)) : null;

                        if (!recordingsByStream[streamId]) {
                            recordingsByStream[streamId] = [];
                        }

                        recordingsByStream[streamId].push({
                            file_name: file_name,
                            from_ms: fromDate?.getTime(),
                            to_ms: toDate?.getTime(),
                        });
                    }
                    return Response.json(recordingsByStream);
                } catch (error) {
                    logger.error({ error }, 'Error fetching recordings');
                    return new Response('Error fetching recordings', { status: 500 });
                }
            }
        },
        '/settings': {
            GET: async () => {
                const settings = await table_settings.query().toArray();
                return Response.json(settings);
            },
            PUT: async (req: Request) => {
                // TODO: secure this endpoint
                const body = await req.json();
                const { key, value } = body;
                if (!key || value === undefined) {
                    return new Response('Missing key or value', { status: 400 });
                }
                await table_settings.mergeInsert("key")
                    .whenMatchedUpdateAll()
                    .execute([{ key, value: value.toString() }]);
                SETTINGS[key] = value.toString();
                return Response.json({ success: true });
            }
        },
    },
    websocket: {
        open(ws) {
            logger.info("WebSocket connection opened");
            clients.set(ws, new WsClient(ws));
        },
        close(ws, code, reason) {
            logger.info(`WebSocket connection closed: ${code} - ${reason}`);
            const client = clients.get(ws);
            if (client) {
                // Mark the client as closed to prevent further processing
                // Just in case other functions are still referencing it
                client.destroy();
            }
            clients.delete(ws);
        },
        message(ws, message) {
            try {
                const decoded = decode(message as Buffer) as ClientToServerMessage;
                if (decoded.type === 'set_subscription') {
                    const client = clients.get(ws);
                    if (client) {
                        const oldFileStreams = client.subscription?.streams.filter(s => s.file_name) || [];

                        client.updateSubscription(decoded.subscription);
                        logger.info(`Client subscription updated for ${ws.remoteAddress}: ${JSON.stringify(client.subscription)}`);

                        const newFileStreams = decoded.subscription?.streams.filter(s => s.file_name) || [];
                        logger.info(`Client file subscriptions for ${ws.remoteAddress}: ${JSON.stringify(newFileStreams)}`);

                        const removedOldFileStreams = oldFileStreams.filter(oldStream =>
                            !newFileStreams.find(newStream => newStream.id === oldStream.id && newStream.file_name === oldStream.file_name)
                        );

                        for (const stream of removedOldFileStreams) {
                            logger.info(`Client unsubscribed from file stream ${stream.id} (file: ${stream.file_name})`);
                            // Notify the worker about the removed file stream
                            stop_stream({
                                worker: worker_stream,
                                stream_id: stream.id,
                                file_name: stream.file_name,
                            });
                        }

                        const addedNewFileStreams = newFileStreams.filter(newStream =>
                            !oldFileStreams.find(oldStream => oldStream.id === newStream.id && oldStream.file_name === newStream.file_name)
                        );

                        for (const stream of addedNewFileStreams) {
                            logger.info(`Client subscribed to file stream ${stream.id} (file: ${stream.file_name})`);
                            // Notify the worker about the added file stream
                            start_stream_file({
                                worker: worker_stream,
                                stream_id: stream.id,
                                file_name: stream.file_name!,
                            });
                        }

                    }
                }
            } catch (error) {
                logger.error(error, 'Error parsing websocket message');
            }
        },
    },

    async fetch(req, server) {
        const url = new URL(req.url);

        // WebSocket upgrade
        if (url.pathname === "/ws") {
            if (server.upgrade(req)) {
                return; // do not return a Response
            } else {
                return new Response("Cannot upgrade to WebSocket", { status: 400 });
            }
        }

        // API Proxying
        if (url.pathname.startsWith("/api")) {
            const targetUrl = new URL(req.url);
            targetUrl.host = ENGINE_URL;
            targetUrl.protocol = "https:";

            const headers = new Headers(req.headers);
            // if (appConfig.store.auth_token) {
            //     headers.set("authorization", `Bearer ${appConfig.store.auth_token}`);
            // }

            try {
                const response = await fetch(targetUrl.toString(), {
                    method: req.method,
                    headers: headers,
                    body: req.body,
                    redirect: "manual",
                });
                return response;
            } catch (error) {
                logger.error(error, "Proxy error:");
                return new Response(JSON.stringify({ error: "Proxy error occurred" }), {
                    status: 500,
                    headers: { "Content-Type": "application/json" },
                });
            }
        }

        return new Response("Not found", { status: 404 });
    },

    development: process.env.NODE_ENV === "development",
});

logger.info("Server running on http://localhost:3000");

const forward = createForwardFunction({
    clients,
    worker_object_detection: () => worker_object_detection,
    settings: () => SETTINGS,
    engine_conn: () => engine_conn,
})

const worker_stream = spawn_worker('worker_stream.js', forward);
const worker_object_detection = spawn_worker('worker_object_detection.js', forward);


if (process.env.DEV_MODE === 'lite') {
    logger.info("Running in lite development mode - skipping stream startup");
} else {
    // Start all streams from the database
    start_streams({
        worker_stream,
    });
}
