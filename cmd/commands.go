package cmd

import (
	"sort"
	"strings"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The catalogue: the CLI describing itself as data, for a program — an AI agent first of
// all — that has to work out WHICH command and WHICH flags without reading help prose.
// The help answers "what does this do" for a person; this answers "what can I call, with
// what" for a caller that would otherwise guess (--payee where the flag is --to) and
// learn from the usage error.
//
// Everything here is read from the command tree cobra already holds, so it cannot drift
// from the commands: the same source the help and the generated reference use. The three
// things the tree does not say are kept beside it and checked by tests: which commands
// agent mode refuses (agentUnavailable), the exit codes (exitCodeMeanings) and the CLI's
// own error codes (cliErrorCodes).

// agentUnavailable are the commands agent mode refuses, by path. Each refuses in its own
// RunE (refuseInAgentMode, or init's interactive_only); a test runs them to keep this
// list true.
var agentUnavailable = map[string]string{
	"rail0 init":          "interactive: it asks for the gateway, the key and the sign-in",
	"rail0 keys generate": "it would print a private key into the agent's context",
	"rail0 keys add":      "it reads a private key from the terminal or a pipe",
}

// exitCodeMeanings is exitCode's table, in words. A test checks the two agree.
var exitCodeMeanings = []struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}{
	{0, "success"},
	{1, "a failure the caller cannot fix by changing the command"},
	{ExitCodeUsage, "usage: fix the call (a wrong, missing or conflicting flag or argument, a missing key or --yes)"},
	{3, "unauthorized: the gateway answered 401, or no usable session is stored — sign in"},
	{4, "forbidden: the gateway answered 403, or the session's wallet has no account"},
	{5, "not found (404)"},
	{6, "conflict (409)"},
	{7, "rejected by the gateway (400, 422): the request is well formed and not acceptable now"},
	{8, "gateway error (5xx)"},
	{9, "network: no response from the gateway"},
	{ExitCodeRateLimited, "rate limited (429): the same command later is the right move"},
	{ExitCodeInFlight, "in flight: a --wait ran out of time; check the payment, never repeat the operation"},
	{ExitCodeAborted, "cancelled by the user"},
}

// cliErrorCodes are the error.code values the CLI itself produces; any other code in an
// error envelope is the gateway's own. A test checks every code the source constructs
// is here, and that the README lists each.
var cliErrorCodes = []string{
	"usage_error", "missing_private_key", "invalid_chain", "flag_conflict",
	"confirmation_required", "minimum_required", "minimum_conflict", "label_required",
	"unavailable_in_agent_mode", "interactive_only", "not_logged_in", "session_expired",
	"account_required", "aborted", "wait_timeout", "network_error", "cli_error",
}

// extraEnvironment are the variables read outside the config layer (configKeys carries
// the rest, each with its env name).
var extraEnvironment = []map[string]string{
	{"name": "RAIL0_PRIVATE_KEY", "purpose": "The signing key, when --private-key/-p is not passed (0x-prefixed hex)"},
	{"name": "RAIL0_AGENT", "purpose": "1 turns agent mode on for every call, like --agent"},
	{"name": "RAIL0_NO_UPDATE_CHECK", "purpose": "1 switches off the once-a-day check for a newer release"},
}

