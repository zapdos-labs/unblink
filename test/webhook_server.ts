const server = Bun.serve({
    port: 4000,
    async fetch(req) {
        const data = await req.json();
        console.log(`Received webhook POST request`, data);
        return new Response('OK');
    }
});

console.log(`Webhook server running at http://localhost:${server.port}/`);