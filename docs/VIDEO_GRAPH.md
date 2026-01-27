# Unblink Relay Architecture

## Video Graph

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                               CAMERA SOURCES                                │
└──────────────────────┬──────────────────────────────┬───────────────────────┘
                       │                              │
            ┌──────────▼──────────┐        ┌──────────▼──────────┐
            │     RTSP Camera     │        │    MJPEG Camera     │
            │   (H.264 + Audio)   │        │    (MJPEG/HTTP)     │
            └──────────┬──────────┘        └──────────┬──────────┘
                       │                              │
┌──────────────────────▼──────────────────────────────▼───────────────────────┐
│                              go2rtc Integration                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │ RTSP: rtsp.Conn extracts raw H.264 + audio from RTP packets          │  │
│  │ MJPEG: FFmpeg transcodes MJPEG → H.264 Annex-B format                │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└──────────────┬──────────────────────────────────────────────┬───────────────┘
               │                                              │
               │ Raw H.264 + Audio                            │ H.264 (transcoded)
               │                                              │
┌──────────────▼─────────────┐                ┌──────────────▼──────────────┐
│     MediaSource (RTSP)     │                │         MJPEGSource         │
│  GetProducer() → rtsp.Conn │                │ ┌─────────────────────────┐ │
│  Returns H.264 + audio     │                │ │  FFmpeg: TRANSCODE      │ │
│                            │                │ │  MJPEG → H.264          │ │
│                            │                │ │  Output: Raw H.264      │ │
└──────────────┬─────────────┘                └──────────────┬──────────────┘
               │                                              │
               │                                              │
               └──────────────────────┬───────────────────────┘
                                      │
                                      │ H.264 Producer (go2rtc core.Producer)
                                      │
                 ┌────────────────────┼────────────────────┐
                 │                    │                    │
      ┌──────────▼─────────┐ ┌────────▼──────────┐ ┌───────▼──────────┐
      │   FrameExtractor   │ │   VideoRecorder   │ │  WebRTC Session  │
      │ ┌────────────────┐ │ │     (Future)      │ │ ┌──────────────┐ │
      │ │ H264Consumer   │ │ │                   │ │ │ Direct tracks│ │
      │ │ → Raw H.264    │ │ │                   │ │ │ H.264 + audio│ │
      │ │ → FFmpeg       │ │ │                   │ │ │ to browser   │ │
      │ │ → JPEG frames  │ │ │                   │ │ └──────────────┘ │
      │ └───────┬────────┘ │ └───────────────────┘ └────────┬─────────┘
      └─────────│──────────┘                                │
                ▼                                            ▼
           JPEG for AI                                  WebRTC Stream
           (stored to disk)                             (video + audio)
```

## Pipeline Details

### RTSP Camera Pipeline

```
RTSP Camera (H.264 + PCMA/PCMU/AAC audio)
    ↓
go2rtc rtsp.Conn (RTP depayload)
    ↓
Producer (H.264 video + audio tracks)
    ↓
├─→ WebRTC Session
│   └─ Direct track streaming (no transcoding)
│   └─ Browser plays H.264 video + audio
│
└─→ FrameExtractor
    └─ H264Consumer → Raw H.264 Annex-B
    └─ FFmpeg (-f h264) → JPEG frames
    └─ Saved to disk for AI analysis
```

### MJPEG Camera Pipeline

```
MJPEG Camera (HTTP multipart/x-mixed-replace)
    ↓
FFmpeg (MJPEG → H.264 transcoding, -f h264 output)
    ↓
go2rtc magic.Open() (parses raw H.264)
    ↓
Producer (H.264 video track)
    ↓
├─→ WebRTC Session
│   └─ Direct track streaming (no additional transcoding)
│   └─ Browser plays H.264 video
│
└─→ FrameExtractor
    └─ H264Consumer → Raw H.264 Annex-B
    └─ FFmpeg (-f h264) → JPEG frames
    └─ Saved to disk for AI analysis
