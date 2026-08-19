package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Payment lifecycle operations",
	Long: `Manage the full RAIL0 payment lifecycle.

Every on-chain operation is atomic: pass --private-key (-p) and the CLI prepares
the unsigned transaction, signs it locally, and broadcasts it in a single step.

  rail0 payments authorize <rail0Id> -p 0xYourPrivateKey…

Authorize-and-capture flow:
  1. payments create -F 0xBuyer… -p 0xPayerKey…  — payer creates + signs
  2. payments authorize <rail0Id> -p 0xPayeeKey…  — payee authorizes
  3. payments capture   <rail0Id> -a <n> -p 0xPayeeKey…  — payee captures

Charge (one-shot) flow:
  1. payments create -C -F 0xBuyer… -p 0xPayerKey…
  2. payments charge <rail0Id> -p 0xPayeeKey…`,
}

// ── payments get ─────────────────────────────────────────────────────────────

var paymentsGetCmd = &cobra.Command{
	Use:   "get <rail0Id>",
	Short: "Fetch the current state of a payment",
	Long: `Fetch a payment's current status from the RAIL0 gateway together with its
last confirmed on-chain balances (capturableAmount, refundableAmount).

Those two are MIRRORS, not live reads: the gateway writes them from the balance
the indexer reports on each confirmation, and never recomputes them. So while a
capture or refund is broadcast but not yet confirmed, they still show the
pre-operation values — and the gateway's own residual guards read the same
mirror, so a second operation against a stale balance is accepted here and
refused on-chain. Use 'rail0 payments wait' to get a settled value before
deciding an amount.

Requires login — a payment is readable only by its payer or payee. Run
'rail0 auth login -p 0xKey…' first (any wallet can log in, no account needed).

Status values:
  unsigned   — created, payer signature not yet submitted
  signed              — payer signature stored, awaiting payee action
  authorized          — funds held in escrow, capture available
  charged             — one-shot charge executed, refund available
  captured            — funds fully captured by payee
  partially_captured  — partial capture executed; remaining escrow still capturable
  voided              — authorization cancelled (only before any capture), funds returned to payer
  released            — full authorization released back to payer (total release, nothing captured)
  refunded            — payment fully settled by refund (escrow and refundable both drained)
  partially_refunded  — legacy; no longer produced (a partial refund leaves the status unchanged)
  failed              — terminal failure; payment cannot proceed`,
	Example: `  rail0 payments get 0xabc123…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newReadClient()
			id := args[0]
			p, err := client.Payments.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			// Human companions alongside the raw fields — see cmd/humanize.go.
			return humanize(ctx, p), nil
		})
	},
}

// ── payments list ────────────────────────────────────────────────────────────

var (
	paymentsListSort        string
	paymentsListChainID     int
	paymentsListDisputed    bool
	paymentsListStatus      string
	paymentsListMode        string
	paymentsListPayer       string
	paymentsListPayee       string
	paymentsListToken       string
	paymentsListMinAmount   string
	paymentsListMaxAmount   string
	paymentsListCreatedFrom string
	paymentsListCreatedTo   string
	paymentsListPage        int
	paymentsListPerPage     int
	paymentsListAll         bool
)

var paymentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List payments for the authenticated wallet (requires login)",
	Long: `Returns all payments where your wallet is payer or payee.
Requires authentication — run 'rail0 auth login -p 0xKey…' first.

Filter with --status, --mode, --payer, --payee, --token, --chain-id, --disputed,
--min-amount / --max-amount, and --created-from / --created-to (ISO-8601).

--min-amount / --max-amount are DECIMALS in token units ('1.00'), like every other
amount flag. Converting them to the base units the gateway filters on needs the
token's decimals, so both --chain-id and --token are required with them — and an
amount range only means something within one token anyway.`,
	Example: `  rail0 payments list
  rail0 payments list --sort -created_at
  rail0 payments list --status authorized --mode charge
  rail0 payments list --chain-id 5042002 --disputed
  rail0 payments list --token 0x3600… --min-amount 1000000 --created-from 2026-01-01T00:00:00Z`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		// The gateway filters in base units; these flags now take decimals like every
		// other --amount in the CLI, so they are converted here.
		minAmount, maxAmount, err := amountFilters(ctx, paymentsListMinAmount, paymentsListMaxAmount,
			paymentsListChainID, paymentsListToken)
		if err != nil {
			return err
		}
		p := rail0.ListPaymentsParams{
			Sort:        paymentsListSort,
			Status:      paymentsListStatus,
			Mode:        paymentsListMode,
			Payer:       paymentsListPayer,
			Payee:       paymentsListPayee,
			Token:       paymentsListToken,
			ChainID:     paymentsListChainID,
			MinAmount:   minAmount,
			MaxAmount:   maxAmount,
			CreatedFrom: paymentsListCreatedFrom,
			CreatedTo:   paymentsListCreatedTo,
			Page:        paymentsListPage,
			PerPage:     paymentsListPerPage,
		}
		// Only send disputed when the flag was explicitly set (tri-state).
		if cmd.Flags().Changed("disputed") {
			p.Disputed = &paymentsListDisputed
		}
		return ui.RunAPI(func() (any, error) {
			client := newReadClient()
			if paymentsListAll {
				return fetchAllPayments(ctx, client, p)
			}
			resp, err := client.Payments.List(ctx, p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; page/total live in resp.Meta
		})
	},
}

// fetchAllPayments pages through the entire result set (used by `list --all`),
// concatenating each page until every row is collected. PerPage defaults to 100
// so it isn't limited to the API's small default page size.
func fetchAllPayments(ctx context.Context, client *rail0.Client, p rail0.ListPaymentsParams) (any, error) {
	if p.PerPage == 0 {
		p.PerPage = 100
	}
	var all []rail0.Payment
	for page := 1; ; page++ {
		p.Page = page
		resp, err := client.Payments.List(ctx, p)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		if len(resp.Data) == 0 || len(all) >= resp.Meta.Total {
			break
		}
	}
	return humanizeAll(ctx, all), nil
}

// ── payments transactions ─────────────────────────────────────────────────────

var (
	paymentsTransactionsSort    string
	paymentsTransactionsPage    int
	paymentsTransactionsPerPage int
)

var paymentsTransactionsCmd = &cobra.Command{
	Use:   "transactions <rail0Id>",
	Short: "List on-chain transactions for a payment",
	Args:  cobra.ExactArgs(1),
	Example: `  rail0 payments transactions 0xabc123…
  rail0 payments transactions 0xabc123… --sort -submitted_at`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newReadClient()
			id := args[0]
			p := rail0.ListTransactionsParams{Sort: paymentsTransactionsSort, Page: paymentsTransactionsPage, PerPage: paymentsTransactionsPerPage}
			resp, err := client.Payments.Transactions(ctx, id, p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── payments create ───────────────────────────────────────────────────────────

var signPrivateKey string
var signCreateFlags createPaymentFlags
var signCreateIdempotencyKey string

var paymentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a payment and sign it with the payer's key",
	Long: `Create a RAIL0 payment and sign the EIP-3009 payload with the payer's key in
one step (--private-key / -p). The API stores the signature; the payee can then
authorize. If --private-key is omitted, the CLI prompts for it (hidden) on a
terminal — so the key stays out of the shell history and process arguments.

--from (payer) and --to (payee) must be different addresses: a payment can't be
sent from an address to itself.`,
	Example: `  rail0 payments create \
    -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 \
    -p 0xPayerKey…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Create + sign: payer signs the EIP-712 payload locally with the key
		// (--private-key, or RAIL0_PRIVATE_KEY).
		signingKey, err := signingKeyOrPrompt(signPrivateKey)
		if err != nil {
			return err
		}
		req, err := signCreateFlags.build()
		if err != nil {
			return err
		}
		// Preventive check: the payer signs with their own key. Compare the signing
		// key's address to the resolved payer up front, so signing with the wrong
		// key (e.g. the payee's) fails immediately with a clear message instead of a
		// gateway signer_mismatch after the create round-trip.
		signer, err := addressFromKey(signingKey)
		if err != nil {
			return err
		}
		if !strings.EqualFold(signer, string(req.Payer)) {
			return fmt.Errorf("signing key address %s does not match the payer %s (--from) — you're signing with the wrong key", signer, req.Payer)
		}
		// The gateway gates every /payments write behind a SIWE session, and
		// create requires the payer to BE the authenticated address. Sign in with
		// the payer's key (the same one we sign the payload with) so the session's
		// address == payer; a no-op when already logged in as the payer.
		// Build the client FIRST, then sign in on it: Auth.Login stashes the JWT on
		// the client it runs against, so the order matters. (#59)
		client := newClient()
		if err := ensureSession(ctx, client, signingKey, string(req.Payer)); err != nil {
			return err
		}
		// Replay safety (#91). rail0_id is server-minted and random, so nothing
		// else deduplicates a create: without a key, re-running this command after
		// a lost response (timeout, 429) mints a SECOND payment the merchant can
		// also authorize. One key per invocation — a caller that wants to retry
		// the exact same create across processes passes --idempotency-key.
		idemKey := signCreateIdempotencyKey
		if idemKey == "" {
			if idemKey, err = newIdempotencyKey(); err != nil {
				return err
			}
		}
		return ui.RunAPIWithMessage("Creating payment…", func() (any, error) {
			created, err := client.Payments.Create(ctx, req, idemKey)
			if err != nil {
				return nil, err
			}
			// Check the authorization against what was ASKED FOR before signing it.
			// Everything on the right-hand side here came from the user (the flags
			// they typed) or from the key itself, not from the response being
			// checked — see verifySigningPayload.
			if err := verifySigningPayload(created.SigningPayload, payloadExpectation{
				signer:         signer,
				chainID:        req.ChainID,
				token:          string(req.Token),
				valueBaseUnits: expectedBaseUnits(ctx, req.ChainID, string(req.Token), req.Amount),
			}); err != nil {
				return nil, err
			}
			// Sign the gateway's payload verbatim — see cmd/signing.go for why the
			// CLI must not rebuild the digest (#93).
			sig, err := signSigningPayload(created.SigningPayload, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Sign(ctx, created.ID,
				rail0.PayerSignatureRequest{Signature: sig})
		})
	},
}

