<p align="center">
<img width="300" src="assets/logo.svg">
</p>

[![GitHub Stars](https://img.shields.io/github/stars/unblink/unblink?style=flat)](https://github.com/unblink/unblink/stargazers)
[![Discord](https://img.shields.io/badge/Discord-Join%20Server-5865F2?style=flat&logo=discord&logoColor=white)](https://discord.gg/34jpgpW9Hy)

# Unblink

Unblink is an AI camera monitoring application.

# Get started

## Run the Node

The node runs in your private network and forwards traffic from the relay to your local cameras.

```bash
# Install
go install github.com/zapdos-labs/unblink/cmd/node@latest

# Run (make sure ~/go/bin is in your PATH)
node
```

The node will:

- Connect to the relay
- Load config from `~/.unblink/config.json`
- Create bridges to local services on demand

On first run, a URL will be shown to your so that you can open in your browser and authorize (link) the node with your account.

### Relay

Public traffic router and multiplexer. The relay:

- Is publicly reachable
- Manages nodes and clients via Connect RPC
- Creates and multiplexes bidirectional data streams
- Handles user authentication with JWT tokens
- Stores configuration in SQLite database

### Node

Private proxy that runs in your private network. The node:

- Maintains one persistent WebSocket connection to the relay
- Opens TCP connections to local services on demand (cameras, RTSP, etc.)
- Forwards data via bidirectional streaming without inspection

### Protocol

See [docs/NODE_RELAY_PROTOCOL.md](docs/NODE_RELAY_PROTOCOL.md) for detailed protocol specifications.

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.