var commandsCmd = &cobra.Command{
	Use:   "commands [command…]",
	Short: "Describe the CLI's commands and flags as data (for programs and agents)",
	Long: `Prints what this CLI can be called with, as data rather than as help text.

Bare, it prints an index: every command with its one-line purpose, its arguments and
the names of its flags, plus what holds for all of them — the global flags, the
environment variables, the exit codes and the CLI's own error codes.

With a command, it prints that command in full: each flag with its type, default,
whether it is required and what it means, the examples, and whether the command signs
(--private-key), asks for consent (--yes), can wait for the chain (--wait) or is refused
in agent mode. With a group (payments, webhooks, …) it prints the index of that group.

It reads the command tree itself, the same source as --help, so it cannot describe a
flag that does not exist. It is local: no gateway call, no session.`,
	Example: `  rail0 commands                      # the index, with the exit and error codes
  rail0 commands payments              # the payment commands
  rail0 commands payments capture      # one command, every flag
  rail0 --agent commands payments create | jq '.data.command.flags[].name'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		target := cmd.Root()
		if len(args) > 0 {
			found, rest, err := cmd.Root().Find(args)
			if err != nil || len(rest) > 0 || found == nil || !describable(found) {
				return usagef("unknown command %q\n\nRun '%s commands' for the list", strings.Join(args, " "), cmd.Root().CommandPath())
			}
			target = found
		}
		return ui.RunAPI(func() (any, error) {
			if target.HasAvailableSubCommands() {
				return commandIndex(target), nil
			}
			return map[string]any{"command": describeCommand(target)}, nil
		})
	},
}

// describable reports whether a command belongs in the catalogue: one a caller can run
// and the help lists. Hidden commands, cobra's completion plumbing and the help command
// itself are left out.
func describable(c *cobra.Command) bool {
	if c.Hidden || c.Deprecated != "" {
		return false
	}
	switch c.Name() {
	case "help", "completion":
		return c.Parent() != nil && c.Parent() != c.Root()
	}
	return true
}

// commandIndex is the short form for a group (or the whole CLI): one line of data per
// runnable command beneath it. The root's index also carries what holds for every call.
func commandIndex(group *cobra.Command) map[string]any {
	var entries []map[string]any
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if !describable(sub) {
				continue
			}
			if sub.Runnable() && !isBareGroup(sub) {
				entries = append(entries, summarizeCommand(sub))
			}
			walk(sub)
		}
	}
	walk(group)
	out := map[string]any{"commands": entries}
	if group != group.Root() {
		out["group"] = group.CommandPath()
		return out
	}
	out["cli"] = map[string]any{"name": group.Name(), "version": version}
	out["global_flags"] = describeFlags(group.PersistentFlags(), nil)
	out["environment"] = describeEnvironment()
	out["agent_mode"] = map[string]any{
		"flag":            "--agent",
		"env":             "RAIL0_AGENT=1",
		"stdout":          `one JSON line per command: {"ok":true,"data":…,"page"?:…} or {"ok":false,"error":{…},"next_steps"?:[…]}`,
		"stderr":          "JSON Lines events",
		"exit_codes":      exitCodeMeanings,
		"cli_error_codes": cliErrorCodes,
		"detail":          "rail0 commands <command> prints one command in full",
	}
	return out
}

// isBareGroup reports a group whose only action is to print its help (see
// rejectUnknownSubcommands, which gives every pure group a RunE): not something to call.
func isBareGroup(c *cobra.Command) bool {
	return c.HasAvailableSubCommands() && len(nonHelpFlags(c.LocalFlags())) == 0 && c.Args == nil
}

func nonHelpFlags(set *pflag.FlagSet) []string {
	var names []string
	set.VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" && !f.Hidden {
			names = append(names, "--"+f.Name)
		}
	})
	return names
}

// summarizeCommand is a command's line in the index: enough to choose it and to know
// its flags by name. The types and meanings are one call away (rail0 commands <path>).
func summarizeCommand(c *cobra.Command) map[string]any {
	entry := map[string]any{
		"path":  c.CommandPath(),
		"short": c.Short,
		"flags": nonHelpFlags(c.LocalFlags()),
	}
	if args := argsOf(c); args != "" {
		entry["args"] = args
	}
	markTraits(entry, c)
	return entry
}

// describeCommand is a command in full.
func describeCommand(c *cobra.Command) map[string]any {
	entry := map[string]any{
		"path":  c.CommandPath(),
		"usage": c.UseLine(),
		"short": c.Short,
		"flags": describeFlags(c.LocalFlags(), c),
	}
	if long := strings.TrimSpace(c.Long); long != "" {
		entry["description"] = long
	}
	if args := argsOf(c); args != "" {
		entry["args"] = args
	}
	if len(c.Aliases) > 0 {
		entry["aliases"] = c.Aliases
	}
	if ex := strings.TrimSpace(c.Example); ex != "" {
		entry["examples"] = ex
	}
	var subs []map[string]string
	for _, sub := range c.Commands() {
		if describable(sub) {
			subs = append(subs, map[string]string{"name": sub.Name(), "short": sub.Short})
		}
	}
	if len(subs) > 0 {
		entry["subcommands"] = subs
	}
	markTraits(entry, c)
	return entry
}

// markTraits adds what a caller must know before running a command and cannot read from
// a flag list at a glance: that it signs, that it needs consent, that it can wait for the
// chain, that agent mode refuses it. The first three are read from the command's own
// flags, so they are true by construction.
func markTraits(entry map[string]any, c *cobra.Command) {
	flags := c.LocalFlags()
	if flags.Lookup("private-key") != nil {
		entry["signs"] = true
	}
	if flags.Lookup("yes") != nil {
		entry["needs_yes"] = true
	}
	if flags.Lookup("wait") != nil {
		entry["can_wait"] = true
	}
	if why, refused := agentUnavailable[c.CommandPath()]; refused {
		entry["agent_mode"] = "unavailable: " + why
	}
}

// argsOf is the positional part of a command's usage line: "capture <rail0Id> [flags]"
// gives "<rail0Id>".
func argsOf(c *cobra.Command) string {
	rest := strings.TrimSpace(strings.TrimPrefix(c.Use, c.Name()))
	return strings.TrimSpace(strings.TrimSuffix(rest, "[flags]"))
}

// describeFlags lists a flag set in full, by name. owner, when given, is asked which
// flags it requires (cobra records that as an annotation on the flag).
func describeFlags(set *pflag.FlagSet, owner *cobra.Command) []map[string]any {
	var out []map[string]any
	set.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		entry := map[string]any{"name": "--" + f.Name, "type": f.Value.Type(), "usage": f.Usage}
		if f.Shorthand != "" {
			entry["shorthand"] = "-" + f.Shorthand
		}
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "[]" {
			entry["default"] = f.DefValue
		}
		if owner != nil {
			if _, required := f.Annotations[cobra.BashCompOneRequiredFlag]; required {
				entry["required"] = true
			}
		}
		if f.Deprecated != "" {
			entry["deprecated"] = f.Deprecated
		}
		out = append(out, entry)
	})
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	return out
}

// describeEnvironment is every variable the CLI reads: the settable defaults, each with
// the `config` key it mirrors, and the few read outside the config layer.
func describeEnvironment() []map[string]string {
	var out []map[string]string
	for _, key := range configKeys {
		if key.env == "" {
			continue
		}
		out = append(out, map[string]string{"name": key.env, "purpose": key.desc, "config_key": key.name})
	}
	out = append(out, extraEnvironment...)
	sort.Slice(out, func(i, j int) bool { return out[i]["name"] < out[j]["name"] })
	return out
}

func init() {
	rootCmd.AddCommand(commandsCmd)
}
