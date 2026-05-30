#!/usr/bin/env bash
# test_partial_capture_refund_release.sh
#
# End-to-end CLI flow:
#   create → authorize → capture 300k → capture 200k →
#   refund 100k → refund 150k → (wait expiry) → release
#
# Requires the API running with a short authorization TTL:
#   POLICY_AUTHORIZATION_TTL=30 bundle exec puma -C config/puma.rb
#
# Usage:
#   POLICY_AUTHORIZATION_TTL=30 ./scripts/test_partial_capture_refund_release.sh

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

API_URL="${RAIL0_API_URL:-http://0.0.0.0:4567}"
CHAIN="arc-testnet"
TOKEN="USDC"
AMOUNT=1000000        # 1 USDC (6 decimals)
CAPTURE1=300000
CAPTURE2=200000
REFUND1=100000
REFUND2=150000
AUTHORIZATION_TTL="${POLICY_AUTHORIZATION_TTL:-604800}"
BUYER_ADDRESS="${BUYER_ADDRESS:-0xe3DD5ea618F74De4C95B0ff0EbF9921A06a694c4}"
BUYER_PRIVATE_KEY="${BUYER_PRIVATE_KEY:-0x58cfb681b2a04e9dc0083d8258da8c19ac7cc1750f94bb1ab7651f606df5a4d6}"
ACCOUNT_PRIVATE_KEY="${ACCOUNT_PRIVATE_KEY:-0x700e3be6cef3c28464d6c432eda04d87b5a0122424f4c0adeeea1e81c6137475}"
ACCOUNT_ID="${ACCOUNT_ID:-019e748b-da9a-7c3f-ba32-50572ffd5388}"
POLL_INTERVAL="${POLL_INTERVAL:-3}"
POLL_TIMEOUT="${POLL_TIMEOUT:-120}"

: "${RAIL0:=$(go build -o /tmp/rail0-bin . 2>&1 && echo /tmp/rail0-bin)}"
RAIL0="${RAIL0:-/tmp/rail0-bin}"
BASE="--base-url $API_URL"

# ── Helpers ───────────────────────────────────────────────────────────────────

