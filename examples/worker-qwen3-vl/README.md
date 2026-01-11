# CV Worker - Qwen3-VL Example

A simple Computer Vision worker that connects to the Unblink relay and receives jobs.

## Overview

This worker demonstrates how to:
- Connect to the CV relay via WebSocket
- Register and receive authentication keys
- Listen for job broadcasts
- Log job details to console
- (Optionally) Claim jobs, download frames, and submit results

By default, this worker **only logs jobs** without claiming them. This is useful for testing and observing the CV system in action.

## Features

- **WebSocket Connection**: Maintains persistent connection to relay
- **Authentication**: Uses worker key-based authentication
- **Heartbeat**: Sends periodic heartbeats to keep connection alive
- **Job Logging**: Displays detailed job information including:
  - Job ID, type, service ID
  - Timestamp
  - Frame count and UUIDs
  - Metadata

## Installation

```bash
# Install dependencies using uv
cd /home/tri/unblink/examples/worker-qwen3-vl
uv sync

# Or using pip
pip install -e .
```

## Usage

### Basic Usage (Log Only)

By default, the worker will connect and log all available jobs:

```bash
python main.py
```

You should see output like:

```
============================================================
CV Worker - Qwen3-VL
============================================================
This worker connects to the Unblink relay and logs jobs
To enable job claiming/processing, uncomment the claim_job
call in the handle_job method
============================================================

[Worker] Connecting to ws://localhost:7010/connect
[Worker] Connected!
[Worker] Registering as 123e4567-e89b-12d3-a456-426614174000
[Worker] Registered successfully!
[Worker] Worker ID: 123e4567-e89b-12d3-a456-426614174000
[Worker] Auth Key: a3f9e2b1c4d5e6f7...
[Worker] Listening for jobs...
[Worker] Heartbeat sent

============================================================
[Worker] JOB AVAILABLE!
============================================================
  Job ID:       job-uuid-1234
  Type:         frame_analysis
  Service ID:   camera-service-uuid
  Timestamp:    2026-01-10T12:00:00Z
  Frames:       1 frames
  Frame UUIDs:  ['frame-uuid-abcd']
  Metadata:     {
  "resolution": "1920x1080"
}
============================================================
```

### Enable Job Claiming

To make the worker actually claim and process jobs, uncomment line 98 in [main.py](main.py):

```python
async def handle_job(self, job):
    """Handle a job broadcast - log it and optionally claim it"""
    # ... logging code ...

    # Uncomment this line:
    await self.claim_job(job)
```

This will enable:
- Job claiming via HTTP POST
- Frame downloading with authentication
- Job completion/failure reporting

### Custom Relay URL

```python
worker = CVWorker(
    relay_url="ws://your-relay:7010/connect",
    http_url="http://your-relay:7010"
)
asyncio.run(worker.run())
```

## How It Works

### 1. Connection & Registration

The worker connects to the relay's WebSocket endpoint and registers:

```json
→ {"type": "register", "data": {"worker_id": "worker-uuid"}}
← {"type": "registered", "data": {"worker_id": "worker-uuid", "key": "auth-key"}}
```

### 2. Job Broadcasting

When frames are extracted, the relay broadcasts jobs:

```json
← {
    "type": "job_available",
    "job": {
        "id": "job-uuid",
        "type": "frame_analysis",
        "service_id": "cam-uuid",
        "timestamp": "2026-01-10T12:00:00Z",
        "frames": ["frame-uuid-1"],
        "metadata": {"resolution": "1920x1080"}
    }
}
```

### 3. Job Claiming (Optional)

If enabled, the worker claims the job via HTTP:

```bash
POST /jobs/{job_id}/claim
{"worker_id": "worker-uuid"}
```

### 4. Frame Download (Optional)

Download frames using the authentication key:

```bash
GET /jobs/{job_id}/frames/{frame_uuid}
X-Worker-Key: {worker_key}
```

### 5. Job Completion (Optional)

Submit results or report failures:

```bash
POST /jobs/{job_id}/complete
{"worker_id": "worker-uuid", "result": {...}}
```

## Architecture

```
┌─────────────┐                  ┌──────────────┐
│   Relay     │                  │    Worker    │
│             │                  │              │
│  WebSocket  │◄─────Connect─────│  WebSocket   │
│   Server    │                  │    Client    │
│             │                  │              │
│             │─────Register────►│              │
│             │◄────Auth Key─────│              │
│             │                  │              │
│ CVJobQueue  │────Job Broadcast►│ handle_job() │
│             │     (WebSocket)  │    (log)     │
│             │                  │              │
│  HTTP API   │◄───Claim Job─────│   (HTTP)     │
│             │      (POST)      │              │
│             │                  │              │
│  Storage    │◄──Download Frame─│   (HTTP)     │
│  Manager    │    (GET + Key)   │              │
│             │                  │              │
└─────────────┘                  └──────────────┘
```

## Next Steps

To extend this worker for actual CV processing:

1. **Uncomment job claiming** in `handle_job()`
2. **Add CV model loading** (Qwen3-VL, YOLO, etc.)
3. **Process downloaded frames** in `process_job()`
4. **Return meaningful results** in `complete_job()`

Example CV processing addition:

```python
async def process_job(self, job):
    job_id = job.get("id")
    frames = job.get("frames", [])

    results = []
    for frame_uuid in frames:
        # Download frame
        frame_data = await self.download_frame(job_id, frame_uuid)

        # Run CV model
        detections = self.run_cv_model(frame_data)
        results.extend(detections)

    # Submit results
    await self.complete_job(job_id, {
        "detections": results,
        "model": "qwen3-vl",
        "version": "1.0"
    })
```

## Requirements

- Python 3.12+
- websockets >= 12.0
- requests >= 2.31.0
- Unblink relay running with CV system enabled

## Troubleshooting

**Connection refused**
- Make sure the relay is running on port 7010
- Check that CV system is enabled in relay configuration

**No jobs appearing**
- Ensure a camera with RTSP/MJPEG service is connected
- Check relay logs for frame extraction messages
- Verify AUTO_REQUEST_REALTIME_STREAM=true in relay config

**Authentication errors**
- Worker key is issued during registration
- Key is used for all HTTP frame downloads
- Key is invalidated when worker disconnects

## Reference

See [CV_SYSTEM.md](/home/tri/unblink/CV_SYSTEM.md) for complete CV system documentation.
