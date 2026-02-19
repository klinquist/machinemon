package store

import (
	"testing"
	"time"
)

func TestSessionStartAtFromBootTime(t *testing.T) {
	now := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	boot := now.Add(-37 * time.Minute).Unix()

	got := sessionStartAt(now, boot)
	want := time.Unix(boot, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestSessionStartAtMissingBootTimeFallsBackToNow(t *testing.T) {
	now := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	got := sessionStartAt(now, 0)
	if !got.Equal(now) {
		t.Fatalf("expected %s, got %s", now, got)
	}
}

func TestSessionStartAtFutureBootTimeFallsBackToNow(t *testing.T) {
	now := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	boot := now.Add(6 * time.Minute).Unix()

	got := sessionStartAt(now, boot)
	if !got.Equal(now) {
		t.Fatalf("expected %s, got %s", now, got)
	}
}
