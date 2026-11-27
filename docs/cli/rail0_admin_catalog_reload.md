## rail0 admin catalog reload

Rebuild the catalogue snapshot from the database, in the process that answers (write mode, audited)

### Synopsis

Rebuild the chain catalogue's in-memory snapshot from the database. What the database
holds becomes what the process serves: a chain switched on starts taking payments, a
token retired stops being offered.

The command reads the diff first and asks, saying how many changes are pending. The
answer says what the rebuilt snapshot holds and what the reload applied.

ONE PROCESS, the one that answers the request. It is the tool for an environment served
by a single process. Where several serve (web machines, a worker) each has its own
snapshot and this reaches one of them: a restart is what reaches them all.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin catalog reload [flags]
```

### Examples

```
  rail0 admin catalog reload
  rail0 admin catalog reload --yes
```

### Options

```
  -h, --help   help for reload
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 admin catalog](rail0_admin_catalog.md)	 - The chain catalogue: what a reload would change, the reload, chains, tokens, contracts (shows the diff when run bare)

