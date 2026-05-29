#!/usr/bin/env bash
# test_authorize_capture_refund.sh
#
# End-to-end CLI flow: create → authorize → capture → refund (EIP-3009, no approve)
#
# Usage:
#   ./scripts/test_authorize_capture_refund.sh

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────

API_URL=http://0.0.0.0:4567
CHAIN="arc-testnet"
TOKEN="USDC"
AMOUNT=1000000   # 1 USDC (6 decimals)
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
run  payments authorize "$RAIL0_ID" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling for indexer confirmation…"
poll_until "$RAIL0_ID" authorized
ok "Authorized"

# ── 3. Capture ────────────────────────────────────────────────────────────────

step "3. Capture $AMOUNT"
run  payments capture "$RAIL0_ID" -a "$AMOUNT" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling for indexer confirmation…"
poll_until "$RAIL0_ID" captured partially_captured
ok "Captured"

# ── 4. Refund (EIP-3009, no approve needed) ───────────────────────────────────

step "4. Refund $AMOUNT (EIP-3009 receiveWithAuthorization)"
run  payments refund "$RAIL0_ID" -a "$AMOUNT" -p "$ACCOUNT_PRIVATE_KEY" $BASE > /dev/null
ok "Broadcast — polling for indexer confirmation…"
poll_until "$RAIL0_ID" refunded partially_refunded
ok "Refunded"

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
echo "✓ authorize → capture → refund complete (v9 EIP-3009)"
