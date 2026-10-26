## rail0 auth logout

End the session: revoke the token on the gateway and remove it locally

### Synopsis

End the current session.

The token is revoked on the gateway — its jti goes into the denylist, so the JWT
stops being accepted BEFORE it expires — and then removed from this machine.

Removing the local copy alone is not a logout: the JWT stays valid for the rest
of its lifetime, so anyone holding another copy keeps the session. The local file
is removed even when the revocation cannot be performed (an unreachable gateway
is a normal reason to log out), and the outcome says which of the two happened.

```
rail0 auth logout [flags]
```

### Options

```
  -h, --help   help for logout
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

* [rail0 auth](rail0_auth.md)	 - Authenticate with the RAIL0 gateway using SIWE

