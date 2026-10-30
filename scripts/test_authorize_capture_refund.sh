#!/usr/bin/env bash
# test_authorize_capture_refund.sh
#
# End-to-end CLI flow: create → authorize → capture → refund (EIP-3009, no approve).
#
# There is no polling loop here on purpose. Every on-chain step passes --wait, so
# the CLI itself blocks until the indexer confirms the operation — or reports the
# revert and exits non-zero. The previous version of this script reimplemented that
# in ~30 lines of python3 over `payments get`, which both duplicated the CLI's job
# and drifted from it (it waited for `partially_refunded`, a status the gateway no
# longer produces).
#
# The action commands sign in by themselves with the key they are given (SIWE), so
# they need no prior `auth login`. The login below exists only to fix WHICH session
# backs the read-only summary at the end.
#
# Usage:
#   ./scripts/test_authorize_capture_refund.sh
#
# Overrides: RAIL0_API_URL CHAIN (or CHAIN_ID) TOKEN AMOUNT ACCOUNT_ID PAYEE BUYER_ADDRESS
#            BUYER_PRIVATE_KEY ACCOUNT_PRIVATE_KEY WAIT_TIMEOUT RAIL0

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

API_URL="${RAIL0_API_URL:-http://localhost:9292}"
# --chain takes a slug (arc, base, …) or a chain id, as listed by `rail0 chains`;
# CHAIN_ID (the numeric override this script used to take) is still honoured. Arc
# Testnet is the default
# because it needs 4 confirmations against Base Sepolia's 60 — the difference
# between a script that finishes in a minute and one that looks hung.
CHAIN="${CHAIN:-${CHAIN_ID:-arc}}"
TOKEN="${TOKEN:-USDC}"
AMOUNT="${AMOUNT:-1.00}"
BUYER_ADDRESS="${BUYER_ADDRESS:-0xe3DD5ea618F74De4C95B0ff0EbF9921A06a694c4}"
BUYER_PRIVATE_KEY="${BUYER_PRIVATE_KEY:-0x58cfb681b2a04e9dc0083d8258da8c19ac7cc1750f94bb1ab7651f606df5a4d6}"
ACCOUNT_PRIVATE_KEY="${ACCOUNT_PRIVATE_KEY:-0x700e3be6cef3c28464d6c432eda04d87b5a0122424f4c0adeeea1e81c6137475}"
ACCOUNT_ID="${ACCOUNT_ID:-019e748b-da9a-7c3f-ba32-50572ffd5388}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-3m}"

# Build unless a binary was supplied. Assigning the build's OUTPUT to RAIL0 (what
# this used to do) turns a compile error into an unrunnable command name.
RAIL0="${RAIL0:-}"
if [[ -z "$RAIL0" ]]; then
  RAIL0=/tmp/rail0-bin
  go build -o "$RAIL0" .
fi

BASE=(--base-url "$API_URL")
WAIT=(--wait --timeout "$WAIT_TIMEOUT")

# ── Helpers ───────────────────────────────────────────────────────────────────

step() { echo; echo "── $* ──"; }
ok()   { echo "  ✓ $*"; }

# Echoes the command with the signing key redacted. These are throwaway testnet
# keys, but a script that prints -p values teaches the habit of doing it with real
# ones, into terminal scrollback and CI logs.
run() {
  local shown=() redact=0 a
  for a in "$@"; do
    if (( redact )); then
      shown+=("***"); redact=0
    else
      shown+=("$a")
      [[ "$a" == "-p" || "$a" == "--private-key" || "$a" == "--pk" ]] && redact=1
    fi
  done
  echo "  $ rail0 ${shown[*]}" >&2
  "$RAIL0" "$@" "${BASE[@]}"
}

# ── Login (payee) ─────────────────────────────────────────────────────────────

step "Login as payee"
run auth login -p "$ACCOUNT_PRIVATE_KEY" > /dev/null
ok "Logged in"

# ── Resolve payee ─────────────────────────────────────────────────────────────

# The payee lookup matches the numeric chain_id payment-methods carries, so the slug
# is resolved to it from the gateway's own catalog.
step "Resolve chain $CHAIN"
CHAIN_ID=$("$RAIL0" chains "${BASE[@]}" | CHAIN="$CHAIN" python3 -c '
import json, os, sys
want = os.environ["CHAIN"]
for c in json.load(sys.stdin):
    if c.get("slug") == want or str(c.get("chain_id")) == want:
        print(c["chain_id"])
        raise SystemExit
raise SystemExit(f"unknown chain {want!r}: see rail0 chains")
')
ok "Chain: $CHAIN ($CHAIN_ID)"

step "Resolve payee for $TOKEN on chain $CHAIN"
PAYEE="${PAYEE:-$(CHAIN_ID="$CHAIN_ID" TOKEN="$TOKEN" ACCOUNT_ID="$ACCOUNT_ID" \
  "$RAIL0" payment-methods --account "$ACCOUNT_ID" "${BASE[@]}" | python3 -c '
import json, os, sys
chain, sym = int(os.environ["CHAIN_ID"]), os.environ["TOKEN"]
for w in json.load(sys.stdin):
    for holding in w.get("tokens") or []:
        t = holding.get("token") or {}
        if t.get("symbol") == sym and t.get("chain_id") == chain:
            print(w["address"])
            raise SystemExit
raise SystemExit(f"no {sym} wallet on chain {chain} for account {os.environ['ACCOUNT_ID']}")
')}"
ok "Payee: $PAYEE"

# ── 1. Create + payer signs ───────────────────────────────────────────────────

# -q prints just the rail0_id, so no JSON parsing is needed to chain the steps.
step "1. Create payment and sign (payer)"
RAIL0_ID=$(run payments create \
  -F "$BUYER_ADDRESS" -T "$PAYEE" -t "$TOKEN" -a "$AMOUNT" -c "$CHAIN" \
  -p "$BUYER_PRIVATE_KEY" -q)
ok "rail0_id: $RAIL0_ID"

# ── 2. Authorize ──────────────────────────────────────────────────────────────

step "2. Authorize (payee)"
run payments authorize "$RAIL0_ID" -p "$ACCOUNT_PRIVATE_KEY" "${WAIT[@]}" > /dev/null
ok "Authorized"

# ── 3. Capture ────────────────────────────────────────────────────────────────

step "3. Capture $AMOUNT"
run payments capture "$RAIL0_ID" -a "$AMOUNT" -p "$ACCOUNT_PRIVATE_KEY" "${WAIT[@]}" > /dev/null
ok "Captured"

# ── 4. Refund (EIP-3009, no approve needed) ───────────────────────────────────

# -y because a refund moves funds and the CLI asks for confirmation otherwise —
# without it this line blocks forever in CI.
step "4. Refund $AMOUNT (EIP-3009 receiveWithAuthorization)"
run payments refund "$RAIL0_ID" -a "$AMOUNT" -p "$ACCOUNT_PRIVATE_KEY" -y "${WAIT[@]}" > /dev/null
ok "Refunded"

# ── Summary ───────────────────────────────────────────────────────────────────

step "Final state"
run payments history "$RAIL0_ID" --pretty
echo
echo "✓ authorize → capture → refund complete"
