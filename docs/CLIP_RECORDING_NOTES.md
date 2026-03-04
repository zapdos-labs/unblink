## Clip Recording Notes

This document records the FFmpeg experiments around making an actively written clip viewable while keeping the current recording architecture.

### Goal

Keep the existing architecture:

- one long-running FFmpeg process
- FFmpeg `segment` muxer owns clip boundaries
- one file per clip

but make the currently active clip viewable before the segment closes.

### Environment

Tested locally with:

- `ffmpeg version 8.0.1`

### What Was Tested

#### 1. Single-file fragmented MP4

Test command shape:

```bash
ffmpeg ... \
  -flush_packets 1 \
  -movflags +hybrid_fragmented+frag_keyframe+empty_moov+default_base_moof \
  -f mp4 active.mp4
```

Observed result:

- the file was readable by `ffprobe` while still being written

Important detail:

- `-flush_packets 1` mattered on this machine

#### 2. FFmpeg `segment` muxer with fragmented MP4 options

Tested combinations included:

- `movflags=+frag_keyframe+empty_moov+default_base_moof`
- `movflags=+hybrid_fragmented+frag_keyframe+empty_moov+default_base_moof`
- `movflags=+dash+frag_keyframe+empty_moov+default_base_moof`
- with and without `-flush_packets 1`

Observed result:

- the active segment still was not readable while open
- the file often stayed extremely small while active
- `ffprobe` failed with `moov atom not found`

So on this FFmpeg build, fragmented MP4 plus the `segment` muxer did not give a live-viewable active segment.

### Experimental Architectural Change

A follow-up experiment switched to app-managed rotation:

- one FFmpeg process per clip
- the app decided 15-minute boundaries
- each process wrote one fragmented MP4 file

That version did produce a viewable actively written file.

However, it is a larger architectural change because clip rotation moves from FFmpeg into the application. That change was reverted.

### Current Decision

The code has been reverted to:

- regular MP4 clips
- FFmpeg `segment` muxer
- no app-managed clip rotation

Current tradeoff:

- finalized clips are normal MP4 and play reliably
- the active clip is not presented as a finished `.mp4` file until the segment closes

### Future Options

If live-viewable active clips become a hard requirement, revisit one of these explicitly:

1. Accept app-managed clip rotation with one FFmpeg process per clip.
2. Move to a true streaming artifact model such as HLS/CMAF or DASH instead of standalone clip files.
3. Re-test newer FFmpeg builds to see whether `segment` plus fragmented MP4 behavior changes.
