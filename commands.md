# rail0 — Demo tecnica del gateway

**Ambiente:** staging · Arc testnet (`5042002`) · USDC

| | Alias | Chiave | Indirizzo |
|---|---|---|---|
| Buyer | `@payer` | `@payer` | `0xe3DD…94c4` |
| Merchant | `@payee` | `@payee` | `0x2D2B…57D5` |

> **Tempi:** ogni operazione on-chain con `--wait` richiede **~1 minuto** su Arc.
> Mentre aspetti, passa all'admin e fai vedere la transazione *in flight*.

> La config CLI ha già `chain-id 5042002` e `token USDC`, quindi `-c` e `-t` si possono omettere.

---

## 0. Prima della demo (10 minuti prima, a microfoni spenti)

```bash
rail0 --version   # deve essere v1.1.0-8-g1619963 o successiva (alias nei comandi wallets)
rail0 env current
rail0 health
rail0 auth login -p @payee
```

Pagamento per il **void**: autorizzato, mai catturato.

```bash
P2=$(rail0 payments create -F @payer -T @payee -a 0.50 -d "demo void" -p @payer -q)
rail0 payments authorize "$P2" -p @payee --wait
```

Pagamento per **charge + refund**: solo creato, perché il charge lo fai dal vivo.

```bash
P3=$(rail0 payments create -F @payer -T @payee -a 1.00 -C -d "demo charge" -p @payer -q)
echo "P2=$P2  P3=$P3"
```

> Annota i due id. Se il terminale si chiude, li reimposti con `P2=0x…` e `P3=0x…`.

- [ ] Admin aperto e loggato su <https://rail0-admin-staging.fly.dev> (wallet `payee` in MetaMask)
- [ ] <https://webhook.site> aperto, URL copiato

---

## 1. Discovery

```bash
rail0 health
rail0 chains
rail0 tokens --chain-id 5042002
rail0 payment-methods --address @payee
rail0 auth status
rail0 account
```

Saldi on-chain in tempo reale del wallet merchant:

```bash
rail0 wallets balances @payee --chain-id 5042002 --symbol USDC
```

---

## 2. Flusso escrow: create → authorize → capture parziale

Il **buyer** crea e firma il pagamento (EIP-3009) in locale: la chiave non lascia il Mac.

```bash
P1=$(rail0 payments create -F @payer -T @payee -a 1.00 -d "demo ordine 42" -p @payer -q)
echo "$P1"
rail0 payments get "$P1" --pretty
```

> **Admin:** Payments → apri il pagamento (stato `signed`)

Il **merchant** autorizza e i fondi vanno in escrow (~1 min).

```bash
rail0 payments authorize "$P1" -p @payee --wait
```

> **Admin:** stato `authorized`, catturabile 1.00

Capture **parziale** (chiede conferma `y/N`):

```bash
rail0 payments capture "$P1" -a 0.40 -p @payee --wait
rail0 payments get "$P1" --pretty
```

> Da mostrare: `status: partially_captured`, catturabile 0.60,
> **`escrow_stranded: true`** e `escrow_returnable_at` = scadenza dell'autorizzazione.
> **Admin**, area operatore `/admin/payments/<id>`: riga *"Escrow stranded — until …"*

### Il punto didattico

Dopo un capture parziale il **void è escluso per sempre**. Questo comando **fallisce apposta**: leggi il messaggio, che indica `release` e la data.

```bash
rail0 payments void "$P1" -p @payee
```

Anche il **release** non è ancora possibile (l'autorizzazione dura 7 giorni su staging). Anche questo **fallisce apposta**: indica `void` e la scadenza.

```bash
rail0 payments release "$P1" -p @payee
```

---

## 3. Storia del pagamento

```bash
rail0 payments transactions "$P1"
rail0 payments history "$P1"
```

> **Admin:** Payments → `<id>` → Timeline

---

## 4. Void (sul pagamento preparato prima, con autorizzazione intatta)

```bash
rail0 payments get "$P2" --pretty
rail0 payments void "$P2" -p @payee --wait
```

> **Admin:** stato `voided`, i 0.50 tornano al buyer

---

## 5. Charge one-shot + refund

Niente escrow: i fondi vanno subito al merchant.

```bash
rail0 payments charge "$P3" -p @payee --wait
rail0 payments refund "$P3" -a 1.00 -p @payee --wait
```

---

## 6. Dispute (la apre solo il buyer)

```bash
rail0 disputes reasons
rail0 payments dispute "$P1" -p @payer --reason not_received --wait
rail0 disputes --status open
```

> **Admin:** pagina Disputes. Il merchant non può chiudere la dispute: la chiude il buyer, oppure un refund completo.

```bash
rail0 payments dispute close "$P1" -p @payer --reason withdrawn --wait
```

---

## 7. Integrazione: webhook, idempotenza, script

Sostituisci l'URL con il tuo di webhook.site:

```bash
rail0 webhooks create --name "demo" --callback-url https://webhook.site/IL-TUO-ID -t payments.authorized -t payments.captured -t payments.refunded
rail0 webhooks list
```

Chiudi anche P1 catturando il residuo con una chiave di idempotenza (fa partire il webhook):

```bash
rail0 payments capture "$P1" -a 0.60 -p @payee --idempotency-key demo-ordine-42 --wait --yes
```

Rilancialo identico: **non** apre una seconda transazione, restituisce la prima.

```bash
rail0 payments capture "$P1" -a 0.60 -p @payee --idempotency-key demo-ordine-42 --yes
```

> **webhook.site:** callback `payments.captured` con header `X-Rail0-Signature`

Output per script:

```bash
rail0 payments get "$P1" | jq -r .status
rail0 payments list --status authorized,partially_captured
```

---

## 8. Analytics

```bash
rail0 analytics summary
rail0 analytics breakdown --by operation
rail0 analytics timeseries --interval week
```

> **Admin:** Dashboard

---

## 9. Operatività (solo admin)

> **Admin**, area `/admin`: Accounts · Chains · Tokens · Sync errors

Transazione bloccata:

```bash
rail0 payments redrive <id_pagamento> <id_transazione> --wait
```

---

## Pulizia (dopo la demo)

```bash
rail0 webhooks list
rail0 webhooks delete <webhook_id>
```

---

## Se qualcosa si blocca

| Sintomo | Causa | Controllo |
|---|---|---|
| Transazione `pending` con hash `null` | worker del gateway fermo | `fly machines list -a rail0-gateway-staging` (deve esserci un worker `started`) |
| `submitted` con hash ma non confermata | indexer | `curl -s https://rail0-indexer-staging.fly.dev/health` |
| `401` / token scaduto | sessione scaduta | `rail0 auth login -p @payee` |
