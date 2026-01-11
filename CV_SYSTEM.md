# Computer Vision Event System

## Overview

The CV system uses an event-driven architecture where the relay emits frame events and workers consume them, processing frames and emitting their own events back.

## Key Concepts

### Event Bus

Central event bus that manages all event distribution:
- Emits `frame` events for single frames
- Emits `frame_batch` events for batches of frames
- Receives worker events (summary, metrics, alerts, etc.)
- Broadcasts events to all connected workers

### Event Types

#### Frame Event (from relay)
```json
{
  "type": "frame",
  "id": "...",
  "created_at": "2026-01-10T12:00:00Z",
  "data": {
    "service_id": "cam-uuid",
    "frame_uuid": "frame-uuid"
  }
}
```

#### Frame Batch Event (from relay)
```json
{
  "type": "frame_batch",
  "id": "...",
  "created_at": "2026-01-10T12:00:00Z",
  "data": {
    "service_id": "cam-uuid",
    "frames": ["uuid-1", "uuid-2", ..., "uuid-10"],
    "metadata": {
      "duration_seconds": 50.5
    }
  }
}
```

#### Worker Event (from worker to relay)

Workers can emit flexible event data. The relay interprets common patterns:

**Summary Event:**
```json
{
  "summary": "Processed 10 frames from cam-123. Duration: 50.5s"
}
```

**Metrics Event:**
```json
{
  "metric_type": "detection_count",
  "value": 42,
  "created_at": "2026-01-10T12:00:00Z"
}
```

**Alert Event:**
```json
{
  "alert": "Motion detected in zone A",
  "severity": "high",
  "created_at": "2026-01-10T12:00:00Z"
}
```

## Worker Flow

1. **Connect** via WebSocket: `ws://relay:7010/connect`
2. **Register** and receive authentication key (256-bit)
3. **Listen** for `frame` and `frame_batch` events via WebSocket
4. **Download** frames: `GET /frames/{frameUUID}` with `X-Worker-Key` header
5. **Process** frames with CV models
6. **Emit** events: `POST /events` with event data

## Frame Download

### API

**URL**: `GET /frames/{frameUUID}`
**Auth**: `X-Worker-Key: {worker_key}`

Any authenticated worker can download any frame - no restrictions.

### Example

```bash
curl http://localhost:7010/frames/{frame_uuid} \
  -H "X-Worker-Key: {your_key}" \
  -o frame.jpg
```

## Worker Event Publishing

### API

**URL**: `POST /events`
**Auth**: `X-Worker-Key: {worker_key}`
**Body**: JSON object with event data

```bash
curl -X POST http://localhost:7010/events \
  -H "Content-Type: application/json" \
  -H "X-Worker-Key: {your_key}" \
  -d '{"summary": "Processed batch successfully"}'
```

### Event Data Structure

Workers can emit any JSON-serializable data. Common patterns:

- `summary`: Text summary of processing results
- `metric_type` + `value`: Quantitative metrics
- `alert`: Alert messages
- Custom fields as needed

## Benefits

### For Workers

- **Simple**: Listen for events and emit results
- **Flexible**: Workers self-select events to process
- **Streaming**: Workers can emit continuous event streams
- **Stateless**: No state management required

### For Relay

- **Scalable**: Event bus scales better than traditional queuing
- **Extensible**: Easy to add new event types
- **Observable**: All worker outputs are events (easy to log/analyze)
- **Decoupled**: Workers and relay communicate purely through events

## Configuration

**Required:**
```bash
APP_DIR=/var/lib/unblink
FRAME_INTERVAL_SECONDS=5
WORKER_PORT=7010
DASHBOARD_URL=http://localhost:3000
```

**Optional:**
```bash
BATCH_SIZE=10                      # Frames per batch event (default: 10)
AUTO_REQUEST_REALTIME_STREAM=true  # Auto-create streams (default: true)
```

## Example Worker Implementation

See [examples/worker-qwen3-vl/main.py](examples/worker-qwen3-vl/main.py) for a complete Python implementation.

Key methods:
- `listen_for_events()`: Handle incoming frame/batch events
- `handle_frame_batch_event()`: Process batch and emit summary
- `download_frame()`: Download frame with worker key
- `emit_event()`: Publish event back to relay

## Testing

Run the worker:
```bash
cd /home/tri/unblink/examples/worker-qwen3-vl
python main.py
```

The worker will:
1. Connect and register
2. Receive frame and frame_batch events
3. Download frames for batch events
4. Emit summary events back to relay

Check relay logs for:
- `[CVEventBus] Emitted frame event`
- `[CVEventBus] Emitted frame batch event`
- `[CVEventBus] Worker {id}: {summary}`

## Architecture Details

### Frame Extraction

1. Relay creates persistent realtime streams for each camera
2. `CVFrameExtractor` extracts JPEG frames at configured interval (default: 5s)
3. Frames are stored in `/storage/frames/{uuid}.jpg`
4. For each frame:
   - Emit `frame` event to all workers
   - Add to batch accumulator
5. When batch reaches configured size (default: 10 frames):
   - Emit `frame_batch` event to all workers
   - Reset batch accumulator

### Worker Authentication

1. Worker connects via WebSocket
2. Sends `register` message with worker_id
3. Relay generates unique 256-bit key
4. Relay returns key to worker
5. Worker stores key for all future requests

### Event Broadcasting

1. Frame extractor emits events to event bus
2. Event bus broadcasts to all connected workers via WebSocket
3. Workers self-select which events to process
4. Workers download frames and process
5. Workers emit events back to relay

### Security

- Each worker gets unique 256-bit cryptographic key at registration
- Keys are valid until worker disconnects
- All frame downloads require valid worker key
- All event publishing requires valid worker key
- Keys are deleted when worker disconnects

## Storage Structure

```
/var/lib/unblink/storage/
└── frames/
    ├── {frame-uuid-1}.jpg
    ├── {frame-uuid-2}.jpg
    └── ...
```

Frames are stored flat by UUID and persist indefinitely.

## WebSocket Protocol

### Worker → Relay: Register
```json
{
  "type": "register",
  "data": {}
}
```

### Relay → Worker: Registration Confirmed
```json
{
  "type": "registered",
  "data": {
    "worker_id": "worker-uuid",
    "key": "a3f9e2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1"
  }
}
```

### Worker → Relay: Heartbeat
```json
{
  "type": "heartbeat"
}
```

## Next Steps

1. **Dashboard Integration**: Display worker events in real-time
2. **Event Persistence**: Store worker events in database
3. **Event Filtering**: Allow workers to subscribe to specific event types
4. **Event Replay**: Replay historical events for debugging
5. **Metrics Aggregation**: Aggregate metrics from worker events
6. **Alert System**: Route alert events to notification system
