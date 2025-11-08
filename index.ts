import { decode } from "cbor-x";
import { randomUUID } from "crypto";
import { RECORDINGS_DIR, RUNTIME_DIR } from "./backend/appdir";
import { searchMediaUnitsByEmbedding, table_media, table_sessions, table_settings, table_users } from "./backend/database";
import { logger } from "./backend/logger";

import type { ServerWebSocket } from "bun";
import { WsClient } from "./backend/WsClient";
import { verifyPassword } from "./backend/auth";
import { createForwardFunction } from "./backend/forward";
import { check_version } from "./backend/startup/check_version";
import { connect_to_engine } from "./backend/startup/connect_to_engine";
import { load_secrets } from "./backend/startup/load_secrets";
import { load_settings } from "./backend/startup/load_settings";
import { create_webhook_forward } from "./backend/webhook";
import { spawn_worker } from "./backend/worker_connect/shared";
import { start_stream_file, start_streams, stop_stream } from "./backend/worker_connect/worker_stream_connector";
import homepage from "./index.html";
import type { ClientToServerMessage, DbUser, RecordingsResponse } from "./shared";


logger.info(`Using runtime directory: ${RUNTIME_DIR}`);

const ENGINE_URL = process.env.ENGINE_URL || "api.zapdoslabs.com";
await check_version({ ENGINE_URL });

const clients = new Map<ServerWebSocket, WsClient>();

const { settings, setSettings } = await load_settings();
const { secrets } = await load_secrets();
const forward_to_webhook = create_webhook_forward({ settings });
const engine_conn = connect_to_engine({
    ENGINE_URL,
    clients: () => clients,
    forward_to_webhook,
});

const PORT = process.env.PORT ? parseInt(process.env.PORT) : 3000;
const SESSION_DURATION_HOURS = 8;
// Create Bun server
const server = Bun.serve({
    port: PORT,
    routes: {
        "/": homepage,
        "/test": async (req) => {
            return new Response("Test endpoint working");
        },
        "/auth/login": {
            POST: async (req: Request) => {
                const body = await req.json();
                const { username, password } = body;
                if (!username || !password) {
                    return new Response("Missing username or password", { status: 400 });
                }

                const user: DbUser | undefined = await table_users
                    .query()
                    .where(`username = "${username}"`)
                    .limit(1)
                    .toArray()
                    .then(users => users.at(0));

                if (!user) return new Response("Invalid username or password", { status: 401 });

                const is_valid = await verifyPassword(password, user.password_hash);
                if (!is_valid) return new Response("Invalid username or password", { status: 401 });

                const session_id = randomUUID();
                const created_at = new Date();
                const expires_at = new Date(created_at.getTime() + SESSION_DURATION_HOURS * 60 * 60 * 1000);

                await table_sessions.add([{ session_id, user_id: user.id, created_at, expires_at }]);

                const res = Response.json({ message: "Login successful" });
                let cookie = `session_id=${session_id}; HttpOnly; SameSite=Strict; Path=/; Max-Age=${SESSION_DURATION_HOURS * 3600}; Secure`;
                res.headers.append(
                    "Set-Cookie",
                    cookie
                );

                console.log(`User '${username}' logged in, session created with ID: ${session_id}`);
                return res;
            },
        },

        "/auth/logout": {
            POST: async (req: Request) => {
                const cookies = req.headers.get("cookie");
                const session_id = cookies?.match(/session_id=([^;]+)/)?.[1];

                if (!session_id) return new Response("Missing session_id", { status: 400 });

                await table_sessions.delete(`session_id = '${session_id}'`);

                const res = new Response("Logged out successfully", { status: 200 });
                let cookie = "session_id=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0; Secure";
                res.headers.append(
                    "Set-Cookie",
                    cookie
                );
                return res;
            },
        },

        "/auth/me": {
            GET: async (req: Request) => {
                const cookies = req.headers.get("cookie");
                const session_id = cookies?.match(/session_id=([^;]+)/)?.[1];
                if (!session_id) return new Response("Unauthorized", { status: 401 });

                const session = await table_sessions
                    .query()
                    .where(`session_id = "${session_id}"`)
                    .limit(1)
                    .toArray()
                    .then(s => s.at(0));

                if (!session || new Date(session.expires_at) < new Date())
                    return new Response("Session expired", { status: 401 });

                const user = await table_users
                    .query()
                    .where(`id = "${session.user_id}"`)
                    .limit(1)
                    .toArray()
                    .then(u => u.at(0));

                if (!user) return new Response("User not found", { status: 404 });

                // optional: extend session on activity
                const newExpiresAt = new Date(Date.now() + SESSION_DURATION_HOURS * 3600 * 1000);
                await table_sessions.update(
                    { expires_at: newExpiresAt.getTime().toString() },
                    { where: `session_id = "${session_id}"` }
                );

                return Response.json({ user });
            },
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
                await table_media.delete(`id = "${id}"`);
                return Response.json({ success: true });
            }
        },
        '/media': {
            GET: async () => {
                const media = await table_media.query().toArray();
                // @ts-ignore
                media.sort((a, b) => b.updated_at - a.updated_at);
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
                    .whenNotMatchedInsertAll()
                    .execute([{ key, value: value.toString() }]);

                setSettings(key, value.toString());
                return Response.json({ success: true });
            }
        },
        '/users': {
            GET: async () => {
                const users = await table_users.query().toArray();
                // @ts-ignore
                const safeUsers = users.map(({ password_hash, ...rest }) => rest);
                return Response.json(safeUsers);
            },
        },
        '/search': {
            POST: async (req: Request) => {
                const body = await req.json();
                const { query } = body;
                if (!query) {
                    return new Response('Missing query', { status: 400 });
                }

                // Generate the embedding for the query

                // Forward search request to engine
                const response = await fetch(`https://${ENGINE_URL}/api/worker/fast_embedding`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        job: {
                            text: query,
                            prompt_name: "query"
                        }
                    }),
                });

                if (!response.ok || !response.body) {
                    throw new Error("Search request failed");
                }

                const data = await response.json();
                const embedding: number[] = data.embedding;
                if (!embedding) {
                    throw new Error("No embedding returned from engine");
                }

                const media_units = await searchMediaUnitsByEmbedding(embedding);
                return Response.json({ media_units });
            }
        }
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
                        // logger.info(`Client subscription updated for ${ws.remoteAddress}: ${JSON.stringify(client.subscription)}`);

                        const newFileStreams = decoded.subscription?.streams.filter(s => s.file_name) || [];
                        // logger.info(`Client file subscriptions for ${ws.remoteAddress}: ${JSON.stringify(newFileStreams)}`);

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
            // --- FIX START ---
            // Construct a new URL using the target host and the incoming path.
            // This avoids carrying over the original request's port.
            const targetUrl = new URL(url.pathname, `https://${ENGINE_URL}`);
            targetUrl.search = url.search; // Preserve any query parameters
            // --- FIX END ---

            const headers = new Headers(req.headers);
            // The "host" header should reflect the target server, not the proxy server.
            headers.set("host", new URL(`https://${ENGINE_URL}`).host);

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

logger.info(`Server running on http://localhost:${PORT}`);

const forward = createForwardFunction({
    clients,
    worker_object_detection: () => worker_object_detection,
    settings,
    engine_conn: () => engine_conn,
    forward_to_webhook,
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
