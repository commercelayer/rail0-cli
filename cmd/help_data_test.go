package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func indexPaths(t *testing.T, group *cobra.Command) map[string]map[string]any {
	t.Helper()
	entries, _ := commandIndex(group)["commands"].([]map[string]any)
	out := map[string]map[string]any{}
	for _, e := range entries {
		out[e["path"].(string)] = e
	}
	return out
}

// The index lists every command a caller can run, and none it cannot: it is read from
// the tree, and this is what keeps the reading honest.
func TestCatalogueListsEveryRunnableCommand(t *testing.T) {
	listed := indexPaths(t, rootCmd)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if !describable(sub) {
				if _, ok := listed[sub.CommandPath()]; ok {
					t.Errorf("%s is hidden and listed", sub.CommandPath())
				}
				continue
			}
			if sub.Runnable() && !isBareGroup(sub) {
				if _, ok := listed[sub.CommandPath()]; !ok {
					t.Errorf("%s is runnable and not listed", sub.CommandPath())
				}
			}
			walk(sub)
		}
	}
	walk(rootCmd)
	for _, want := range []string{"rail0 payments capture", "rail0 payments create", "rail0 auth login", "rail0 health"} {
		if _, ok := listed[want]; !ok {
			t.Errorf("%s is not listed", want)
		}
	}
	for _, unwanted := range []string{"rail0 env", "rail0 help", "rail0 completion", "rail0 payments"} {
		if _, ok := listed[unwanted]; ok {
			t.Errorf("%s is listed", unwanted)
		}
	}
}

// A command's entry names its real flags (the guess that started this was --payee on
// 'payments create', where the flag is --to) and what running it involves.
func TestCatalogueDescribesACommand(t *testing.T) {
	listed := indexPaths(t, rootCmd)
	create := listed["rail0 payments create"]
	flags := strings.Join(create["flags"].([]string), " ")
	for _, want := range []string{"--from", "--to", "--amount", "--private-key"} {
		if !strings.Contains(flags, want) {
			t.Errorf("payments create: no %s in %q", want, flags)
		}
	}
	if strings.Contains(flags, "--payee") || strings.Contains(flags, "--help") {
		t.Errorf("payments create: unexpected flag in %q", flags)
	}

	capture := describeCommand(paymentsCaptureCmd)
	for _, trait := range []string{"signs", "needs_yes", "can_wait"} {
		if capture[trait] != true {
			t.Errorf("payments capture: %s = %v", trait, capture[trait])
		}
	}
	if capture["args"] != "<rail0Id>" || capture["usage"] != "rail0 payments capture <rail0Id> [flags]" {
		t.Errorf("payments capture: args %q usage %q", capture["args"], capture["usage"])
	}
	byName := map[string]map[string]any{}
	for _, f := range capture["flags"].([]map[string]any) {
		byName[f["name"].(string)] = f
	}
	if f := byName["--amount"]; f["shorthand"] != "-a" || f["type"] != "string" || f["usage"] == "" {
		t.Errorf("--amount: %v", f)
	}
	if f := byName["--timeout"]; f["type"] != "duration" || f["default"] == nil {
		t.Errorf("--timeout: %v", f)
	}

	required := map[string]bool{}
	for _, f := range describeCommand(webhooksCreateCmd)["flags"].([]map[string]any) {
		if f["required"] == true {
			required[f["name"].(string)] = true
		}
	}
	if !required["--name"] || !required["--callback-url"] || !required["--topic"] {
		t.Errorf("webhooks create: required = %v", required)
	}
}

// What holds for every call is in the root index only; a group's index is its commands.
func TestCatalogueRootAndGroupIndex(t *testing.T) {
	root := commandIndex(rootCmd)
	for _, key := range []string{"cli", "global_flags", "environment", "agent_mode", "commands"} {
		if root[key] == nil {
			t.Errorf("root index has no %q", key)
		}
	}
	var envNames []string
	for _, e := range root["environment"].([]map[string]string) {
		envNames = append(envNames, e["name"])
	}
	for _, want := range []string{"RAIL0_BASE_URL", "RAIL0_PRIVATE_KEY", "RAIL0_AGENT", "RAIL0_HTTP_TIMEOUT"} {
		if !strings.Contains(strings.Join(envNames, " "), want) {
			t.Errorf("environment lacks %s: %v", want, envNames)
		}
	}
	if !sort.StringsAreSorted(envNames) {
		t.Errorf("environment is not sorted: %v", envNames)
	}

	group := commandIndex(paymentsCmd)
	if group["group"] != "rail0 payments" || group["agent_mode"] != nil {
		t.Errorf("group index: %v", group["group"])
	}
	for path := range indexPaths(t, paymentsCmd) {
		if !strings.HasPrefix(path, "rail0 payments ") {
			t.Errorf("%s is not a payments command", path)
		}
	}
}

// The commands the catalogue says agent mode refuses do refuse, each in its own RunE.
func TestAgentUnavailableIsTrue(t *testing.T) {
	withAgentMode(t)
	for path := range agentUnavailable {
		cmd, _, err := rootCmd.Find(strings.Fields(path)[1:])
		if err != nil || cmd.CommandPath() != path {
			t.Errorf("%s: not a command", path)
			continue
		}
		err = cmd.RunE(cmd, []string{"name"})
		code := agentErrorCode(err, exitCode(err))
		if err == nil || (code != "unavailable_in_agent_mode" && code != "interactive_only") {
			t.Errorf("%s: got %v (%s)", path, err, code)
		}
		if describeCommand(cmd)["agent_mode"] == nil {
			t.Errorf("%s: not marked in its description", path)
		}
	}
}