step() { echo; echo "── $* ──"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ $*" >&2; exit 1; }
run()  { echo "  $ $RAIL0 $*" >&2; $RAIL0 "$@"; }

poll_until() {
  local id="$1"; shift; local want=("$@")
  local elapsed=0
  while true; do
    local resp; resp=$($RAIL0 payments get "$id" $BASE)
    local status; status=$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin)['status'])")
    local txs; txs=$(echo "$resp" | python3 -c "
import json,sys
ts=json.load(sys.stdin).get('transactions',[])
print(', '.join(f\"{t['operation']}:{t['status']}\" for t in ts) or '-')
")
    echo "  polling… payment=$status  transactions=[$txs]"
    for s in "${want[@]}"; do [[ "$status" == "$s" ]] && return 0; done
    [[ "$status" == "failed" ]] && fail "payment $id failed"
    (( elapsed += POLL_INTERVAL ))
    (( elapsed >= POLL_TIMEOUT )) && fail "timeout after ${POLL_TIMEOUT}s (last: $status)"
    sleep "$POLL_INTERVAL"
  done
}

# ── Resolve payee ─────────────────────────────────────────────────────────────

step "Resolve payee for $TOKEN on $CHAIN"
PAYEE=$($RAIL0 accounts payment-methods "$ACCOUNT_ID" $BASE | python3 -c "
import json,sys
ms=json.load(sys.stdin)
m=next((m for m in ms if m['chain_slug']=='$CHAIN' and m['token_symbol']=='$TOKEN'),None)
if not m: raise SystemExit('no $TOKEN wallet on $CHAIN for account $ACCOUNT_ID')
print(m['wallet_address'])
")
ok "Payee: $PAYEE"

# ── 1. Create + payer signs ───────────────────────────────────────────────────

step "1. Create payment and sign (payer)"
echo "  $ $RAIL0 payments create -F $BUYER_ADDRESS -T $PAYEE -t $TOKEN -a $AMOUNT -c $CHAIN -p <key> $BASE" >&2
RAIL0_ID=$($RAIL0 payments create \
  -F "$BUYER_ADDRESS" \
  -T "$PAYEE" \
  -t "$TOKEN" \
  -a "$AMOUNT" \
  -c "$CHAIN" \
  -p "$BUYER_PRIVATE_KEY" \
  $BASE | python3 -c "import json,sys; print(json.load(sys.stdin)['rail0_id'])")
ok "rail0_id: $RAIL0_ID"

# ── 2. Authorize ──────────────────────────────────────────────────────────────

step "2. Authorize (payee)"
run payments authorize "$RAIL0_ID" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling for indexer confirmation…"
poll_until "$RAIL0_ID" authorized
ok "Authorized"

# ── 3. Partial capture #1 (300k) ──────────────────────────────────────────────

step "3. Partial capture #1 ($CAPTURE1)"
run payments capture "$RAIL0_ID" -a "$CAPTURE1" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling…"
poll_until "$RAIL0_ID" partially_captured
ok "Partially captured"

# ── 4. Partial capture #2 (200k) ──────────────────────────────────────────────

step "4. Partial capture #2 ($CAPTURE2)"
run payments capture "$RAIL0_ID" -a "$CAPTURE2" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling…"
poll_until "$RAIL0_ID" partially_captured
ok "Partially captured"

# ── 5. Partial refund #1 (100k) ───────────────────────────────────────────────

step "5. Partial refund #1 ($REFUND1, EIP-3009)"
run payments refund "$RAIL0_ID" -a "$REFUND1" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling…"
poll_until "$RAIL0_ID" partially_refunded
ok "Partially refunded"

# ── 6. Partial refund #2 (150k) ───────────────────────────────────────────────

step "6. Partial refund #2 ($REFUND2, EIP-3009)"
run payments refund "$RAIL0_ID" -a "$REFUND2" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling…"
poll_until "$RAIL0_ID" partially_refunded
ok "Partially refunded"

# ── 7. Wait for authorization_expiry ─────────────────────────────────────────

step "7. Wait for authorization_expiry (TTL=${AUTHORIZATION_TTL}s)"
EXPIRY=$($RAIL0 payments get "$RAIL0_ID" $BASE | python3 -c "
import json,sys; print(json.load(sys.stdin)['authorization_expiry'])
")
NOW=$(date +%s)
WAIT=$(( EXPIRY - NOW + 3 ))
if (( WAIT > 0 )); then
  echo "  waiting ${WAIT}s (expiry=$EXPIRY, now=$NOW)…"
  for (( i=WAIT; i>0; i-- )); do
    printf "\r  ⏳ %ds remaining…   " "$i"
    sleep 1
  done
  echo
else
  echo "  expiry already passed — proceeding"
fi
ok "authorization_expiry passed"

# ── 8. Release (buyer) ────────────────────────────────────────────────────────

step "8. Release (buyer)"
run payments release "$RAIL0_ID" --caller-address "$BUYER_ADDRESS" -p "$BUYER_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling…"
poll_until "$RAIL0_ID" released
ok "Released"

# ── Summary ───────────────────────────────────────────────────────────────────

step "Final state"
$RAIL0 payments get "$RAIL0_ID" $BASE | python3 -c "
import json, sys
d = json.load(sys.stdin)
oc = d.get('on_chain') or {}
print(f\"  status:     {d['status']}\")
print(f\"  capturable: {oc.get('capturable_amount', '-')}\")
print(f\"  refundable: {oc.get('refundable_amount', '-')}\")
print()
for t in d.get('transactions', []):
    print(f\"  {t['operation']:12s} {t['status']:12s} {(t.get('transaction_hash') or '')[:22]}…\")
"
echo
echo "✓ partial-capture × 2 → partial-refund × 2 → release complete"
