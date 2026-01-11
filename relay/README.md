# Relay

WebRTC relay server for streaming RTSP/MJPEG sources to browsers.

## Tests

### Unit Tests

```bash
go test -v -run TestRelayStartStop      # Relay lifecycle
go test -v -run TestNodeRegister        # Node registration
go test -v -run TestServiceAnnounce     # Service announcement
go test -v -run TestOpenBridge          # Bridge opening
```

### Integration Tests

Require `allowlist.json` with real services.

```bash
go test -v -run TestEndToEndWithRealService  # RTSP bridge flow
go test -v -run TestGo2RTCIntegration        # go2rtc RTSP streaming
go test -v -run TestWebRTCIntegration        # WebRTC from RTSP
go test -v -run TestWebRTCIntegrationMJPEG   # WebRTC from MJPEG
```

### Manual Browser Tests

Start server and test with the web app.

```bash
# All services
go test -v -run TestWebRTCManual -relay-addr=:8080 -http-addr=:8081

# MJPEG only (with FFmpeg transcoding)
go test -v -run TestWebRTCMJPEGManual -mjpeg-relay-addr=:8080 -mjpeg-http-addr=:8081
```

Then open the app dev server or `http://localhost:8081`.

## HTTP API

- `GET /services` - List available streams
- `POST /offer` - WebRTC signaling (send SDP offer, receive answer)

## Audio Support

The relay streams both video and audio if the source provides them.
