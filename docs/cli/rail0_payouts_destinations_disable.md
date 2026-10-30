## rail0 payouts destinations disable

Switch a destination off (reversible)

### Synopsis

Switch a payout destination off: nothing can be prepared to it until it is
enabled again. Any wallet of the account may disable it.

Its pending payout is settled first: an unsigned one fails "cancelled"; a SIGNED
one refuses the switch-off (409 payout_already_signed), because its transaction
may still be mined — resend it or let it settle, then disable.

```
rail0 payouts destinations disable <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations disable <destination_id>
```

### Options

```
  -h, --help   help for disable
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

* [rail0 payouts destinations](rail0_payouts_destinations.md)	 - Manage where payouts go (lists when run bare)

