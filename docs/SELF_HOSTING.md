# Self-Hosting Guide

## Requirements

- Go 1.25+
- PostgreSQL 16+
- FFmpeg (for video processing)

## Quick Start

```bash
# Set GOPRIVATE first (required for custom go2rtc fork)
go env -w GOPRIVATE=github.com/unblink

# Clone
git clone https://github.com/unblink/unb.git
cd unb

# Install dependencies
go mod download
cd app && bun install && cd ..

# Configure
cp config.example.json server.config.json
# Edit server.config.json with your settings

# Run
go run cmd/server/main.go
```

## Configuration

Edit `server.config.json`:

```json
{
  "database_url": "postgresql://user:password@localhost:5432/unb?sslmode=disable",
  "port": 8080,
  "openai_api_key": "sk-..."  // optional, for models
}
```

## Build & Run

```bash
# Development
go run cmd/server/main.go

# Production build
go build -o unb cmd/server/main.go
./unb
```

## Troubleshooting

### Port Already in Use

Edit `port` in `server.config.json`.

### Database Connection Failed

Ensure PostgreSQL is running and the database URL in `server.config.json` is correct.
