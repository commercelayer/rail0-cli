## rail0 admin health

Operator diagnostics: one verdict, then a check per thing that can be wrong

### Synopsis

The gateway's operator diagnostics: a verdict ("ok", "degraded" or "error") and a named
check per standing failure condition — the worker fleet, the indexer's intake, stuck
transactions, open webhook circuits, balance divergences — each with its figures and a
sentence saying what they mean.

--rpc asks the other question: can the gateway reach the chains. Every active chain's RPC
endpoints are probed from the gateway's own process, with each one's status, head block
and latency.

The exit code follows the verdict: 0 when it is "ok", 1 otherwise, after the report is
printed. So 'rail0 admin health -q || alert' works as a check.

```
rail0 admin health [flags]
```

### Examples

```
  rail0 admin health
  rail0 admin health --rpc
  rail0 admin health --pretty
```

### Options

```
  -h, --help   help for health
      --rpc    Probe every active chain's RPC endpoints from the gateway instead of reading the diagnostics
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, operators, audit, write mode

