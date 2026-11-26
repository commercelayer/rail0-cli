## rail0 admin config

The configuration the gateway is running with: every window, limit and feature flag

### Synopsis

Print the effective configuration of the gateway process that answers: every window,
threshold, limit and feature flag, one per row.

  group        build, payments, recovery, retention, webhooks, payouts, settlement,
               rate_limit, siwe, session, fees, rpc, onboarding, email
  setting      its name within the group
  value        what the gateway is using now
  env          the environment variable that sets it (empty for a constant of the code)
  overridden   whether that variable is set in this process: a default and an override
               that happen to coincide read differently
  description  what it controls and its unit, in the gateway's own words

Pass a group to print only that one. --overridden keeps only the settings whose variable
is set: what this environment changed from the defaults.

Read-only by nature: the values come from the environment when the process starts, and a
deploy is the only way to change one. No secret is among them.

```
rail0 admin config [group] [flags]
```

### Examples

```
  rail0 admin config --pretty
  rail0 admin config payouts               # is the payouts feature on here
  rail0 admin config rate_limit --pretty
  rail0 admin config --overridden          # what this environment changed from the defaults
```

### Options

```
  -h, --help         help for config
      --overridden   Only the settings whose environment variable is set in this process
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode

