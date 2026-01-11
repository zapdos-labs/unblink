<p align="center">
<img width="300" src="assets/logo.svg">
</p>

[![GitHub Stars](https://img.shields.io/github/stars/zapdos-labs/unblink?style=flat)](https://github.com/zapdos-labs/unblink/stargazers)
[![Discord](https://img.shields.io/badge/Discord-Join%20Server-5865F2?style=flat&logo=discord&logoColor=white)](https://discord.gg/YMAjT8A6e2)

# Unblink

Unblink is an AI camera monitoring application.

Unblink lets AI workers connect to your cameras in a federated manner: once published, camera events are broadcast to whitelisted workers, and these workers emit back events (summaries, detections, alerts, etc.) that are stored and searchable later. You can self-host your AI workers, use public ones, or use dedicated workers provided by Unblink.

# Get started

## Run the Node

The node runs in your private network and forwards traffic from the relay to your local cameras.

```bash
# Install
go install github.com/zapdos-labs/unblink/node/cmd/unblink@latest

# Run (make sure ~/go/bin is in your PATH)
~/go/bin/unblink

# First run: authorizes with relay and generates credentials
# Subsequent runs: uses saved credentials
```

### Configuration

The node creates a config file at `~/.unblink_config.jsonc` on first run. Edit it to add your cameras:

```json
{
  "relay_addr": "your-relay.com:9000",
  "services": [
    { "addr": "192.168.1.100", "port": 554 },
    { "addr": "192.168.1.101", "port": 80 }
  ]
}
```

### Commands

```bash
unblink show-config  # Show config file path
unblink login        # Authorize with relay
unblink logout       # Remove saved credentials
```

### Authorization Flow

On first run, the node will print an authorization URL. Open it in your browser to authorize the node with your account. The node will then save the credentials and connect automatically on subsequent runs.

## Architecture

Unblink uses a minimal, TCP-based bridging protocol with strict separation of concerns:

```
Public Internet                       Private Network
────────────────                     ─────────────────

┌─────────┐         HTTPS/WSS           ┌──────────┐
│ Client  │◄───────────────────────────►│          │
│  (Web)  │                             │          │
└─────────┘                             │          │
                                        │  Relay   │
┌─────────┐         WebSocket           │ (Public) │
│ Worker  │◄───────────────────────────►│          │
└─────────┘   (frame events, etc.)      └────┬─────┘
                                             │
                                             │ TLS (persistent)
                                             │
                                        ┌────▼────┐
                                        │  Node   │
                                        │ (Proxy) │
                                        └────┬────┘
                                             │
                                 ┌───────────┴──────────┐
                                 │                      │
                            ┌────▼─────┐           ┌────▼─────┐
                            │   RTSP   │           │  MJPEG   │
                            │  Camera  │           │  Camera  │
                            └──────────┘           └──────────┘
```

### Relay

Public traffic router and multiplexer. The relay:

- Is publicly reachable
- Manages nodes and clients
- Creates and multiplexes bridges (logical data channels between client and service)
- Handles CV event bus, frame extraction, and worker coordination

### Node

Private TCP forwarder that runs in your private network. The node:

- Maintains one persistent TLS connection to the relay
- Opens TCP connections to local services on demand (cameras, RTSP, etc.)
- Forwards raw bytes without inspection

### Worker

AI vision processors that integrate with the relay's CV system:

- Connect via WebSocket to receive events
- Run custom AI models (e.g., Qwen3-VL)
- Emit events back (summaries, metrics, alerts)

See [UNBLINK_PROTOCOL.md](UNBLINK_PROTOCOL.md) and [docs/FEDERATED_WORKERS.md](docs/FEDERATED_WORKERS.md) for detailed specifications.

## Uninstall

```bash
# Remove the binary
rm ~/go/bin/unblink

# Remove config
rm ~/.unblink_config.jsonc
```

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.
