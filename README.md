# Unblink V2

AI-powered camera monitoring application with relay-node architecture and real-time video processing.

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/zapdos-labs/unblink)

- **Vision Language Model**: Qwen3-VL for frame analysis and summarization
- **Chat Interface**: Natural language interaction with your camera feeds
- **Video Search**: Search through recorded frames using natural language

## Getting Started

The node runs locally and connects to the relay:

### Without Go Toolchain

Download the prebuilt `unblink-node` archive for your platform from GitHub Releases:

- Linux x86_64: `unblink-node_linux_amd64.tar.gz`
- Linux ARM64: `unblink-node_linux_arm64.tar.gz`
- Windows x86_64: `unblink-node_windows_amd64.zip`
- Windows ARM64: `unblink-node_windows_arm64.zip`

Extract the archive and run:

```bash
unblink-node
```

On Windows, run:

```powershell
.\unblink-node.exe
```

### With Go Toolchain

Install from source:

```bash
go install github.com/zapdos-labs/unblink/cmd/unblink-node@main

# Make sure you have Go’s bin dir on PATH
unblink-node

# Or you can run
# $(go env GOPATH)/bin/unblink-node
```

Uninstall:

```bash
rm -f "$(go env GOBIN)/unblink-node"
# if GOBIN is empty, use:
rm -f "$(go env GOPATH)/bin/unblink-node"
```

On first run, authorize the node by opening the displayed URL in your browser.

## Development

```bash
# Install dependencies
make install

# Run development environment (tmux with server + node + app)
make dev

# Typecheck code
make typecheck

# Generate protobuf code
make proto
```

### Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

## Architecture
- **Server**: Publicly reachable server that manages nodes, handles authentication, and stores configuration
- **Node**: Private proxy that runs in your network and forwards traffic to local cameras (RTSP, MJPEG)
- **Frontend**: SolidJS with TypeScript, Vite, and Ark UI components
- **Database**: PostgreSQL with pgx
- **Video**: go2rtc for RTSP/WebRTC handling