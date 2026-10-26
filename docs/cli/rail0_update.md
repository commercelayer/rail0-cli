## rail0 update

Update rail0 to the latest release

### Synopsis

Download the latest rail0 release and install it in place of this binary.

The archive for this platform is downloaded from the GitHub release (with a progress
bar on a terminal), its SHA-256 is verified against the release's checksums.txt, and
the binary inside replaces the running one. Nothing on disk changes unless the
download verifies. The r0 alias follows, since it is a link to the same file.

If rail0 lives in a directory you cannot write to (e.g. /usr/local/bin, where
'make install' puts it), run it with sudo.

Interactive sessions also check for a new release once a day and offer to install
it. The check never runs in CI, with --json or --quiet (or a stored 'output' default
of json or quiet, unless another output flag is passed), when stdin or stderr is not
a terminal, or with RAIL0_NO_UPDATE_CHECK=1.

```
rail0 update [flags]
```

### Examples

```
  rail0 update           # install the latest release
  rail0 update --check   # only report whether one is available
```

### Options

```
      --check   Only report whether a newer release is available
  -h, --help    help for update
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

