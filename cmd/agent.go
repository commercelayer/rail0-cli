package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Agent mode (--agent, or RAIL0_AGENT=1): the CLI as a tool for a program — an AI agent
// first of all — rather than for a person at a terminal.
//
//   - stdout: exactly one JSON object per command, on one line. {"ok":true,"data":…} on
//     success (data is what --json prints), {"ok":false,"error":{…},"next_steps":[…]} on
//     failure (agentErrorPayload).
//   - stderr: JSON Lines events only ({"event":"signed_in",…}, {"event":"progress",…}).
//   - never a prompt, a spinner or colour: a value that a terminal would have asked for
//     is a usage error naming the flag, and destructive commands still need --yes.
//   - the exit code is the same one a script gets (exitCode), and is repeated in the
//     envelope as error.exit_code.
//
// It cannot be combined with --quiet or --pretty: both replace the one shape an agent
// parses. Commands that only make sense with a person in front of them (init, keys
// generate) refuse to run.
var agentFlag bool

// agentMode reports whether agent mode is on, by flag or by environment. The variable
// exists so an agent's harness can set it once instead of adding the flag to every call.
func agentMode() bool {
	if agentFlag {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RAIL0_AGENT"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// interactive reports whether the CLI may ask the user something: stdin is a terminal
// and agent mode is off. An agent may well run the CLI inside a pseudo-terminal, so the
// terminal test alone would let a prompt block it; every prompt goes through this.
func interactive() bool {
	return !agentMode() && term.IsTerminal(os.Stdin.Fd())
}

// usageError is a mistake in how the command was called — an unknown flag, a missing
// argument, a missing --yes, a command that agent mode does not offer — as opposed to a
// failure of the operation. It exits ExitCodeUsage, and Code is its machine code in the
// agent envelope.
type usageError struct {
	Code string
	Msg  string
}

func (e *usageError) Error() string { return e.Msg }

func newUsageError(code, format string, args ...any) error {
	return &usageError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// nextStepError carries what to run next — and whether the operation may still be
// running — alongside an error, for the agent envelope's next_steps and
// error.in_flight. It wraps the original error, so errors.Is/As and the exit code are
// unchanged.
type nextStepError struct {
	err      error
	steps    []string
	inFlight bool
}

func (e *nextStepError) Error() string { return e.err.Error() }
func (e *nextStepError) Unwrap() error { return e.err }

// cobraUsagePrefixes are the starts of the errors cobra and pflag return for a malformed
// command line. They are plain errors, not types, so they are recognised by text; flag
// errors are also wrapped as usageError by the root's FlagErrorFunc.
var cobraUsagePrefixes = []string{
	"unknown command",
	"unknown flag",
	"unknown shorthand flag",
	"flag needs an argument",
	"invalid argument",
	"bad flag syntax",
	"accepts ",
	"requires at least",
	"requires at most",
	"received ",
	"required flag(s)",
	"if any flags in the group",
	"none of the others can be",
	"at least one of the flags",
}

func isUsageError(err error) bool {
	var ue *usageError
	if errors.As(err, &ue) {
		return true
	}
	msg := err.Error()
	for _, p := range cobraUsagePrefixes {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}

// markUsageErrors makes every command's argument-count check return a usageError, so a
// wrong number of arguments exits ExitCodeUsage whatever validator the command uses.
// Applied at Execute, like rejectUnknownSubcommands, so a command added later is covered.
func markUsageErrors(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		markUsageErrors(sub)
	}
	if cmd.Args == nil {
		return
	}
	check := cmd.Args
	cmd.Args = func(c *cobra.Command, args []string) error {
		if err := check(c, args); err != nil {
			return &usageError{Code: "usage_error", Msg: fmt.Sprintf("%v\n\nRun '%s --help' for usage", err, c.CommandPath())}
		}
		return nil
	}
}

// agentErrorPayload is the agent-mode failure envelope, minus the "ok":false that
// ui.FatalAgent adds:
//
//	{"error":{"code","message","exit_code","http_status"?,"reason"?,"hint"?,
//	          "retry_after_s"?,"retryable","in_flight"},"next_steps"?:[…]}
//
// code is the gateway's machine code when the gateway answered, else one of this CLI's:
// usage_error (or the usageError's own code), aborted, wait_timeout, network_error,
// cli_error. message is the same text a person would read. retryable is true only where
// running the identical command again is the right move (a 429, or a gateway code that
// says nothing was submitted). in_flight says an on-chain operation may still be
// running: the caller must check it (next_steps) instead of repeating it.
func agentErrorPayload(cmd *cobra.Command, err error, code int) map[string]any {
	e := map[string]any{
		"code":      agentErrorCode(err, code),
		"message":   humanizeErrorFor(cmd, err).Error(),
		"exit_code": code,
		"retryable": code == ExitCodeRateLimited,
		"in_flight": errors.Is(err, ErrWaitGaveUp),
	}
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status != 0 {
			e["http_status"] = apiErr.Status
		}
		if r := strings.TrimSpace(apiErr.Reason); r != "" {
			e["reason"] = r
		}
		if h := hintFor(apiErr.Code, apiErr.Reason); h != "" {
			e["hint"] = h
		}
		if apiErr.RetryAfter > 0 {
			e["retry_after_s"] = int(apiErr.RetryAfter.Seconds())
		}
		switch apiErr.Code {
		case "rpc_unavailable", "queue_unavailable":
			// Both mean the gateway submitted nothing, so the same command is safe again.
			e["retryable"] = true
		}
	}
	out := map[string]any{"error": e}
	var steps []string
	var ns *nextStepError
	if errors.As(err, &ns) {
		steps = append(steps, ns.steps...)
		if ns.inFlight {
			e["in_flight"] = true
		}
	}
	if code == 3 {
		steps = append(steps, "rail0 auth login -p <key>")
	}
	if len(steps) > 0 {
		out["next_steps"] = steps
	}
	return out
}

// agentErrorCode is error.code in the agent envelope: see agentErrorPayload.
func agentErrorCode(err error, code int) string {
	var ue *usageError
	if errors.As(err, &ue) && ue.Code != "" {
		return ue.Code
	}
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) && apiErr.Code != "" {
		return apiErr.Code
	}
	switch {
	case code == ExitCodeUsage:
		return "usage_error"
	case errors.Is(err, ErrAborted):
		return "aborted"
	case errors.Is(err, ErrWaitGaveUp):
		return "wait_timeout"
	case code == 9:
		return "network_error"
	case apiErr != nil && apiErr.Status != 0:
		return fmt.Sprintf("http_%d", apiErr.Status)
	}
	return "cli_error"
}

// refuseInAgentMode is the guard for a command that needs a person: it returns a
// usageError naming what to do instead, or nil outside agent mode.
func refuseInAgentMode(cmd *cobra.Command, instead string) error {
	if !agentMode() {
		return nil
	}
	return newUsageError("unavailable_in_agent_mode", "'%s' is not available in agent mode: %s", cmd.CommandPath(), instead)
}

// printLine prints a command's one-line, human-only result — text on stdout — or, in
// agent mode, data as the success envelope. For the few commands (version, config path,
// update) whose result was never JSON: the line stays exactly as it was everywhere else.
func printLine(text string, data map[string]any) error {
	if agentMode() {
		return ui.RunAPI(func() (any, error) { return data, nil })
	}
	fmt.Println(text)
	return nil
}
