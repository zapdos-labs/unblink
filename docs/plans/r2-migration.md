# S3 (Cloudflare R2) Storage Implementation Plan

## Overview

Replace the current disk-based frame storage with Cloudflare R2 (S3-compatible) storage. The old disk-based implementation will be removed entirely.

**User decisions:**

- HTTP serving: Presigned URLs with redirect
- R2 bucket: Private with signed URLs only
- Existing frames: Remove old disk implementation, use only R2

## Current Implementation

- **File**: [storage.go](server/webrtc/storage.go)
- **Storage Interface**: `Save(serviceID string, frame *Frame)` with `onSaved` callback
- **HTTP Serving**: `/frames/{serviceID}/{frameID}.jpg` via [storage.go](server/service/storage.go:130)
- **Database**: Stores `storage_path` (currently local file path)

---

## Implementation Plan

### 1. Create S3 Storage Implementation

**New file**: [s3_storage.go](server/webrtc/s3_storage.go)

```go
type S3Storage struct {
    client     *s3.Client
    presigner  *s3.PresignClient
    bucket     string
    onSaved    func(serviceID, frameID, s3Key string, timestamp time.Time, fileSize, sequence int64)
}

// Save uploads frame to S3 as: {serviceID}/{frameID}.jpg
func (s *S3Storage) Save(serviceID string, frame *Frame)

// GetPresignedURL generates a time-limited URL for accessing a frame
func (s *S3Storage) GetPresignedURL(serviceID, frameID string) (string, error)
```

**Key design decisions**:

- Use `github.com/aws/aws-sdk-go-v2` (add to go.mod)
- S3 key pattern: `{serviceID}/{frameID}.jpg` (matches current path structure)
- Content-Type: `image/jpeg`
- Pass S3 key (not local path) to `onSaved` callback
- Use `s3.PresignClient` for generating presigned URLs

### 2. Update Configuration

**File**: [config.go](server/config.go)

Add new required fields to `Config` struct:

```go
// S3/R2 Storage settings (required)
S3Endpoint     string `json:"s3_endpoint"`      // e.g., https://acaa5d87f5e8badf677c8ccf9be7c2f5.r2.cloudflarestorage.com
S3Region       string `json:"s3_region"`        // "auto" for R2
S3Bucket       string `json:"s3_bucket"`        // "unblink-storage"
S3AccessKey    string `json:"s3_access_key"`
S3SecretKey    string `json:"s3_secret_key"`
```

Remove (no longer needed):

- `FramesBaseDir()` method
- `FramesDir()` method
- `AppDir` field (if only used for frames)

### 3. Update HTTP Handler for Presigned URL Redirect

**File**: [storage.go](server/service/storage.go)

**Implementation**:

```go
func (s *StorageService) serveFrame(w http.ResponseWriter, r *http.Request) {
    // Extract serviceID and frameID from path: /frames/{serviceID}/{frameID}.jpg
    // Generate presigned URL for S3 object
    // HTTP 301 redirect to presigned URL
}
```

**Benefits**:

- Lower server bandwidth (direct client-to-R2 transfer)
- Simple implementation
- Built-in S3 authentication via presigned URLs

### 4. Database Considerations

**File**: [storage.go](server/database/storage.go)

The `storage_path` column will store S3 keys:

- Format: `{serviceID}/{frameID}.jpg`
- Used to generate presigned URLs

No migration needed for new deployments.

### 5. Remove Old Disk Storage

**Delete**: [storage.go](server/webrtc/storage.go) (old disk-based implementation)

**Files to update**:

- Remove `FramesBaseDir()` and `FramesDir()` from [config.go](server/config.go) (no longer needed)
- Update any imports that reference the old `webrtc.Storage`

### 6. Main Integration

**File**: [main.go](cmd/server/main.go)

```go
// Initialize S3 storage (required - fail if S3 config missing)
storage := webrtc.NewS3Storage(
    config.S3Endpoint,
    config.S3Region,
    config.S3Bucket,
    config.S3AccessKey,
    config.S3SecretKey,
)
```

---

## Critical Files

1. **New**: [server/webrtc/s3_storage.go](server/webrtc/s3_storage.go) - S3 storage implementation
2. **Delete**: [server/webrtc/storage.go](server/webrtc/storage.go) - Old disk storage
3. **Modify**: [server/config.go](server/config.go) - Add S3 config, remove `FramesBaseDir()`/`FramesDir()`
4. **Modify**: [server/service/storage.go](server/service/storage.go) - Update HTTP handler for presigned URL redirect
5. **Modify**: [cmd/server/main.go](cmd/server/main.go) - Initialize S3 storage
6. **Modify**: [go.mod](go.mod) - Add AWS SDK v2 dependency

---

## Configuration Example

```json
{
  "listen_addr": ":8080",
  "dashboard_url": "http://localhost:5173",
  "database_url": "postgres://...",
  "jwt_secret": "...",
  "chat_openai_api_key": "...",
  "chat_openai_model": "gpt-4o",

  "s3_endpoint": "https://acaa5d87f5e8badf677c8ccf9be7c2f5.r2.cloudflarestorage.com",
  "s3_region": "auto",
  "s3_bucket": "unblink-storage",
  "s3_access_key": "your_r2_access_key_id",
  "s3_secret_key": "your_r2_secret_access_key"
}
```

---

## Verification Steps

1. **Add dependency**:

   ```bash
   cd /home/tri/unb/server && go get github.com/aws/aws-sdk-go-v2/service/s3
   go get github.com/aws/aws-sdk-go-v2/config
   go get github.com/aws/aws-sdk-go-v2/credentials
   ```

2. **Build check**:

   ```bash
   cd /home/tri/unb/server && go vet ./...
   go build ./...
   ```

3. **Test flow**:
   - Configure S3 credentials in config
   - Start server, extract frames from camera
   - Verify frames appear in R2 bucket
   - Test HTTP endpoint: `curl http://localhost:8080/frames/{serviceID}/{frameID}.jpg`
   - Verify redirect to presigned R2 URL
   - Verify database has correct S3 keys
