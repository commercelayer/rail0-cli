package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// keysGenerateCmd creates a brand-new wallet key pair on THIS machine and prints it.
//
// It lives under `keys`, not `wallets`, on purpose. Every `wallets` verb talks to the
// gateway about wallets an account already has, behind a login; this one has no account,
// no session and no gateway — it is a local key operation, like the rest of `keys`, and
// its natural follow-up is `keys add`. Under `wallets` it would also sit next to
// `wallets create`, which registers an EXISTING address with the gateway (and is
// currently unavailable), inviting the reading that "generate" registers something too.
//
// Why it exists at all: a merchant needs an address of their own before the sign-up
// (`/signup` in the admin) or an operator invite, and RAIL0 operators never generate or
// hold merchant keys. Generating it here keeps the key where it belongs — on the
// merchant's machine — and gives a CLI-only user a way in without a browser wallet.
//
// Guarantees, each covered by a test:
//   - fully offline: no API client is constructed, and the automatic update check is
//     skipped for this command (updateCheckSkippedFor), so not even GitHub is asked;
//   - nothing is written anywhere — not the config, not the keychain, not a file. The
//     key exists only in what the command prints;
//   - the values go to stdout through the normal output layer (so --json is one clean
//     JSON object), the warning goes to stderr (so it never corrupts that JSON and is
//     still seen when stdout is redirected).
//
// There is deliberately no flag to silence the warning: it is the one moment the key
// can still be saved, and a script that does not need the warning can discard stderr.
var keysGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Create a new wallet key pair locally and print its address and private key",
	Long: `Create a new wallet (a secp256k1 key pair) on this machine and print its
EIP-55 address and its private key (0x-prefixed hex).

Fully offline: nothing is sent to the gateway or anywhere else, and nothing is
written to disk — not the config file, not the OS keychain. The private key
exists only in this command's output, and it is shown ONCE: it cannot be
recovered by RAIL0 or by anyone else. Store it now in a password manager (and,
to sign with it in a browser, import it into a wallet you control). Anyone who
has it controls the funds of the address.

The address and the key go to stdout (a JSON object with --json); the warning
goes to stderr, so piping or redirecting stdout keeps clean JSON. Mind where
stdout goes: a redirected file, a CI log, shell scrollback or a shared screen
all end up holding the key.

Next steps:
  rail0 keys add <name>     save it in the OS keychain for -p @<name> (prompts hidden)
  then register the ADDRESS (never the key) with RAIL0: sign up as a merchant
  (/signup in the admin) or ask an operator for an invite, and claim the account
  by signing with this wallet. Adding it to an existing account
  ('wallets create') is currently unavailable.`,
	Example: `  rail0 keys generate
  rail0 keys generate --json`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		w, err := generateWallet()
		if err != nil {
			return err
		}
		if err := ui.RunAPI(func() (any, error) { return w, nil }); err != nil {
			return err
		}
		printGeneratedKeyWarning(os.Stderr, term.IsTerminal(os.Stderr.Fd()), !term.IsTerminal(os.Stdout.Fd()))
		return nil
	},
}

// generatedWallet is the command's result. The JSON field names are the output
// contract for --json: {"address": …, "private_key": …}.
type generatedWallet struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}

// generateWallet creates a fresh secp256k1 key with go-ethereum's GenerateKey, which
// draws from crypto/rand (the OS CSPRNG) — never math/rand, never a seed of ours.
// The key is encoded with FromECDSA, which left-pads to exactly 32 bytes, so a key
// whose top byte happens to be zero still prints as 64 hex digits and round-trips
// through every key parser in this CLI (addressFromKey, rail0.HexToPrivateKey). The
// address is derived from the public key and rendered EIP-55 checksummed by Hex().
func generateWallet() (generatedWallet, error) {
	key, err := ethcrypto.GenerateKey()
	if err != nil {
		return generatedWallet{}, fmt.Errorf("generate key: %w", err)
	}
	return generatedWallet{
		Address:    ethcrypto.PubkeyToAddress(key.PublicKey).Hex(),
		PrivateKey: hexutil.Encode(ethcrypto.FromECDSA(key)),
	}, nil
}

// printGeneratedKeyWarning writes the one-time-secret warning. It never contains the
// key itself — the warning goes to stderr, which is the stream people leave on a
// terminal, log, or paste into an issue. styled colours the headline (only when stderr
// is a terminal, so a captured log gets no escape codes); stdoutRedirected adds a
// reminder that the key now sits wherever stdout went (a file, a pipe, a log).
func printGeneratedKeyWarning(w io.Writer, styled, stdoutRedirected bool) {
	headline := "WARNING: save this private key NOW. It is shown only once."
	if styled {
		headline = ui.ErrStyle.Bold(true).Render(headline)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, headline)
	fmt.Fprint(w, `  - It cannot be recovered — not by RAIL0, not by anyone. Lose it and the funds
    of this address are gone.
  - Store it now in a password manager or another secure offline backup.
  - Anyone who has it controls the funds. Never paste it into a website, a chat
    or an email, and never share it with RAIL0 operators: they will never ask.
  - Mind your shell history, terminal scrollback and screen sharing: clear the
    screen once it is saved.
`)
	if stdoutRedirected {
		fmt.Fprintln(w, "  - stdout is not a terminal: the key was written wherever it goes (file, pipe,\n    log). Protect or delete that copy.")
	}
	fmt.Fprintln(w, "\nNext: 'rail0 keys add <name>' keeps it in the OS keychain; register the ADDRESS\n(never the key) via the merchant sign-up or an operator invite.")
}
