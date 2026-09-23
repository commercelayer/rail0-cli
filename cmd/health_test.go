package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

// A 503 from /health carries the report — which part is down. It is printed, and the
// command still fails with the 503 so a script's exit-code check is unchanged.
func TestHealthPrintsTheReportOfAnUnhealthyGateway(t *testing.T) {
	isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"error","db":"error","timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	var err error
	out := captureStdout(t, func() { err = healthCmd.RunE(healthCmd, nil) })

	var apiErr *rail0.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 503 {
		t.Fatalf("err = %v, want the 503", err)
	}
	if exitCode(err) != 8 {
		t.Errorf("exit code = %d, want 8 (upstream)", exitCode(err))
	}
	if !strings.Contains(out, `"db": "error"`) {
		t.Errorf("the report must be printed, got:\n%s", out)
	}
}
