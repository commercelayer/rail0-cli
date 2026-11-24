package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// cobra sets the --agent flag only after it has found the command, so a call it rejects
// before that (an unknown command) ran with the flag unseen and answered in text.
func TestAgentFlagIsSeenInTheRawArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--agent", "nope"},
		{"nope", "--agent"},
		{"--agent=true", "payments"},
		{"payments", "get", "--agent=1"},
	} {
		if !agentInArgs(args) {
			t.Errorf("agentInArgs(%q) = false", args)
		}
	}
	for _, args := range [][]string{
		{},
		{"payments", "list"},
		{"--agent=false", "chains"},
		{"webhooks", "create", "--", "--agent"}, // after "--" it is an argument
		{"--agents"},
	} {
		if agentInArgs(args) {
			t.Errorf("agentInArgs(%q) = true", args)
		}
	}
}

// Help is one JSON line in agent mode, like every other call: --help, 'help <command>'
// and a bare group all come through the root's help function.
func TestAgentHelpIsAnEnvelope(t *testing.T) {
	withAgentMode(t)
	out := captureStdout(t, func() { paymentsCmd.HelpFunc()(paymentsCmd, nil) })
	if strings.Count(strings.TrimRight(out, "\n"), "\n") != 0 {
		t.Fatalf("help is not one line: %q", out)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Command string `json:"command"`
			Help    string `json:"help"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("help is not JSON: %v\n%s", err, out)
	}
	if !env.OK || env.Data.Command != "rail0 payments" {
		t.Errorf("got ok=%v command=%q", env.OK, env.Data.Command)
	}
	for _, want := range []string{"Manage the full RAIL0 payment lifecycle", "Usage:", "capture", "--agent"} {
		if !strings.Contains(env.Data.Help, want) {
			t.Errorf("help lacks %q", want)
		}
	}
}

// Outside agent mode help is the text it always was.
func TestHelpIsTextOutsideAgentMode(t *testing.T) {
	var buf strings.Builder
	paymentsCmd.SetOut(&buf)
	t.Cleanup(func() { paymentsCmd.SetOut(nil) })
	paymentsCmd.HelpFunc()(paymentsCmd, nil)
	if out := buf.String(); strings.HasPrefix(out, "{") || !strings.Contains(out, "Usage:") {
		t.Errorf("got %q", out)
	}
}

// silentStdin swaps stdin for a pipe nobody writes to and nobody closes until the test
// ends: what an agent's harness leaves a command with.
func silentStdin(t *testing.T) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = w.Close(); _ = r.Close() })
}

// No key in agent mode is a usage error naming the flag, at once. It used to read the
// key from stdin and wait for a line that never came.
func TestAgentModeNeverReadsTheKeyFromStdin(t *testing.T) {
	withAgentMode(t)
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	silentStdin(t)

	done := make(chan error, 1)
	go func() { _, err := signingKeyOrPrompt(""); done <- err }()
	select {
	case err := <-done:
		if exitCode(err) != ExitCodeUsage || agentErrorCode(err, exitCode(err)) != "missing_private_key" {
			t.Errorf("got %v (exit %d)", err, exitCode(err))
		}
		if !strings.Contains(err.Error(), "--private-key") || !strings.Contains(err.Error(), "RAIL0_PRIVATE_KEY") {
			t.Errorf("the message does not name how to give the key: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("signingKeyOrPrompt is waiting on stdin in agent mode")
	}
}

// A script that pipes nothing: the same usage error, not "read key from stdin: EOF".
func TestAnEmptyPipeIsAMissingKey(t *testing.T) {
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = r.Close() })

	_, err = signingKeyOrPrompt("")
	if exitCode(err) != ExitCodeUsage || agentErrorCode(err, exitCode(err)) != "missing_private_key" {
		t.Errorf("got %v (exit %d)", err, exitCode(err))
	}
}

// 'keys add' reads a private key from the terminal or a pipe: no place for an agent.
func TestKeysAddRefusedInAgentMode(t *testing.T) {
	withAgentMode(t)
	err := keysAddCmd.RunE(keysAddCmd, []string{"payer"})
	if err == nil || agentErrorCode(err, exitCode(err)) != "unavailable_in_agent_mode" || exitCode(err) != ExitCodeUsage {
		t.Fatalf("got %v", err)
	}
}

// No stored session is the local twin of the gateway's 401: exit 3, a code of its own
// and the sign-in as the next step. It was exit 1, cli_error.
func TestNoSessionExitsThree(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	for name, call := range map[string]func() (string, error){
		"currentAccountID": currentAccountID,
		"sessionAddress":   sessionAddress,
		"resolveAccountID": func() (string, error) { return resolveAccountID("") },
	} {
		_, err := call()
		if exitCode(err) != 3 || agentErrorCode(err, exitCode(err)) != "not_logged_in" {
			t.Errorf("%s: got %v (exit %d, code %s)", name, err, exitCode(err), agentErrorCode(err, exitCode(err)))
		}
	}
	_, err := currentAccountID()
	payload := agentErrorPayload(accountCmd, err, exitCode(err))
	steps, _ := payload["next_steps"].([]string)
	if len(steps) == 0 || !strings.Contains(steps[0], "rail0 auth login") {
		t.Errorf("next_steps = %v", payload["next_steps"])
	}
}

// A mistake in the call exits 2 with usage_error, whoever noticed it; the cause of a
// wrapped one stays reachable.
func TestCallMistakesAreUsageErrors(t *testing.T) {
	cause := errors.New("encoding/hex: invalid byte")
	for name, err := range map[string]error{
		"usagef":              usagef("--scope must be mine or all, got %q", "everyone"),
		"usageWrap":           usageWrap(cause, "invalid private key: %v", cause),
		"wrapped again":       fmt.Errorf("sign: %w", usageWrap(cause, "--from: %v", cause)),
		"payment-methods":     paymentMethodsCmd.RunE(paymentMethodsCmd, nil),
		"missing private key": missingKeyError(),
	} {
		if exitCode(err) != ExitCodeUsage {
			t.Errorf("%s: exit %d, want %d (%v)", name, exitCode(err), ExitCodeUsage, err)
		}
	}
	if err := usageWrap(cause, "invalid private key: %v", cause); !errors.Is(err, cause) {
		t.Error("usageWrap lost its cause")
	}
	if got := agentErrorCode(usagef("x"), ExitCodeUsage); got != "usage_error" {
		t.Errorf("code = %q", got)
	}
}

// A plain error whose message starts with a flag's name is a mistake in the call that
// nobody classed: it would exit 1, "the operation failed", where 2 says "fix the call".
func TestNoFlagErrorIsLeftUnclassed(t *testing.T) {
	plain := regexp.MustCompile(`(fmt\.Errorf|errors\.New)\("--`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if plain.MatchString(line) {
				t.Errorf("%s:%d: use usagef or usageWrap: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// 'help <not a command>' is a usage error; a real topic is still served.
func TestUnknownHelpTopicIsAUsageError(t *testing.T) {
	rejectUnknownHelpTopics(rootCmd)
	help, _, err := rootCmd.Find([]string{"help"})
	if err != nil || help.RunE == nil {
		t.Fatalf("help command not wrapped: %v", err)
	}
	if err := help.RunE(help, []string{"nope"}); exitCode(err) != ExitCodeUsage || !strings.Contains(err.Error(), `unknown help topic "nope"`) {
		t.Errorf("got %v (exit %d)", err, exitCode(err))
	}
	withAgentMode(t)
	out := captureStdout(t, func() {
		if err := help.RunE(help, []string{"payments"}); err != nil {
			t.Errorf("help payments: %v", err)
		}
	})
	if !strings.HasPrefix(out, `{"ok":true,"data":{"command":"rail0 payments"`) {
		t.Errorf("got %q", out)
	}
}

// The session token is a credential: agent mode does not print it, by either door.
func TestSessionTokenIsNotPrintedInAgentMode(t *testing.T) {
	withAgentMode(t)
	err := tokenCmd.RunE(tokenCmd, nil)
	if err == nil || agentErrorCode(err, exitCode(err)) != "unavailable_in_agent_mode" || exitCode(err) != ExitCodeUsage {
		t.Errorf("rail0 token: got %v", err)
	}

	loginShowToken = true
	t.Cleanup(func() { loginShowToken = false })
	err = authLoginCmd.RunE(authLoginCmd, nil)
	if err == nil || agentErrorCode(err, exitCode(err)) != "unavailable_in_agent_mode" || !strings.Contains(err.Error(), "--token") {
		t.Errorf("auth login --token: got %v", err)
	}
}

// Outside agent mode the command is what it was: no session is a hint, not a refusal.
func TestTokenCommandOutsideAgentMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := tokenCmd.RunE(tokenCmd, nil); err != nil {
		t.Errorf("got %v", err)
	}
}
