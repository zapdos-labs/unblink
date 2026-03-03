package node

import (
	"testing"
	"time"
)

func TestShouldResetBackoff(t *testing.T) {
	r := &Reconnector{stableThreshold: time.Minute}

	if r.shouldResetBackoff(59 * time.Second) {
		t.Fatal("expected backoff to remain for short-lived connection")
	}

	if !r.shouldResetBackoff(time.Minute) {
		t.Fatal("expected backoff reset at the stability threshold")
	}

	if !r.shouldResetBackoff(3 * time.Minute) {
		t.Fatal("expected backoff reset for stable connection")
	}
}
