## rail0 open

Open the admin app in the browser, on a payment's page when given its id

### Synopsis

Open the rail0 admin app in the browser: its home page, or — given a payment's
rail0_id (0x…) or UUID — that payment's page.

--console opens the operator console's page for the payment instead of the merchant's
(it needs an admin session in the browser). --print only prints the URL, as it does
when there is no terminal or with --agent, so a script can pass it on.

The admin's address is the admin-url setting (RAIL0_ADMIN_URL), or else the one the
current environment declares: development and staging have one, production does not
— there, set it with 'rail0 config set admin-url https://…'.

```
rail0 open [rail0Id|uuid] [flags]
```

### Examples

```
  rail0 open                          # the admin's home page
  rail0 open 0xabc123…                # that payment's page
  rail0 open 0xabc123… --console      # the console's page for it (operators)
  rail0 open 0xabc123… --print        # just the URL
```

### Options

```
      --console   Open the operator console's page for the payment (admin session)
  -h, --help      help for open
      --print     Only print the URL; do not open a browser
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

