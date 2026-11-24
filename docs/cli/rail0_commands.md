## rail0 commands

Describe the CLI's commands and flags as data (for programs and agents)

### Synopsis

Prints what this CLI can be called with, as data rather than as help text.

Bare, it prints an index: every command with its one-line purpose, its arguments and
the names of its flags, plus what holds for all of them — the global flags, the
environment variables, the exit codes and the CLI's own error codes.

With a command, it prints that command in full: each flag with its type, default,
whether it is required and what it means, the examples, and whether the command signs
(--private-key), asks for consent (--yes), can wait for the chain (--wait) or is refused
in agent mode. With a group (payments, webhooks, …) it prints the index of that group.

It reads the command tree itself, the same source as --help, so it cannot describe a
flag that does not exist. It is local: no gateway call, no session.

```
rail0 commands [command…] [flags]
```

### Examples

```
  rail0 commands                      # the index, with the exit and error codes
  rail0 commands payments              # the payment commands
  rail0 commands payments capture      # one command, every flag
  rail0 --agent commands payments create | jq '.data.command.flags[].name'
```

### Options

```
  -h, --help   help for commands
```

### Options inherited from parent commands

```
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol

