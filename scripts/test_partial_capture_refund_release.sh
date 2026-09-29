#!/usr/bin/env bash
# test_partial_capture_refund_release.sh
#
# End-to-end CLI flow:
#   create → authorize → capture 0.30 → capture 0.20 →
#   refund 0.10 → refund 0.15 → (wait for expiry) → release
#
# Every on-chain step passes --wait, which blocks until THAT operation's own
# transaction confirms. The previous version instead polled for a payment STATUS,
# which made it wrong twice over: two partial captures both leave the payment in
# `partially_captured` (so the second wait passed instantly on the first one's
# state), and it waited for `partially_refunded`, which the gateway no longer
# produces — a partial refund leaves the status unchanged.
#
# Requires the gateway running with a short authorization TTL, so step 7 does not
# wait out the real one:
#   POLICY_AUTHORIZATION_TTL=30 bundle exec puma -C config/puma.rb
#
# Usage:
#   POLICY_AUTHORIZATION_TTL=30 ./scripts/test_partial_capture_refund_release.sh
#
# Overrides: RAIL0_API_URL CHAIN_ID TOKEN AMOUNT CAPTURE1 CAPTURE2 REFUND1 REFUND2
#            ACCOUNT_ID PAYEE BUYER_ADDRESS BUYER_PRIVATE_KEY ACCOUNT_PRIVATE_KEY
#            WAIT_TIMEOUT RAIL0

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

API_URL="${RAIL0_API_URL:-http://localhost:9292}"
# --chain-id is an integer EVM chain id, never a slug. Arc Testnet is the default
# because it needs 4 confirmations against Base Sepolia's 60, and this flow waits
# for six of them.
CHAIN_ID="${CHAIN_ID:-5042002}"
TOKEN="${TOKEN:-USDC}"
AMOUNT="${AMOUNT:-1.00}"
CAPTURE1="${CAPTURE1:-0.30}"
CAPTURE2="${CAPTURE2:-0.20}"
REFUND1="${REFUND1:-0.10}"
REFUND2="${REFUND2:-0.15}"
BUYER_ADDRESS="${BUYER_ADDRESS:-0xe3DD5ea618F74De4C95B0ff0EbF9921A06a694c4}"
BUYER_PRIVATE_KEY="${BUYER_PRIVATE_KEY:-0x58cfb681b2a04e9dc0083d8258da8c19ac7cc1750f94bb1ab7651f606df5a4d6}"
ACCOUNT_PRIVATE_KEY="${ACCOUNT_PRIVATE_KEY:-0x700e3be6cef3c28464d6c432eda04d87b5a0122424f4c0adeeea1e81c6137475}"
ACCOUNT_ID="${ACCOUNT_ID:-019e748b-da9a-7c3f-ba32-50572ffd5388}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-3m}"

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

step "Resolve payee for $TOKEN on chain $CHAIN_ID"
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

step "1. Create payment and sign (payer)"
RAIL0_ID=$(run payments create \
  -F "$BUYER_ADDRESS" -T "$PAYEE" -t "$TOKEN" -a "$AMOUNT" -c "$CHAIN_ID" \
  -p "$BUYER_PRIVATE_KEY" -q)
ok "rail0_id: $RAIL0_ID"

# ── 2. Authorize ──────────────────────────────────────────────────────────────

step "2. Authorize (payee)"
run payments authorize "$RAIL0_ID" -p "$ACCOUNT_PRIVATE_KEY" "${WAIT[@]}" > /dev/null
ok "Authorized"

# ── 3-4. Partial captures ─────────────────────────────────────────────────────

for n in 1 2; do
  amount_var="CAPTURE$n"
  step "$((2 + n)). Partial capture #$n (${!amount_var})"
  run payments capture "$RAIL0_ID" -a "${!amount_var}" -p "$ACCOUNT_PRIVATE_KEY" "${WAIT[@]}" > /dev/null
  ok "Captured ${!amount_var}"
done

# ── 5-6. Partial refunds ──────────────────────────────────────────────────────

# -y because a refund moves funds and is confirmation-gated; without it these
# lines block forever in CI.
for n in 1 2; do
  amount_var="REFUND$n"
  step "$((4 + n)). Partial refund #$n (${!amount_var}, EIP-3009)"
  run payments refund "$RAIL0_ID" -a "${!amount_var}" -p "$ACCOUNT_PRIVATE_KEY" -y "${WAIT[@]}" > /dev/null
  ok "Refunded ${!amount_var}"
done

# ── 7. Wait for authorization_expiry ─────────────────────────────────────────

step "7. Wait for authorization_expiry"
EXPIRY=$(run payments get "$RAIL0_ID" | python3 -c '
import json, sys
print(json.load(sys.stdin)["authorization_expiry"])
')
WAIT_SECS=$(( EXPIRY - $(date +%s) + 3 ))
if (( WAIT_SECS > 0 )); then
  echo "  waiting ${WAIT_SECS}s for the authorization to expire…"
  sleep "$WAIT_SECS"
else
  echo "  expiry already passed — proceeding"
fi
ok "authorization_expiry passed"

# ── 8. Release (payer) ────────────────────────────────────────────────────────

# Payer or payee only; the caller is whoever the key belongs to, derived from -p —
# there is no --caller-address flag to pass (and never was one on this command).
step "8. Release (payer)"
run payments release "$RAIL0_ID" -p "$BUYER_PRIVATE_KEY" -y "${WAIT[@]}" > /dev/null
ok "Released"

# ── Summary ───────────────────────────────────────────────────────────────────

step "Final state"
run payments history "$RAIL0_ID" --pretty
echo
echo "✓ partial-capture × 2 → partial-refund × 2 → release complete"
