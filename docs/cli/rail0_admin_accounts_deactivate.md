## rail0 admin accounts deactivate

Switch an account off (write mode, audited)

### Synopsis

Switch an account off. From then on it can start no new payment (authorize, charge)
and write no payout; its sessions keep reading, and what is already in motion stays
where it is. Reversible: 'rail0 admin accounts activate'.

The command reads the account and its exposure first ('rail0 admin accounts exposure')
and asks, naming the account and what it still has in motion. Answering yes to that
question is the acknowledgement the gateway requires for an account with open exposure.
Without a question (--yes, or no terminal) an account with open exposure is refused
unless --acknowledge-exposure is passed too: --yes alone never switches off an account
that has something in motion.

The gateway also refuses your own account (another operator has to) and the account
holding the last active operator grant.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin accounts deactivate <account> [flags]
```

### Examples

```
  rail0 admin accounts deactivate 019e748b-…
  rail0 admin accounts deactivate 019e748b-… --yes                          # only with nothing in motion
  rail0 admin accounts deactivate 019e748b-… --yes --acknowledge-exposure
```

### Options

```
      --acknowledge-exposure   Switch the account off although it still has payments, escrow, disputes or payouts in motion (needed with --yes)
  -h, --help                   help for deactivate
  -y, --yes                    Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 admin accounts](rail0_admin_accounts.md)	 - Every account: find one, read it, switch it off or on (lists when run bare)

