# Unblink V2

AI-powered camera monitoring application with relay-node architecture and real-time video processing.

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
go install github.com/zapdos-labs/unblink/cmd/unblink-node@latest
unblink-node
```

On first run, authorize the node by opening the displayed URL in your browser.

## Features

### Relay-Node Architecture
- **Relay (Server)**: Publicly reachable server that manages nodes, handles authentication, and stores configuration
- **Node**: Private proxy that runs in your network and forwards traffic to local cameras (RTSP, MJPEG)

### AI-Powered Monitoring
- **Vision Language Model**: Qwen3-VL for frame analysis and summarization
- **Chat Interface**: Natural language interaction with your camera feeds
- **Video Search**: Search through recorded frames using natural language

### Frame Processing
- Configurable frame extraction intervals
- Batch processing for efficient VLM calls
- Automatic frame storage and indexing

### Real-time Communication
- WebSocket / WebRTC connections for live video streams
- Connect RPC for type-safe API communication

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

Required variables:
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - JWT signing secret
- `CHAT_OPENAI_API_KEY` - OpenAI API key for chat
- `VLM_OPENAI_API_KEY` - OpenAI API key for vision

Server and CLI commands are env-only. `server.config.json` is no longer supported.

## Deployment

### Docker

```bash
make docker-build
make docker-run
```

### Render

1. Fork the repository
2. Connect to Render
3. Configure environment variables in render.yaml
4. Deploy

## Architecture

- **Backend**: Go
- **Frontend**: SolidJS with TypeScript, Vite, and Ark UI components
- **Database**: PostgreSQL with pgx
- **Video**: go2rtc for RTSP/WebRTC handling

## License

MIT
