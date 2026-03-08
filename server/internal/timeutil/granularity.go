package timeutil

import "time"

// GranularityLevel represents a time granularity level
type GranularityLevel string

const (
	GranularitySecond GranularityLevel = "second"
	GranularityMinute GranularityLevel = "minute"
	GranularityHour   GranularityLevel = "hour"
	GranularityDay    GranularityLevel = "day"
	GranularityWeek   GranularityLevel = "week"
	GranularityMonth  GranularityLevel = "month"
)

// CalculateGranularity computes the appropriate granularity level
// for a given duration in seconds using midpoint strategy.
// Range mapping (per EVENT_GRANULARITY.md):
//   - 0-29s       → "second"
//   - 30-1799s    → "minute"  (30s to ~30min)
//   - 1800-43199s → "hour"    (~30min to ~12hr)
//   - 43200-604799s → "day"   (~12hr to ~7day)
//   - 604800-1209599s → "week" (~7day to ~14day)
//   - 1209600+    → "month"  (>=14day)
func CalculateGranularity(durationSeconds int64) GranularityLevel {
	switch {
	case durationSeconds < 30:
		return GranularitySecond
	case durationSeconds < 1800:
		return GranularityMinute
	case durationSeconds < 43200:
		return GranularityHour
	case durationSeconds < 604800:
		return GranularityDay
	case durationSeconds < 1209600:
		return GranularityWeek
	default:
		return GranularityMonth
	}
}

// NextGranularity returns the next coarser granularity level.
func NextGranularity(level GranularityLevel) (GranularityLevel, bool) {
	switch level {
	case GranularitySecond:
		return GranularityMinute, true
	case GranularityMinute:
		return GranularityHour, true
	case GranularityHour:
		return GranularityDay, true
	case GranularityDay:
		return GranularityWeek, true
	case GranularityWeek:
		return GranularityMonth, true
	default:
		return "", false
	}
}

// MinSecondsForGranularity returns the minimum duration in seconds for a granularity.
func MinSecondsForGranularity(level GranularityLevel) int64 {
	switch level {
	case GranularitySecond:
		return 0
	case GranularityMinute:
		return 30
	case GranularityHour:
		return 1800
	case GranularityDay:
		return 43200
	case GranularityWeek:
		return 604800
	case GranularityMonth:
		return 1209600
	default:
		return 0
	}
}

// FormatToISO formats a time.Time to ISO 8601 string with UTC (e.g., "2026-02-01T10:00:00Z")
func FormatToISO(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
