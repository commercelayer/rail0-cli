## rail0

CLI for the RAIL0 stablecoin payment protocol

### Synopsis

rail0 is a command-line interface for the RAIL0 stablecoin payment gateway.

Defaults to https://api.rail0.xyz. Override with --base-url or RAIL0_BASE_URL.

Use 'rail0 <command> --help' for detailed usage of any command.

### Options

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
  -h, --help              help for rail0
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 aliases](rail0_aliases.md)	 - Address book — list saved addresses, or manage them with add/rm
* [rail0 analytics](rail0_analytics.md)	 - Account sales analytics: summary, timeseries, breakdown (requires login)
* [rail0 auth](rail0_auth.md)	 - Authenticate with the RAIL0 gateway using SIWE
* [rail0 chains](rail0_chains.md)	 - List supported blockchains (chain IDs used in --chain-id)
* [rail0 config](rail0_config.md)	 - Manage persisted CLI defaults (base-url, chain-id, token, output, http-timeout)
* [rail0 disputes](rail0_disputes.md)	 - List every dispute on your payments (requires login)
* [rail0 env](rail0_env.md)	 - Switch the active environment (development/staging/production) in one shot
* [rail0 health](rail0_health.md)	 - Report gateway health (liveness/readiness)
* [rail0 keys](rail0_keys.md)	 - Manage signing keys in the OS keychain (lists names when run bare)
* [rail0 payment-methods](rail0_payment-methods.md)	 - Discover a merchant's accepted wallets/tokens (public, no login)
* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations
* [rail0 token](rail0_token.md)	 - Print the stored session token, or suggest logging in
* [rail0 tokens](rail0_tokens.md)	 - List supported tokens (symbols used in --token)
* [rail0 version](rail0_version.md)	 - Print the rail0 CLI version
* [rail0 wallets](rail0_wallets.md)	 - Manage the logged-in account's wallets (lists when run bare)
* [rail0 webhooks](rail0_webhooks.md)	 - Webhook subscription management (requires login)