// ── payments sign ─────────────────────────────────────────────────────────────

var paymentsSignCmd = &cobra.Command{
	Use:   "sign <rail0Id>",
	Short: "Deposit the payer signature on an already-created payment",
	Long: `Sign an existing unsigned payment with the payer's key.

'payments create' creates AND signs in one step. This command is the recovery
path for when that second half didn't happen: the payment exists at status
'unsigned' but carries no signature, because the local signing or the submit
failed (a lost connection, a rate limit, the wrong key). Without it the payment
would be stranded — the only way forward would be creating a duplicate.

The payer signs locally: the key never leaves this machine, only the signature
is sent. Requires the payer's own key, since the gateway checks that the
recovered signer is the payment's payer.`,
	Example: `  rail0 payments sign 0xabc123… -p 0xPayerKey…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		signingKey, err := signingKeyOrPrompt(signPrivateKey)
		if err != nil {
			return err
		}
		signer, err := addressFromKey(signingKey)
		if err != nil {
			return err
		}

		client := newClient()
		// The whole of /payments is SIWE-gated, and a single payment is readable
		// only by its participants — so authenticate before the GET, as the payer
		// whose key we are about to sign with.
		if err := ensureSession(ctx, client, signingKey, signer); err != nil {
			return err
		}

		return ui.RunAPIWithMessage("Signing payment…", func() (any, error) {
			payment, err := client.Payments.Get(ctx, rail0.Bytes32(args[0]))
			if err != nil {
				return nil, err
			}
			// Fail on the wrong key BEFORE signing: the gateway would answer
			// signer_mismatch, but the local message names both addresses.
			if !strings.EqualFold(signer, string(payment.Payer)) {
				return nil, fmt.Errorf(
					"signing key address %s is not the payer of this payment (%s) — you're signing with the wrong key",
					signer, payment.Payer)
			}
			// The gateway only embeds signing_payload while the payment is unsigned,
			// so an already-signed one is reported as such instead of as a missing
			// payload.
			if payment.SigningPayload == nil {
				return nil, fmt.Errorf(
					"payment %s is %s, not unsigned — there is no signature to deposit",
					args[0], payment.Status)
			}
			// The payment's own stored amount is the expectation here: the caller named
			// a payment rather than an amount, so this is what they are agreeing to.
			if err := verifySigningPayload(payment.SigningPayload, payloadExpectation{
				signer:         signer,
				chainID:        payment.ChainID,
				token:          string(payment.Token),
				valueBaseUnits: payment.Amount,
			}); err != nil {
				return nil, err
			}
			sig, err := signSigningPayload(payment.SigningPayload, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Sign(ctx, payment.ID,
				rail0.PayerSignatureRequest{Signature: sig})
		})
	},
}

// newIdempotencyKey mints a random key for a single create invocation. Random
// rather than derived from the request: two DELIBERATE creates with identical
// terms (a customer buying the same thing twice) must both go through, so the
// key identifies the attempt, not the payload.
func newIdempotencyKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate an idempotency key: %w", err)
	}
	return "cli-" + hex.EncodeToString(buf), nil
}

// ── action command helpers ────────────────────────────────────────────────────

// addSigningFlags attaches the signing-key flags to cmd: --private-key / -p and
// the hidden alias --pk. Every on-chain operation is atomic: the CLI prepares
// the unsigned transaction, signs it locally with this key, and broadcasts it in
// a single call.
func addSigningFlags(cmd *cobra.Command, privateKey *string) {
	cmd.Flags().StringVarP(privateKey, "private-key", "p", "",
		"Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step")
	cmd.Flags().StringVar(privateKey, "pk", "", "Alias for --private-key")
	_ = cmd.Flags().MarkHidden("pk")
}

// resolveSigningKey determines the private key to sign with.
// Precedence: --private-key / --pk (explicit) > RAIL0_PRIVATE_KEY env var.
// When the explicit value starts with "@", it names a key stored in the OS
// keychain ('rail0 keys add') and is resolved to the underlying hex via loadKey.
// Returns a clear error when none is available.
func resolveSigningKey(explicitKey string) (string, error) {
	if explicitKey != "" {
		// "@name" resolves to a key saved in the OS keychain ('rail0 keys add').
		if strings.HasPrefix(explicitKey, "@") {
			return loadKey(strings.TrimPrefix(explicitKey, "@"))
		}
		return explicitKey, nil
	}
	if env := os.Getenv("RAIL0_PRIVATE_KEY"); env != "" {
		return env, nil
	}
	return "", errors.New("no signing key: pass --private-key (-p / --pk), set RAIL0_PRIVATE_KEY, or save one with 'rail0 keys add' and pass it as @name")
}

// signingKeyOrPrompt resolves the signing key like resolveSigningKey, but when
// none was supplied (no --private-key/--pk, no RAIL0_PRIVATE_KEY) it asks for one
// WITHOUT echoing — the same masked prompt as `keys add`, or a single line read
// from a piped stdin — instead of erroring. This keeps the key out of the shell
// history and the process arguments while letting the signing commands run
// interactively. The RunE handlers use this; resolveSigningKey stays pure (and
// unit-testable) as the non-interactive core.
func signingKeyOrPrompt(explicitKey string) (string, error) {
	if key, err := resolveSigningKey(explicitKey); err == nil {
		return key, nil
	}
	key, err := promptSecret("Private key (0x-prefixed hex, or @name for a saved key; hidden)")
	if err != nil {
		return "", err
	}
	if key = strings.TrimSpace(key); key == "" {
		return "", errors.New("no signing key provided")
	}
	// Accept the same "@name" keychain reference as the --private-key flag, so the
	// interactive prompt and the flag behave identically — a saved key can be used
	// without pasting the raw secret.
	if strings.HasPrefix(key, "@") {
		return loadKey(strings.TrimPrefix(key, "@"))
	}
	return key, nil
}

// ensureSignerFor fetches the payment and checks the signing key's address is one
// the RAIL0 contract accepts as msg.sender for op — so signing with the wrong key
// fails locally with a clear message, before the prepare/submit round-trip and
// before the gateway rejects it (mirrors the gateway's ensure_signed_by! guard).
//
// It returns the payment and the signer address it already had to obtain, bundled as
// a signingContext, so the pre-sign checks in verifyUnsignedTx run on data ALREADY in
// hand — no second payment fetch and no second key derivation. (#63)
func ensureSignerFor(ctx context.Context, client *rail0.Client, rail0ID, signingKey, op string) (*signingContext, error) {
	signer, err := addressFromKey(signingKey)
	if err != nil {
		return nil, err
	}
	// The gateway gates the whole /payments surface behind a SIWE session and
	// these reads are participant-only. The command holds the signer's key, so
	// sign in as that address up front (no separate `auth login` needed) — a
	// no-op when already logged in as it. Mirrors what `create` does. (#48)
	if err := ensureSession(ctx, client, signingKey, signer); err != nil {
		return nil, err
	}
	p, err := client.Payments.Get(ctx, rail0ID)
	if err != nil {
		return nil, err
	}
	allowed, err := allowedSigners(op, p)
	if err != nil {
		return nil, err
	}
	for _, a := range allowed {
		if strings.EqualFold(signer, a) {
			return &signingContext{payment: p, signer: signer}, nil
		}
	}
	return nil, fmt.Errorf("signing key address %s can't %s this payment — expected %s (wrong key?)",
		signer, op, strings.Join(allowed, " or "))
}

// canonicalOp folds the two vocabularies the CLI uses for an operation onto one key.
// The picker filter and the signer check take a human LABEL ("close dispute", which
// also has to read correctly inside an error sentence), while submitThenWait takes
// the gateway's wire name ("close_dispute"). The two coincide for every operation
// EXCEPT close_dispute — which is exactly why the drift between them was invisible.
func canonicalOp(op string) string {
	return strings.ReplaceAll(strings.TrimSpace(strings.ToLower(op)), " ", "_")
}

// allowedSigners lists the addresses the contract accepts as msg.sender for op:
// dispute/close_dispute are payer-only, release is payer-or-payee, every other
// operation is payee-only.
//
// An unrecognised operation is an ERROR rather than a silent payee-only default.
// The default was the hazard: for the two payer-only operations it produces the
// opposite of the right answer, rejecting the payer — the ONLY party permitted to
// sign them — with a "wrong key?" message pointing nowhere near the real cause. A
// caller passing the wire form "close_dispute" to a function keyed on the label
// "close dispute" was one plausible way in, and canonicalOp now closes that door
// too. (#72)
func allowedSigners(op string, p *rail0.Payment) ([]string, error) {
	switch canonicalOp(op) {
	case "dispute", "close_dispute":
		return []string{string(p.Payer)}, nil
	case "release":
		return []string{string(p.Payer), string(p.Payee)}, nil
	case "authorize", "charge", "capture", "void", "refund":
		return []string{string(p.Payee)}, nil
	default:
		return nil, fmt.Errorf("internal error: no signer rule for operation %q", op)
	}
}

// ── payments authorize ────────────────────────────────────────────────────────

var authorizePrivateKey string

var paymentsAuthorizeCmd = &cobra.Command{
	Use:   "authorize <rail0Id>",
	Short: "Authorize a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls authorize() on the RAIL0 contract,
pulling funds from the payer into escrow.

The payer must have created and signed the payment first via
'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately: broadcasting is not settling. The payment
stays at "signed" until rail0-indexer sees the on-chain event and the API
transitions it to "authorized".

Add -w/--wait to block until that happens (or the operation fails) and exit
non-zero if it didn't land — that is almost always what you want, and it saves
polling 'payments get <rail0Id>' by hand. Without it the command returns as
soon as the transaction is broadcast.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # 1. Payer creates and signs the payment in one step
  rail0 payments create --from @buyer --to @merchant \
    --token USDC --amount 1.00 --chain-id 5042002 -p @payer

  # 2. Payee authorizes, and waits for the chain to confirm it
  rail0 payments authorize <rail0Id> -p @merchant-key --wait`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(authorizePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "authorize")
		if err != nil {
			return err
		}
		rail0Id := idInput
		return ui.RunAPIWithMessage(actionLabel("Authorizing…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "authorize")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.AuthorizePrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "authorize", func() (any, error) {
				return client.Payments.Authorize(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments charge ───────────────────────────────────────────────────────────

var chargePrivateKey string

var paymentsChargeCmd = &cobra.Command{
	Use:   "charge <rail0Id>",
	Short: "Charge a payment — authorize + capture in one call (no escrow)",
	Long: `Prepare the transaction that calls charge() on the RAIL0 contract.
Combines authorize + capture into a single on-chain call; funds move
directly from payer to payee without an escrow window.

The payment must have been created with -C/--charge and signed by the
payer via 'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately: broadcasting is not settling. The payment
stays at "signed" until rail0-indexer sees the on-chain event and the API
transitions it to "charged".

Add -w/--wait to block until that happens (or the operation fails) and exit
non-zero if it didn't land, instead of polling 'payments get <rail0Id>'.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  rail0 payments charge 0xrail0Id… -p @merchant-key --wait`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(chargePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "charge")
		if err != nil {
			return err
		}
		rail0Id := idInput
		// Charge is an immediate payer→payee transfer — the most irreversible
		// operation the CLI can broadcast. (#104)
		if err := confirmDestructive(fmt.Sprintf("Charge payment %s? This immediately transfers the full amount from the payer to the payee.", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Charging…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "charge")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.ChargePrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "charge", func() (any, error) {
				return client.Payments.Charge(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments capture ──────────────────────────────────────────────────────────

var captureAmount string
var capturePrivateKey string

var paymentsCaptureCmd = &cobra.Command{
	Use:   "capture <rail0Id>",
	Short: "Capture a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls capture() on the RAIL0 contract.

The payment must be in 'authorized' or 'partially_captured' status.
Partial captures are supported: pass --amount less than capturableAmount
to capture a portion; the remaining balance stays available until
authorizationExpiry.

The API returns 202 immediately: broadcasting is not settling. The payment
stays at "authorized" until rail0-indexer sees the on-chain event and the API
transitions it to "captured" or "partially_captured".

Add -w/--wait to block until that happens (or the operation fails) and exit
non-zero if it didn't land. On partial captures it is close to required:
capturableAmount is the LAST CONFIRMED balance, so it reads stale while a
capture of yours is still in flight, and picking the next amount from it is how
you over- or under-capture. ('payments wait' settles it separately if you'd
rather not block the capture itself.)

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Full capture, waiting for the chain to confirm it
  rail0 payments capture 0xrail0Id… --amount 1.00 -p @merchant-key --wait

  # Partial capture — --wait settles the balance before the next one
  rail0 payments capture 0xrail0Id… --amount 0.50 -p @merchant-key --wait`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(capturePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "capture")
		if err != nil {
			return err
		}
		rail0Id := idInput
		// validateAmount was wired only into `create`, so the two commands that
		// actually move money took the amount raw: `-a 1,50`, `-a 0` or `-a -1` each
		// burned a session round-trip AND ensureSignerFor's payment fetch before the
		// gateway answered 422 — while the identical typo on `create` was caught
		// instantly. Checked here, before any network call, and inside the prompt so
		// interactive input is rejected inline. (#61)
		if captureAmount == "" {
			captureAmount, err = promptValidated("Amount", "decimal in token units (e.g. 1.00)", false, validateAmount)
			if err != nil {
				return err
			}
		}
		if err := validateAmount(captureAmount); err != nil {
			return err
		}
		// Capture settles escrow to the merchant, irreversibly — the same gate
		// void/release/refund already have, naming the amount at stake. (#104)
		if err := confirmDestructive(fmt.Sprintf("Capture %s from payment %s? This settles the funds to the merchant.", captureAmount, shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Capturing…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "capture")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.CapturePrepare(ctx, rail0Id, captureAmount)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "capture", func() (any, error) {
				return client.Payments.Capture(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments void ─────────────────────────────────────────────────────────────

var voidPrivateKey string

var paymentsVoidCmd = &cobra.Command{
	Use:   "void <rail0Id>",
	Short: "Void a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls void() on the RAIL0 contract.

The payment must be in 'authorized' status with NOTHING captured yet: void is
forbidden once any (even partial) capture has occurred (the contract reverts
AlreadyCaptured) — recover the uncaptured remainder via 'release' instead. Void
cancels the authorization and returns the full escrowed balance to the payer.
Only callable before authorizationExpiry, by the payee.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land, instead of polling 'payments get <rail0Id>' yourself.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  rail0 payments void 0xrail0Id… -p @merchant-key --wait`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(voidPrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "void")
		if err != nil {
			return err
		}
		rail0Id := idInput
		if err := confirmDestructive(fmt.Sprintf("Void payment %s? This returns the full escrow to the payer.", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Voiding…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "void")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.VoidPrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "void", func() (any, error) {
				return client.Payments.Void(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments release ──────────────────────────────────────────────────────────

var releasePrivateKey string

var paymentsReleaseCmd = &cobra.Command{
	Use:   "release <rail0Id>",
	Short: "Release a payment after expiry (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls release() on the RAIL0 contract.

Release is only available after authorizationExpiry has passed on-chain.
Returns the remaining (uncaptured) escrowed balance to the payer. Permissionless
— both payer and payee can call it; the caller is whoever the signing key belongs
to (derived from the key), so pass the payer's key to release as the payer.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land.

Note that the payment STATUS advances to "released" only on a TOTAL release (an
untouched authorization, nothing captured); if a capture left a refundable
residual the status stays unchanged and that residual remains refundable. This
does not weaken --wait: it waits on THIS operation's transaction reaching
confirmed or failed, not on a status change — so it settles a partial release
correctly, where waiting for a new status would hang until the deadline.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Payee releases
  rail0 payments release 0xrail0Id… -p @merchant-key --wait

  # Payer releases (just use the payer's key)
  rail0 payments release 0xrail0Id… -p @buyer-key --wait`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(releasePrivateKey)
		if err != nil {
			return err
		}
		// The on-chain caller is whoever signs, so derive it from the signing key
		// rather than taking a separate flag.
		caller, err := addressFromKey(signingKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "release")
		if err != nil {
			return err
		}
		rail0Id := idInput
		if err := confirmDestructive(fmt.Sprintf("Release payment %s? This returns the uncaptured escrow to the payer.", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Releasing…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "release")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.ReleasePrepare(ctx, rail0Id, caller)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "release", func() (any, error) {
				return client.Payments.Release(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments refund ───────────────────────────────────────────────────────────

var refundAmount string
var refundPrivateKey string

var paymentsRefundCmd = &cobra.Command{
	Use:   "refund <rail0Id>",
	Short: "Refund a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls refund() on the RAIL0 contract,
pulling refundAmount from the payee and returning it to the payer.

The payment must be in 'charged', 'captured', or 'partially_captured' status
with a positive refundableAmount. Partial refunds are supported: pass --amount
less than refundableAmount.

No separate approval step is needed: the CLI signs the payee's EIP-3009
ReceiveWithAuthorization payload locally with the same key, so the refund
funds are pulled from the payee without a prior allowance grant.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land.

Note that the payment STATUS advances to "refunded" only when the payment is
fully settled (both escrow and refundable drained); a partial refund leaves it
unchanged. This does not weaken --wait: it waits on THIS operation's transaction
reaching confirmed or failed, not on a status change — so it settles a partial
refund correctly, where waiting for a new status would hang until the deadline.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Full refund
  rail0 payments refund 0xrail0Id… --amount 1.00 -p @merchant-key --wait

  # Partial refund
  rail0 payments refund 0xrail0Id… --amount 0.25 -p @merchant-key --wait`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(refundPrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "refund")
		if err != nil {
			return err
		}
		rail0Id := idInput
		// Same local check as capture — see the note there. (#61)
		if refundAmount == "" {
			refundAmount, err = promptValidated("Amount", "decimal in token units (e.g. 1.00)", false, validateAmount)
			if err != nil {
				return err
			}
		}
		if err := validateAmount(refundAmount); err != nil {
			return err
		}
		if err := confirmDestructive(fmt.Sprintf("Refund payment %s? This sends funds back to the payer.", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Refunding…"), func() (any, error) {
			// Phase 1: get the EIP-712 ReceiveWithAuthorization signing payload.
			// Omitting the signature returns a Transaction whose SigningPayload is set.
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "refund")
			if err != nil {
				return nil, err
			}
			phase1, err := client.Payments.RefundPrepare(ctx, rail0Id, rail0.PrepareRequest{Amount: refundAmount})
			if err != nil {
				return nil, err
			}
			if phase1.SigningPayload == nil {
				return nil, errors.New("refund prepare did not return a signing_payload")
			}

			// This is the payload the whole guard exists for: a ReceiveWithAuthorization
			// is honoured by the token itself, so an endpoint that answered with the
			// payee's whole balance as `value` used to get exactly that signed.
			// `refundAmount` is what the user typed, scaled locally.
			if err := sc.verifySigningPayload(phase1.SigningPayload,
				expectedBaseUnits(ctx, sc.payment.ChainID, string(sc.payment.Token), refundAmount)); err != nil {
				return nil, err
			}
			// Sign the EIP-3009 ReceiveWithAuthorization payload (payee key).
			v, r, s, err := signRefundPayload(phase1.SigningPayload, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign refund payload: %w", err)
			}
			// Combine V/R/S into a single 65-byte 0x-prefixed signature for phase 2.
			signature := "0x" + r[2:] + s[2:] + fmt.Sprintf("%02x", v)

			// Phase 2: submit the signature → get the unsigned transaction with the
			// EIP-3009 authorization embedded.
			phase2, err := client.Payments.RefundPrepare(ctx, rail0Id, rail0.PrepareRequest{
				Amount:    refundAmount,
				Signature: signature,
			})
			if err != nil {
				return nil, fmt.Errorf("refund prepare phase 2: %w", err)
			}

			// Sign the unsigned EIP-1559 transaction and broadcast.
			if err := sc.verify(phase2.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(phase2.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign tx: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "refund", func() (any, error) {
				return client.Payments.Refund(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

// ── payments dispute ───────────────────────────────────────────────────────

var disputeReason string
var disputePrivateKey string
var closeDisputeReason string
var closeDisputePrivateKey string

var paymentsDisputeCmd = &cobra.Command{
	Use:   "dispute <rail0Id>",
	Short: "Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast",
	Long: `Prepare the transaction that calls dispute() on the RAIL0 contract, then
sign and submit it like any other operation (the gateway broadcasts it).

Disputes are payer-driven and signal-only: they move no money. Only the payer
may dispute, while the refund window is open and the merchant holds the funds.

Signal-only does not mean instant: it is still an on-chain transaction, so the
API returns 202 and the dispute is not visible until it confirms. Add -w/--wait
to block until it does (or fails).

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.`,
	Example: `  rail0 payments dispute 0xrail0Id… -p @buyer-key --reason 0x… --wait`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(disputePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "dispute")
		if err != nil {
			return err
		}
		rail0Id := idInput
		if disputeReason == "" {
			disputeReason, err = promptValue("Reason", "optional bytes32 reason (0x…); blank for none", true)
			if err != nil {
				return err
			}
		}
		// An on-chain, gas-burning action the counterparty sees — not undoable
		// except by explicitly closing the dispute. (#104)
		if err := confirmDestructive(fmt.Sprintf("Open a dispute on payment %s? This is an on-chain action the merchant will see.", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Opening dispute…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "dispute")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.DisputePrepare(ctx, rail0Id, disputeReason)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "dispute", func() (any, error) {
				return client.Payments.Dispute(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

var paymentsDisputeCloseCmd = &cobra.Command{
	Use:   "close <rail0Id>",
	Short: "Close a dispute (payer only): prepare, sign, broadcast",
	Long: `Prepare the transaction that calls closeDispute() on the RAIL0 contract, then
sign and submit it like any other operation.

Only the PAYER may withdraw a dispute — a merchant cannot dismiss one. The
merchant's only way to close a dispute is a full 'refund', which closes it
automatically. A withdrawn dispute can be reopened while the refund window is
open.

Like 'dispute', this is an on-chain transaction: the API returns 202 and the
close is not visible until it confirms. Add -w/--wait to block until it does
(or fails).

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.`,
	Example: `  rail0 payments dispute close 0xrail0Id… -p @buyer-key --wait`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		signingKey, err := signingKeyOrPrompt(closeDisputePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		idInput, err := pickPaymentID(ctx, client, args, "close dispute")
		if err != nil {
			return err
		}
		rail0Id := idInput
		if closeDisputeReason == "" {
			closeDisputeReason, err = promptValue("Reason", "optional bytes32 reason (0x…); blank for none", true)
			if err != nil {
				return err
			}
		}
		// Closing is on-chain too, and reopening requires a fresh dispute. (#104)
		if err := confirmDestructive(fmt.Sprintf("Close the dispute on payment %s?", shortID(rail0Id))); err != nil {
			return err
		}
		return ui.RunAPIWithMessage(actionLabel("Closing dispute…"), func() (any, error) {
			sc, err := ensureSignerFor(ctx, client, rail0Id, signingKey, "close dispute")
			if err != nil {
				return nil, err
			}
			prep, err := client.Payments.CloseDisputePrepare(ctx, rail0Id, closeDisputeReason)
			if err != nil {
				return nil, err
			}
			if err := sc.verify(prep.UnsignedTransaction); err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return submitThenWait(ctx, client, rail0Id, "close_dispute", func() (any, error) {
				return client.Payments.CloseDispute(ctx, rail0Id,
					rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
			})
		})
	},
}

var (
	disputesStatus  string
	disputesSort    string
	disputesPage    int
	disputesPerPage int
)

var paymentsDisputesCmd = &cobra.Command{
	Use:   "disputes <rail0Id>",
	Short: "List a payment's dispute open/close history",
	Args:  cobra.ExactArgs(1),
	Example: `  rail0 payments disputes 0xrail0Id…
  rail0 payments disputes 0xrail0Id… --status open`,
	RunE: func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newReadClient()
			id := args[0]
			resp, err := client.Payments.Disputes(ctx, id,
				rail0.ListDisputesParams{Status: disputesStatus, Sort: disputesSort, Page: disputesPage, PerPage: disputesPerPage})
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// payments create — creates the payment and signs it with the payer's key
	signCreateFlags.addTo(paymentsCreateCmd)
	paymentsCreateCmd.Flags().StringVarP(&signPrivateKey, "private-key", "p", "",
		"Payer's secp256k1 private key (0x-prefixed hex); creates the payment and signs locally")
	paymentsCreateCmd.Flags().StringVar(&signPrivateKey, "pk", "", "Alias for --private-key")
	_ = paymentsCreateCmd.Flags().MarkHidden("pk")
	paymentsCreateCmd.Flags().StringVar(&signCreateIdempotencyKey, "idempotency-key", "",
		"Replay-safety key (default: a fresh one per invocation). Reuse it to retry the SAME create without minting a second payment")
	paymentsCmd.AddCommand(paymentsCreateCmd)

	// payments sign — deposits the payer signature on an already-created payment
	paymentsSignCmd.Flags().StringVarP(&signPrivateKey, "private-key", "p", "",
		"Payer's secp256k1 private key (0x-prefixed hex); signs locally, the key never leaves this machine")
	paymentsSignCmd.Flags().StringVar(&signPrivateKey, "pk", "", "Alias for --private-key")
	_ = paymentsSignCmd.Flags().MarkHidden("pk")
	paymentsCmd.AddCommand(paymentsSignCmd)

	// payments get
	paymentsCmd.AddCommand(paymentsGetCmd)

	// payments list
	paymentsListCmd.Flags().StringVar(&paymentsListSort, "sort", "", "Sort fields (e.g. -created_at,amount)")
	paymentsListCmd.Flags().StringVar(&paymentsListStatus, "status", "", "Filter by payment status (e.g. authorized, captured, refunded)")
	paymentsListCmd.Flags().StringVar(&paymentsListMode, "mode", "", "Filter by mode (authorize or charge)")
	paymentsListCmd.Flags().StringVar(&paymentsListPayer, "payer", "", "Filter by payer address")
	paymentsListCmd.Flags().StringVar(&paymentsListPayee, "payee", "", "Filter by payee address")
	paymentsListCmd.Flags().StringVar(&paymentsListToken, "token", "", "Filter by token address")
	paymentsListCmd.Flags().IntVar(&paymentsListChainID, "chain-id", 0, "Filter by EVM chain ID")
	paymentsListCmd.Flags().BoolVar(&paymentsListDisputed, "disputed", false, "Filter by whether an open dispute exists")
	paymentsListCmd.Flags().StringVar(&paymentsListMinAmount, "min-amount", "",
		"Minimum amount as a decimal in token units (inclusive); needs --chain-id and --token")
	paymentsListCmd.Flags().StringVar(&paymentsListMaxAmount, "max-amount", "",
		"Maximum amount as a decimal in token units (inclusive); needs --chain-id and --token")
	paymentsListCmd.Flags().StringVar(&paymentsListCreatedFrom, "created-from", "", "Only payments created at/after this ISO-8601 time")
	paymentsListCmd.Flags().StringVar(&paymentsListCreatedTo, "created-to", "", "Only payments created at/before this ISO-8601 time")
	paymentsListCmd.Flags().IntVar(&paymentsListPage, "page", 0, "Page number (1-based)")
	paymentsListCmd.Flags().IntVar(&paymentsListPerPage, "per-page", 0, "Results per page")
	paymentsListCmd.Flags().BoolVar(&paymentsListAll, "all", false, "Fetch every page (ignores --page)")
	_ = paymentsListCmd.RegisterFlagCompletionFunc("status", fixedCompletions(paymentStatuses...))
	_ = paymentsListCmd.RegisterFlagCompletionFunc("mode", fixedCompletions("authorize", "charge"))
	_ = paymentsListCmd.RegisterFlagCompletionFunc("payer", completeAliasNames)
	_ = paymentsListCmd.RegisterFlagCompletionFunc("payee", completeAliasNames)
	paymentsCmd.AddCommand(paymentsListCmd)

	// payments transactions
	paymentsTransactionsCmd.Flags().StringVar(&paymentsTransactionsSort, "sort", "", "Sort fields (e.g. -submitted_at,operation)")
	paymentsTransactionsCmd.Flags().IntVar(&paymentsTransactionsPage, "page", 0, "Page number (1-based)")
	paymentsTransactionsCmd.Flags().IntVar(&paymentsTransactionsPerPage, "per-page", 0, "Results per page")
	paymentsCmd.AddCommand(paymentsTransactionsCmd)

	// payments authorize
	addSigningFlags(paymentsAuthorizeCmd, &authorizePrivateKey)
	addWaitFlags(paymentsAuthorizeCmd)
	paymentsCmd.AddCommand(paymentsAuthorizeCmd)

	// payments charge
	addSigningFlags(paymentsChargeCmd, &chargePrivateKey)
	addWaitFlags(paymentsChargeCmd)
	addYesFlag(paymentsChargeCmd)
	paymentsCmd.AddCommand(paymentsChargeCmd)

	// payments capture
	paymentsCaptureCmd.Flags().StringVarP(&captureAmount, "amount", "a", "",
		"Amount to capture as a decimal in token units (e.g. 1.00); > 0 and ≤ the last confirmed capturable")
	addSigningFlags(paymentsCaptureCmd, &capturePrivateKey)
	addWaitFlags(paymentsCaptureCmd)
	addYesFlag(paymentsCaptureCmd)
	paymentsCmd.AddCommand(paymentsCaptureCmd)

	// payments void
	addSigningFlags(paymentsVoidCmd, &voidPrivateKey)
	addWaitFlags(paymentsVoidCmd)
	addYesFlag(paymentsVoidCmd)
	paymentsCmd.AddCommand(paymentsVoidCmd)

	// payments release
	addSigningFlags(paymentsReleaseCmd, &releasePrivateKey)
	addWaitFlags(paymentsReleaseCmd)
	addYesFlag(paymentsReleaseCmd)
	paymentsCmd.AddCommand(paymentsReleaseCmd)

	// payments refund
	paymentsRefundCmd.Flags().StringVarP(&refundAmount, "amount", "a", "",
		"Amount to refund as a decimal in token units (e.g. 1.00); > 0 and ≤ refundable")
	addSigningFlags(paymentsRefundCmd, &refundPrivateKey)
	addWaitFlags(paymentsRefundCmd)
	addYesFlag(paymentsRefundCmd)
	paymentsCmd.AddCommand(paymentsRefundCmd)

	// payments dispute / dispute close / disputes
	addSigningFlags(paymentsDisputeCmd, &disputePrivateKey)
	addWaitFlags(paymentsDisputeCmd)
	addYesFlag(paymentsDisputeCmd)
	paymentsDisputeCmd.Flags().StringVar(&disputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	addSigningFlags(paymentsDisputeCloseCmd, &closeDisputePrivateKey)
	addWaitFlags(paymentsDisputeCloseCmd)
	addYesFlag(paymentsDisputeCloseCmd)
	paymentsDisputeCloseCmd.Flags().StringVar(&closeDisputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	paymentsDisputeCmd.AddCommand(paymentsDisputeCloseCmd)
	paymentsCmd.AddCommand(paymentsDisputeCmd)
	paymentsDisputesCmd.Flags().StringVar(&disputesStatus, "status", "", "Filter by dispute status: open or closed")
	paymentsDisputesCmd.Flags().StringVar(&disputesSort, "sort", "", "Sort fields (e.g. -opened_at)")
	paymentsDisputesCmd.Flags().IntVar(&disputesPage, "page", 0, "Page number (1-based)")
	paymentsDisputesCmd.Flags().IntVar(&disputesPerPage, "per-page", 0, "Results per page")
	paymentsCmd.AddCommand(paymentsDisputesCmd)
}
