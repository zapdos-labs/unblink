## Usage

# Deploy to Cloudflare

[![Deploy to Cloudflare](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/unblink/unblink/tree/main/app)

## Env vars

If you are also self-hosting relay or other components, update the env vars accordingly in the Cloduflare worker creation page.

For example, in production we replace this

```
RELAY_API_URL=http://127.0.0.1:8080
```

with

```
RELAY_API_URL=https://api.unblink.net
```

# Development

```sh
bun i
bun dev
```

# Run locally

```sh
bun start
```
