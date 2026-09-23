package cmd

// --idempotency-key routing (commercelayer/rail0-gateway#331) and the Link-driven
// paging stop (#332).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

func TestPrepareOpSendsTheKeyOnlyWhenTheFlagIsSet(t *testing.T) {
	var seen http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "tx-1"})
	}))
	defer srv.Close()

	client := rail0.NewClient(rail0.ClientOptions{BaseURL: srv.URL})
	id := rail0.Bytes32("0x" + "ab")

	// Flag set: the header travels.
	idempotencyKey = "k-1"
	if _, err := prepareOp(context.Background(), client, id, rail0.OpCapture, rail0.PrepareRequest{Amount: "1.00"}); err != nil {
		t.Fatalf("prepareOp with key: %v", err)
	}
	if got := seen.Get("Idempotency-Key"); got != "k-1" {
		t.Errorf("Idempotency-Key: got %q, want k-1", got)
	}

	// Flag unset: an un-keyed run must be byte-for-byte what it was before.
	idempotencyKey = ""
	if _, err := prepareOp(context.Background(), client, id, rail0.OpCapture, rail0.PrepareRequest{Amount: "1.00"}); err != nil {
		t.Fatalf("prepareOp without key: %v", err)
	}
	if got := seen.Get("Idempotency-Key"); got != "" {
		t.Errorf("Idempotency-Key must be absent without the flag, got %q", got)
	}
}

// A keyed re-run gets the ORIGINAL transaction back. Only a row still `pending` may be
// signed and submitted again; one that already left it must be followed, not re-sent —
// submit accepts only `pending` rows, so re-sending answered 404 on every re-run.
func TestReplayedPastPending(t *testing.T) {
	t.Cleanup(func() { idempotencyKey = "" })

	idempotencyKey = "k-1"
	for status, want := range map[string]bool{
		"pending": false, "submitting": true, "submitted": true, "confirmed": true, "failed": true,
	} {
		if got := replayedPastPending(&rail0.Transaction{ID: "tx-1", Status: status}); got != want {
			t.Errorf("status %q: replayedPastPending = %v, want %v", status, got, want)
		}
	}
	// Pending but already signed and held by the gateway: the first attempt's submit
	// landed, so a re-run must follow it, not sign it again (which answered 404).
	if !replayedPastPending(&rail0.Transaction{ID: "tx-1", Status: "pending", Redrivable: true}) {
		t.Error("a redrivable pending row is past the caller's part and must be followed")
	}
	// A refund phase-1 answer carries no row: never a replay.
	if replayedPastPending(&rail0.Transaction{SigningPayload: &rail0.SigningPayload{}}) {
		t.Error("a signing payload is not a replayed transaction")
	}

	// Without a key the gateway never replays, whatever the row says.
	idempotencyKey = ""
	if replayedPastPending(&rail0.Transaction{ID: "tx-1", Status: "confirmed"}) {
		t.Error("an un-keyed prepare must never be treated as a replay")
	}
}

// With --wait a replay waits on the replayed transaction's own id — and never
// submits anything.
func TestResumeReplayedWaitsOnTheOriginalTransaction(t *testing.T) {
	fastPolls(t)
	var submitted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			submitted = true
			w.WriteHeader(404)
		case r.URL.Path == "/payments/0xab/transactions":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "tx-2", "operation": "capture", "status": "submitted"},
				{"id": "tx-1", "operation": "capture", "status": "confirmed"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "status": "captured"})
		}
	}))
	defer srv.Close()
	client := rail0.NewClient(rail0.ClientOptions{BaseURL: srv.URL})

	waitFlag = true
	t.Cleanup(func() { waitFlag = false })
	got, err := resumeReplayed(context.Background(), client, "0xab", "capture",
		&rail0.Transaction{ID: "tx-1", Status: "submitted"})
	if err != nil {
		t.Fatalf("resumeReplayed: %v", err)
	}
	if p, ok := got.(*rail0.Payment); !ok || p.Status != "captured" {
		t.Errorf("result = %+v, want the settled payment", got)
	}
	if submitted {
		t.Error("a replay must never be submitted again")
	}
}
