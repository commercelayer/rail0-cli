package cmd

import (
	"testing"

	rail0 "github.com/rail0/go-sdk"
)

// The exact scenario from #66: a second partial capture. The gateway lists
// transactions newest-first, but capture #2's row is not in the listing yet (the
// 202 has been answered, the row not yet visible to this read), so index 0 is the
// ALREADY CONFIRMED capture #1. Keying on index reported that stale confirmation as
// this attempt's success.
func TestPickWaitTargetIgnoresAnEarlierConfirmedAttempt(t *testing.T) {
	listing := []rail0.Transaction{
		{ID: "tx_capture_1", Operation: "capture", Status: "confirmed"},
	}

	if got := pickWaitTarget(listing, "tx_capture_2"); got != nil {
		t.Fatalf("pickWaitTarget returned %+v for an id not in the listing; want nil so the caller polls again", got)
	}

	// Once the row does appear, it is the one followed — not index 0.
	listing = append([]rail0.Transaction{{ID: "tx_capture_2", Operation: "capture", Status: "submitted"}}, listing...)
	got := pickWaitTarget(listing, "tx_capture_2")
	if got == nil {
		t.Fatal("pickWaitTarget returned nil for an id present in the listing")
	}
	if got.ID != "tx_capture_2" {
		t.Errorf("followed %q, want tx_capture_2", got.ID)
	}
	if got.Status != "submitted" {
		t.Errorf("status = %q, want submitted (the confirmed row is a DIFFERENT attempt)", got.Status)
	}
}

// Order-independence: the row is found wherever the gateway happens to place it.
func TestPickWaitTargetMatchesRegardlessOfPosition(t *testing.T) {
	listing := []rail0.Transaction{
		{ID: "tx_3", Status: "confirmed"},
		{ID: "tx_2", Status: "failed"},
		{ID: "tx_1", Status: "confirmed"},
	}
	for _, want := range []string{"tx_1", "tx_2", "tx_3"} {
		got := pickWaitTarget(listing, want)
		if got == nil || got.ID != want {
			t.Errorf("pickWaitTarget(_, %q) = %+v, want the row with that id", want, got)
		}
	}
}

// Without an id the previous newest-first behaviour is preserved, so a caller with
// no submit response to key on is unaffected.
func TestPickWaitTargetFallsBackToNewestFirst(t *testing.T) {
	if got := pickWaitTarget(nil, ""); got != nil {
		t.Errorf("pickWaitTarget(nil, \"\") = %+v, want nil", got)
	}
	listing := []rail0.Transaction{{ID: "newest"}, {ID: "older"}}
	got := pickWaitTarget(listing, "")
	if got == nil || got.ID != "newest" {
		t.Errorf("pickWaitTarget(_, \"\") = %+v, want the newest row", got)
	}
}

// submittedTransactionID must read the id out of what the submit methods actually
// return, and degrade to "" (newest-first) on anything else rather than panicking
// after the operation has already been broadcast.
func TestSubmittedTransactionID(t *testing.T) {
	if got := submittedTransactionID(&rail0.Transaction{ID: "tx_42"}); got != "tx_42" {
		t.Errorf("submittedTransactionID(*Transaction) = %q, want tx_42", got)
	}
	for _, other := range []any{nil, "a string", rail0.Transaction{ID: "not a pointer"}, &rail0.Payment{}} {
		if got := submittedTransactionID(other); got != "" {
			t.Errorf("submittedTransactionID(%T) = %q, want \"\"", other, got)
		}
	}
}
