package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressBarThrottlesAndFinishes(t *testing.T) {
	var out bytes.Buffer
	clock := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	bar := NewProgressBar(&out, "Downloading rail0 1.4.0")
	bar.now = func() time.Time { return clock }

	bar.Update(0, 4<<20)
	first := out.Len()
	if first == 0 {
		t.Fatal("first update drew nothing")
	}

	clock = clock.Add(10 * time.Millisecond)
	bar.Update(1<<20, 4<<20)
	if out.Len() != first {
		t.Error("redrew within 100ms; want throttled")
	}

	// The last frame is always drawn, however soon it comes, so the bar never
	// stops short of 100%.
	clock = clock.Add(10 * time.Millisecond)
	bar.Update(4<<20, 4<<20)
	bar.Done()
	s := out.String()
	for _, want := range []string{"Downloading rail0 1.4.0", "100%", "4.0/4.0 MB"} {
		if !strings.Contains(s, want) {
			t.Errorf("output %q lacks %q", s, want)
		}
	}
	if !strings.HasSuffix(s, "\n") {
		t.Error("Done did not end the line")
	}
}

func TestProgressBarWithoutATotalShowsBytes(t *testing.T) {
	var out bytes.Buffer
	bar := NewProgressBar(&out, "Downloading")
	bar.Update(3<<20, 0)
	if s := out.String(); !strings.Contains(s, "3.0 MB") || strings.Contains(s, "%") {
		t.Errorf("got %q, want a byte count and no percentage", s)
	}
}