```

## Key Optimizations

### 1. Eliminated MPEG-TS Container
**Before:** Producer → MPEG-TS muxing → FFmpeg demuxing → codec processing
**After:** Producer → Direct H.264 Annex-B streaming → codec processing

**Benefits:**
- ~10-20% CPU reduction
- Lower latency
- No container overhead

### 2. Direct Track Consumption
Both FrameExtractor and WebRTC Session consume tracks directly from the producer using `GetTrack()`, following go2rtc's composable architecture.

### 3. Format Normalization Strategy
All video is normalized to **H.264** through go2rtc:
- RTSP cameras: Native H.264 (pass-through)
- MJPEG cameras: FFmpeg transcodes to H.264 (necessary for WebRTC)

This allows uniform downstream processing while keeping the architecture clean.

## Transcoding vs Remuxing

| Operation     | Description                      | CPU Usage |
| ------------- | -------------------------------- | --------- |
| **Transcode** | Decode → Encode (changes codec)  | HIGH      |
| **Remux**     | Copy codec data to new container | ~1%       |

## Component Responsibilities

| Component            | Input          | Output              | Purpose                                     |
| -------------------- | -------------- | ------------------- | ------------------------------------------- |
| **go2rtc rtsp.Conn** | RTSP/RTP       | H.264 + Audio       | RTSP protocol handling, RTP depayload       |
| **H264Consumer**     | H.264 Producer | Raw H.264 Annex-B   | Direct H.264 streaming (no container)       |
| **MJPEGSource**      | MJPEG          | H.264 Producer      | MJPEG → H.264 transcoding for WebRTC        |
| **FrameExtractor**   | H.264 Producer | JPEG frames         | AI frame extraction via H264Consumer        |
| **WebRTC Session**   | H.264 Producer | WebRTC tracks       | Browser streaming with audio support        |

## FFmpeg Usage

### FrameExtractor FFmpeg Command
```bash
ffmpeg -f h264 -i pipe:0 -vf fps=0.2 -f image2pipe -c:v mjpeg -q:v 2 pipe:1
```
- Input: Raw H.264 Annex-B stream
- Output: JPEG frames at 5-second intervals
- Purpose: Extract frames for AI analysis

### MJPEGSource FFmpeg Command
```bash
ffmpeg -f mjpeg -i pipe:0 -c:v libx264 -preset superfast -tune zerolatency -f h264 pipe:1
```
- Input: MJPEG HTTP stream
- Output: Raw H.264 Annex-B stream
- Purpose: Transcode for WebRTC (browsers don't support MJPEG)

## Audio Support

### RTSP Cameras
Supports multiple audio codecs:
- **PCMA** (G.711 A-law)
- **PCMU** (G.711 µ-law)
- **AAC** (Advanced Audio Coding)
- **Opus** (if available)

Audio tracks are passed directly to WebRTC without transcoding.

### MJPEG Cameras
Typically video-only (no audio in MJPEG streams).

## Architecture Benefits

1. **Clean separation:** Frame extraction vs WebRTC streaming use same producer
2. **Leverages go2rtc:** Uses battle-tested components (rtsp.Conn, magic.Open)
3. **Minimal overhead:** Direct codec streaming, no container muxing/demuxing
4. **Audio support:** PCMA/PCMU/AAC work seamlessly (no MPEG-TS limitations)
5. **Composable:** Multiple consumers can attach to same producer
6. **Future-proof:** Easy to add HLS recording, more codecs, etc.

## Performance Characteristics

| Scenario           | CPU Usage | Notes                                  |
| ------------------ | --------- | -------------------------------------- |
| RTSP streaming     | ~10-15%   | Direct H.264, minimal processing       |
| RTSP + extraction  | ~30-40%   | Adds H.264→JPEG transcoding            |
| MJPEG streaming    | ~20-30%   | MJPEG→H.264 transcoding required       |
| MJPEG + extraction | ~50-60%   | MJPEG→H.264 + H.264→JPEG (unavoidable) |

**Note:** MJPEG cameras require transcoding because browsers don't support MJPEG in WebRTC. The H.264→JPEG conversion for frame extraction is necessary because we've normalized everything to H.264 for WebRTC compatibility.