// The table of exit codes has a line for every code exitCode can return.
func TestExitCodeMeaningsAreComplete(t *testing.T) {
	seen := map[int]bool{}
	for _, e := range exitCodeMeanings {
		if seen[e.Code] || e.Meaning == "" {
			t.Errorf("code %d: duplicated or without a meaning", e.Code)
		}
		seen[e.Code] = true
	}
	for _, code := range []int{0, 1, ExitCodeUsage, 3, 4, 5, 6, 7, 8, 9, ExitCodeRateLimited, ExitCodeInFlight, ExitCodeAborted} {
		if !seen[code] {
			t.Errorf("exit code %d has no meaning listed", code)
		}
	}
}

// Every error code the CLI constructs is in the list the catalogue publishes, and the
// README names each one.
func TestCliErrorCodesAreComplete(t *testing.T) {
	listed := map[string]bool{}
	for _, code := range cliErrorCodes {
		listed[code] = true
	}
	constructed := regexp.MustCompile(`(?:newUsageError|newSessionError)\("([a-z_]+)"|Code: +"([a-z_]+)"|return "([a-z_]+)"\s*$`)
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		inErrorCode := false
		for _, line := range strings.Split(string(src), "\n") {
			// `return "code"` counts only inside agentErrorCode, the one function whose
			// string returns are error codes.
			if strings.HasPrefix(line, "func agentErrorCode(") {
				inErrorCode = true
			} else if strings.HasPrefix(line, "}") {
				inErrorCode = false
			}
			for _, m := range constructed.FindAllStringSubmatch(line, -1) {
				code := m[1] + m[2]
				if m[3] != "" && inErrorCode {
					code = m[3]
				}
				if code != "" && !listed[code] {
					t.Errorf("%s: error code %q is not in cliErrorCodes", file, code)
				}
			}
		}
	}
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range cliErrorCodes {
		if !strings.Contains(string(readme), "`"+code+"`") {
			t.Errorf("README does not name the error code %q", code)
		}
	}
}

// One name for both readers: help answers a person in text and a program in data. For
// the CLI and for a group the data is the index; for a command, the command in full.
func TestHelpDataByKindOfCommand(t *testing.T) {
	root := helpData(rootCmd)
	if root["command"] != "rail0" || root["agent_mode"] == nil || root["environment"] == nil {
		t.Errorf("root help: keys %v", keysOf(root))
	}
	if entries, _ := root["commands"].([]map[string]any); len(entries) < 50 {
		t.Errorf("root help lists %d commands", len(entries))
	}
	if !strings.Contains(root["help"].(string), "QUICKSTART") {
		t.Error("root help lacks the text a person reads")
	}

	group := helpData(paymentsCmd)
	if group["command"] != "rail0 payments" || group["agent_mode"] != nil || group["commands"] == nil {
		t.Errorf("group help: keys %v", keysOf(group))
	}

	leaf := helpData(paymentsCaptureCmd)
	if leaf["command"] != "rail0 payments capture" || leaf["flags"] == nil || leaf["commands"] != nil {
		t.Errorf("command help: keys %v", keysOf(leaf))
	}
	if leaf["needs_yes"] != true || !strings.Contains(leaf["help"].(string), "Usage:") {
		t.Errorf("command help: %v", keysOf(leaf))
	}

	// A group that also does something when run bare (wallets lists) is a command with
	// subcommands: its own flags, and the subcommands by name.
	wallets := helpData(walletsCmd)
	if wallets["flags"] == nil || wallets["subcommands"] == nil {
		t.Errorf("wallets help: keys %v", keysOf(wallets))
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --json asks help for the data without agent mode: plain JSON, no envelope. A piped
// stdout alone does not: 'rail0 help | less' is still a person.
func TestHelpWithJSONIsTheData(t *testing.T) {
	jsonOut = true
	t.Cleanup(func() { jsonOut = false })
	out := captureStdout(t, func() { paymentsCaptureCmd.HelpFunc()(paymentsCaptureCmd, nil) })
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("help --json is not JSON: %v\n%.200s", err, out)
	}
	if data["command"] != "rail0 payments capture" || data["ok"] != nil || data["flags"] == nil {
		t.Errorf("got keys %v", keysOf(data))
	}
}

func TestHelpWithoutAFlagIsText(t *testing.T) {
	var buf strings.Builder
	paymentsCaptureCmd.SetOut(&buf)
	t.Cleanup(func() { paymentsCaptureCmd.SetOut(nil) })
	paymentsCaptureCmd.HelpFunc()(paymentsCaptureCmd, nil)
	if out := buf.String(); strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, "Usage:") {
		t.Errorf("got %.120q", out)
	}
}

// Help in agent mode carries the same description as data, beside the text.
func TestAgentHelpCarriesTheFlagsAsData(t *testing.T) {
	withAgentMode(t)
	out := captureStdout(t, func() { paymentsCaptureCmd.HelpFunc()(paymentsCaptureCmd, nil) })
	for _, want := range []string{`"command":"rail0 payments capture"`, `"name":"--amount"`, `"needs_yes":true`, `"help":"`} {
		if !strings.Contains(out, want) {
			t.Errorf("help envelope lacks %s", want)
		}
	}
}
