package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
)

func withAgentMode(t *testing.T) {
	t.Helper()
	agentFlag = true
	applyOutputMode()
	t.Cleanup(func() { agentFlag = false; applyOutputMode() })
}

func TestAgentModeFromEnvironment(t *testing.T) {
	for _, v := range []string{"1", "true", "YES", "on"} {
		t.Setenv("RAIL0_AGENT", v)
		if !agentMode() {
			t.Errorf("RAIL0_AGENT=%q: agent mode off", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no"} {
		t.Setenv("RAIL0_AGENT", v)
		if agentMode() {
			t.Errorf("RAIL0_AGENT=%q: agent mode on", v)
		}
	}
}

// An agent may run the CLI in a pseudo-terminal: agent mode never prompts, whatever
// stdin is.
func TestAgentModeIsNeverInteractive(t *testing.T) {
	withAgentMode(t)
	if interactive() {
		t.Error("interactive() is true in agent mode")
	}
}

// A malformed command line exits 2, whether cobra or the CLI noticed.
func TestUsageErrorsExitTwo(t *testing.T) {
	for _, err := range []error{
		newUsageError("usage_error", "a payment id is required"),
		errors.New(`unknown flag: --nope`),
		errors.New(`accepts 1 arg(s), received 0`),
		errors.New(`required flag(s) "amount" not set`),
		errors.New(`unknown command "lsit" for "rail0 webhooks"`),
	} {
		if got := exitCode(err); got != ExitCodeUsage {
			t.Errorf("exitCode(%q) = %d, want %d", err, got, ExitCodeUsage)
		}
	}
}

// Destructive commands still need --yes in agent mode: agent mode is not consent.
func TestAgentModeStillNeedsYes(t *testing.T) {
	withAgentMode(t)
	assumeYes = false
	err := confirmDestructive("Capture?")
	if exitCode(err) != ExitCodeUsage || agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Errorf("got %v (exit %d)", err, exitCode(err))
	}
}

func TestKeysGenerateRefusedInAgentMode(t *testing.T) {
	withAgentMode(t)
	err := keysGenerateCmd.RunE(keysGenerateCmd, nil)
	if err == nil || agentErrorCode(err, exitCode(err)) != "unavailable_in_agent_mode" || exitCode(err) != ExitCodeUsage {
		t.Fatalf("got %v", err)
	}
}

func TestAgentModeRefusesQuietAndPretty(t *testing.T) {
	withAgentMode(t)
	for _, set := range []*bool{&quietOut, &prettyOut} {
		*set = true
		err := rootCmd.PersistentPreRunE(versionCmd, nil)
		*set = false
		if agentErrorCode(err, exitCode(err)) != "flag_conflict" {
			t.Errorf("got %v", err)
		}
	}
}

func TestAgentErrorPayloadForAGatewayRejection(t *testing.T) {
	err := &rail0.APIError{Status: 422, Code: "not_capturable", Detail: "payment is captured"}
	p := agentErrorPayload(nil, err, exitCode(err))
	e := p["error"].(map[string]any)
	if e["code"] != "not_capturable" || e["http_status"] != 422 || e["exit_code"] != 7 ||
		e["retryable"] != false || e["in_flight"] != false {
		t.Errorf("error = %v", e)
	}
	if h, _ := e["hint"].(string); !strings.Contains(h, "authorized") {
		t.Errorf("hint = %q", h)
	}
	if !strings.Contains(e["message"].(string), "request rejected (HTTP 422)") {
		t.Errorf("message = %q", e["message"])
	}
}

func TestAgentErrorPayloadRetryable(t *testing.T) {
	limited := &rail0.APIError{Status: 429, Code: "rate_limited", RetryAfter: 30 * time.Second}
	e := agentErrorPayload(nil, limited, exitCode(limited))["error"].(map[string]any)
	if e["retryable"] != true || e["retry_after_s"] != 30 || e["exit_code"] != ExitCodeRateLimited {
		t.Errorf("429 = %v", e)
	}
	rpc := &rail0.APIError{Status: 503, Code: "rpc_unavailable"}
	if e := agentErrorPayload(nil, rpc, exitCode(rpc))["error"].(map[string]any); e["retryable"] != true {
		t.Errorf("rpc_unavailable = %v", e)
	}
	server := &rail0.APIError{Status: 500, Code: "internal_error"}
	if e := agentErrorPayload(nil, server, exitCode(server))["error"].(map[string]any); e["retryable"] != false {
		t.Errorf("500 = %v", e)
	}
}

// A wait that gave up is in flight: exit 11, never "retry", and the check to run next.
func TestAgentErrorPayloadForAWaitThatGaveUp(t *testing.T) {
	err := gaveUpError(time.Minute, "rail0 payments wait 0xabc --until captured", nil)
	code := exitCode(err)
	p := agentErrorPayload(nil, err, code)
	e := p["error"].(map[string]any)
	if code != ExitCodeInFlight || e["code"] != "wait_timeout" || e["in_flight"] != true || e["retryable"] != false {
		t.Errorf("error = %v (exit %d)", e, code)
	}
	steps, _ := p["next_steps"].([]string)
	if len(steps) != 1 || steps[0] != "rail0 payments wait 0xabc --until captured" {
		t.Errorf("next_steps = %v", p["next_steps"])
	}
	// Ctrl-C during the wait: aborted (130), but still in flight.
	err = interruptedError("rail0 payments wait 0xabc")
	e = agentErrorPayload(nil, err, exitCode(err))["error"].(map[string]any)
	if e["code"] != "aborted" || e["in_flight"] != true || exitCode(err) != ExitCodeAborted {
		t.Errorf("interrupted = %v", e)
	}
}

func TestAgentVersionIsAnEnvelope(t *testing.T) {
	withAgentMode(t)
	out := captureStdout(t, func() { _ = versionCmd.RunE(versionCmd, nil) })
	if !strings.HasPrefix(out, `{"ok":true,"data":{`) || !strings.Contains(out, `"version":`) {
		t.Errorf("got %q", out)
	}
}
