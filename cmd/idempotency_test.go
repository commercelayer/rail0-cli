package cmd

// --idempotency-key routing (commercelayer/rail0-gateway#331) and the Link-driven
// paging stop (#332).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rail0 "github.com/rail0/go-sdk"
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
