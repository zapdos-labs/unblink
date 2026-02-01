# Event Indexing Granularity

## Motivation

We add multi-level granularity to help with RAG (AI) & summarization (code).

The **midpoint strategy** prevents semantic confusion: a 23-hour span is labeled `"day"` (not `"hour"`) because it's closer to a day than an hour. This aligns with human intuition and AI reasoning.

## Granularity Format

| Unit | Range (seconds) | Comment |
|------|-----------------|---------|
| `"second"` | 0 - 29 | < 30s (half minute) |
| `"minute"` | 30 - 1799 | 30s - 29m 59s (half hour) |
| `"hour"` | 1800 - 43199 | 30m - 11h 59m (half day) |
| `"day"` | 43200 - 604799 | 12h - 6d 23h (half week) |
| `"week"` | 604800 - 1209599 | 7d - 13d 23h (half month) |
| `"month"` | 1209600+ | ≥ 14 days (half month) |

## Event Payload

```json
{
  "id": "uuid",
  "serviceId": "service-id",
  "payload": {
    "type": "vlm-indexing",
    "granularity": "minute",
    "from_iso": "2026-02-01T10:00:00Z",
    "to_iso": "2026-02-01T10:00:15Z",
    "response": {
      "description": "...",
      "objects": [...]
    }
  },
  "createdAt": "2026-02-01T10:00:00Z"
}
```

## Example Calculation

Default config:
- `frame_interval_seconds`: 5.0 (extract frame every 5 seconds)
- `frame_batch_size`: 3 (batch 3 frames before VLM processing)
- Time span per event: 3 frames × 5 seconds = **15 seconds**
- Falls in range 0-29s → `"second"`
