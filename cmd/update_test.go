package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rail0/cli/internal/config"
)

func TestPromptYesDefaultsToYes(t *testing.T) {
	cases := map[string]bool{
		"\n":      true, // Enter accepts: the question is phrased for it
		"y\n":     true,
		"Yes\n":   true,
		"n\n":     false,
		"no\n":    false,
		"later\n": false,
		"":        false, // EOF (stdin closed) never installs
	}
	for input, want := range cases {
		var out bytes.Buffer
		if got := promptYes(strings.NewReader(input), &out, "Update now?"); got != want {
			t.Errorf("answer %q: got %v, want %v", input, got, want)
		}
		if !strings.Contains(out.String(), "Update now? [Y/n]") {
			t.Errorf("prompt not shown: %q", out.String())
		}
	}
}

// The automatic check must never interrupt a script: any one of these alone keeps it
// quiet. (The TTY conditions are also false under `go test`, which is the point —
// the env/flag cases are asserted with a release version so they are not masked.)
func TestUpdateCheckStaysOutOfScripts(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved; jsonOut, quietOut = false, false })

	version = "dev"
	if updateCheckWanted() {
		t.Error("a development build must never check")
	}

	version = "1.3.0"
	t.Setenv("CI", "true")
	if updateCheckWanted() {
		t.Error("CI must never check")
	}
	t.Setenv("CI", "")
	t.Setenv("RAIL0_NO_UPDATE_CHECK", "1")
	if updateCheckWanted() {
		t.Error("RAIL0_NO_UPDATE_CHECK must disable the check")
	}
	t.Setenv("RAIL0_NO_UPDATE_CHECK", "")
	jsonOut = true
	if updateCheckWanted() {
		t.Error("--json must never check")
	}
	jsonOut, quietOut = false, true
	if updateCheckWanted() {
		t.Error("--quiet must never check")
	}
}

// A stored (or RAIL0_OUTPUT) output default of json or quiet is a scripting setup too:
// the check stays out of it unless an explicit output flag says otherwise. Asserted on
// scriptedOutput directly, since updateCheckWanted is already false under `go test`
// (no TTY) and would mask the case.
func TestUpdateCheckHonoursTheConfiguredOutput(t *testing.T) {
	isolate(t)
	t.Cleanup(func() { jsonOut, quietOut, prettyOut = false, false, false })
	t.Setenv("RAIL0_OUTPUT", "")

	if scriptedOutput() {
		t.Fatal("no flag and no stored output must not count as scripted")
	}
	for _, mode := range []string{"json", "quiet"} {
		t.Setenv("RAIL0_OUTPUT", mode)
		if !scriptedOutput() {
			t.Errorf("RAIL0_OUTPUT=%s must keep the update check out", mode)
		}
	}
	t.Setenv("RAIL0_OUTPUT", "")

	cfg, _, err := config.LoadForWrite()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings == nil {
		cfg.Settings = map[string]string{}
	}
	cfg.Settings["output"] = "quiet"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if !scriptedOutput() {
		t.Error("a stored `output: quiet` must keep the update check out")
	}
	// An explicit flag wins over the stored default, as it does for rendering.
	prettyOut = true
	if scriptedOutput() {
		t.Error("--pretty over a stored quiet is interactive output")
	}
	prettyOut = false

	for _, mode := range []string{"auto", "pretty"} {
		cfg.Settings["output"] = mode
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		if scriptedOutput() {
			t.Errorf("a stored `output: %s` is interactive and must allow the check", mode)
		}
	}
}

func TestUpdateIsALocalCommand(t *testing.T) {
	// No gateway involved: the base-URL check must not stand in front of it.
	if !localOnlyCommands["update"] {
		t.Error("update must be exempt from the base-URL check")
	}
	if !updateCheckSkipped["update"] || !updateCheckSkipped["completion"] {
		t.Error("the automatic check must skip update itself and shell completion")
	}
}
