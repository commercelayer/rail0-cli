package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// `payments redrive --wait` follows the redriven transaction by its own id and
// operation, exactly as a lifecycle command's --wait follows the one it submitted.
func TestRedriveWaitsOnTheRedrivenTransaction(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	fastPolls(t)
	var redriven bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/payments/0xab/transactions/tx-9/redrive":
			redriven = true
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tx-9", "operation": "capture", "status": "pending"})
		case r.URL.Path == "/payments/0xab/transactions":
			if got := r.URL.Query().Get("operation"); got != "capture" {
				t.Errorf("wait polled operation %q, want the redriven one's (capture)", got)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "tx-9", "operation": "capture", "status": "confirmed"}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "status": "captured"})
		}
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	waitFlag, waitTimeout = true, defaultWaitTimeout
	t.Cleanup(func() { waitFlag = false })

	out := captureStdout(t, func() {
		if err := paymentsRedriveCmd.RunE(paymentsRedriveCmd, []string{"0xab", "tx-9"}); err != nil {
			t.Errorf("redrive: %v", err)
		}
	})
	if !redriven {
		t.Fatal("POST .../redrive was never sent")
	}
	if !strings.Contains(out, `"status": "captured"`) {
		t.Errorf("with --wait the result is the settled payment, got:\n%s", out)
	}
}

// The vocabularies behind --operation/--status must be the gateway's own
// (TRANSACTION_OPERATIONS and the transactions status filter): a value outside them
// is a 400, one missing cannot be filtered on.
func TestTransactionVocabulariesMirrorTheGateway(t *testing.T) {
	if !contains(transactionOperations, "close_dispute") || !contains(transactionOperations, "dispute") {
		t.Error("the dispute operations are transaction operations and must be filterable")
	}
	if !contains(transactionStatuses, "failed") || contains(paymentStatuses, "failed") {
		t.Error("failed is a transaction status, never a payment one")
	}
	if len(transactionOperations) != 8 || len(transactionStatuses) != 5 {
		t.Errorf("vocabulary drifted: %v / %v", transactionOperations, transactionStatuses)
	}
}
