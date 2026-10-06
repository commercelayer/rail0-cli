package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

func TestWalletsCommandTree(t *testing.T) {
	// `wallets` is top-level; bare it lists (account from the session), verbs
	// hang off it and take no account_id.
	if walletsCmd.RunE == nil {
		t.Error("wallets should list when run bare (nil RunE)")
	}
	if err := walletsCmd.Args(walletsCmd, []string{}); err != nil {
		t.Errorf("wallets list should take no args, got %v", err)
	}

	for _, verb := range []string{"get", "create", "update", "delete", "balances"} {
		sub, _, err := walletsCmd.Find([]string{verb})
		if err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("wallets %s: not found (%v)", verb, err)
		}
	}

	// get/update/delete/balances take exactly one positional (the wallet),
	// no account id.
	for _, verb := range []string{"get", "update", "delete", "balances"} {
		sub, _, _ := walletsCmd.Find([]string{verb})
		if err := sub.Args(sub, []string{"0xabc"}); err != nil {
			t.Errorf("wallets %s should accept 1 arg, got %v", verb, err)
		}
		if err := sub.Args(sub, []string{"a", "b"}); err == nil {
			t.Errorf("wallets %s should reject 2 args", verb)
		}
	}

	// `accounts` is gone entirely.
	if sub, _, _ := rootCmd.Find([]string{"accounts"}); sub != nil && sub.Name() == "accounts" {
		t.Error("top-level 'accounts' should no longer exist; use 'wallets'")
	}
}

func TestWalletsSymbolHasTokenAlias(t *testing.T) {
	// --token is a hidden alias of --symbol on the list and balances commands.
	if walletsCmd.Flags().Lookup("token") == nil {
		t.Error("wallets: --token should alias --symbol")
	}
	balances, _, _ := walletsCmd.Find([]string{"balances"})
	if balances == nil || balances.Flags().Lookup("token") == nil {
		t.Error("wallets balances: --token should alias --symbol")
	}
}

func TestWalletsCreateHasSigningKeyFlags(t *testing.T) {
	// Adding a wallet now needs the key OF the address being added to build the
	// SIWE proof-of-ownership, resolved like every signing command: --private-key
	// (-p), a hidden --pk alias, and --address (required, accepts an @alias).
	create, _, err := walletsCmd.Find([]string{"create"})
	if err != nil || create == nil {
		t.Fatalf("wallets create: not found (%v)", err)
	}
	pk := create.Flags().Lookup("private-key")
	if pk == nil {
		t.Fatal("wallets create: --private-key flag missing")
	}
	if pk.Shorthand != "p" {
		t.Errorf("wallets create: --private-key shorthand = %q, want p", pk.Shorthand)
	}
	if create.Flags().Lookup("pk") == nil {
		t.Error("wallets create: --pk should alias --private-key")
	}
	addr := create.Flags().Lookup("address")
	if addr == nil {
		t.Fatal("wallets create: --address flag missing")
	}
	if ann := addr.Annotations[cobra.BashCompOneRequiredFlag]; len(ann) != 1 || ann[0] != "true" {
		t.Error("wallets create: --address should be required")
	}
}

// Deactivation is one operation with two spellings — `wallets delete <w>` and
// `wallets update <w> --active false` — and the help says so. So they must ask the same
// question. Only the delete spelling used to ask, while the flag spelling (the one the
// help points at, and the one a script reaches for) went straight through.
//
// The prompt has to name the WEBHOOK consequence, because it is the one nothing else
// surfaces: dispatch resolves the payee wallet's account, so a deactivated wallet stops
// delivering events for payments still in flight, records no skip, and the payments settle
// anyway. (rail0-gateway#237)
func TestDeactivationPromptNamesWhatIsLost(t *testing.T) {
	prompt := deactivationPrompt("0xABCD")

	for _, want := range []string{
		"0xABCD",          // which wallet
		"webhooks STOP",   // the consequence nothing else surfaces
		"still in flight", // and precisely which payments it hits
		"--active true",   // and that it is reversible
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt must mention %q\ngot: %s", want, prompt)
		}
	}
}

// Structural pin: the flag spelling must carry -y/--yes. It only needs one because it is
// gated — so the flag's presence is the cheapest evidence the gate was wired, and its
// absence would mean the new confirmation refuses in every non-interactive shell and
// breaks any script that turns a wallet off.
func TestUpdateCarriesTheYesFlag(t *testing.T) {
	for _, c := range []*cobra.Command{walletsUpdateCmd, walletsDeleteCmd} {
		if c.Flags().Lookup("yes") == nil {
			t.Errorf("%q must accept --yes: it gates on a confirmation", c.Name())
		}
	}
}

// Both spellings gate on the same prompt text, so neither can drift into asking a softer
// question than the other.
func TestBothDeactivationSpellingsAreGated(t *testing.T) {
	// --yes off and no TTY: confirmDestructive refuses, which is what proves the gate is
	// actually reached rather than skipped.
	assumeYes = false
	t.Cleanup(func() { assumeYes = false })

	if err := confirmDestructive(deactivationPrompt("0x1")); err == nil {
		t.Fatal("the gate must refuse without a TTY or --yes")
	}

	// And --yes passes it, so a script can still turn a wallet off deliberately.
	assumeYes = true
	if err := confirmDestructive(deactivationPrompt("0x1")); err != nil {
		t.Errorf("--yes must pass the gate: %v", err)
	}
}

// A new wallet accepts no token and so cannot be paid, and the create response says
// nothing about it: the hint names the consequence and the exact next command.
func TestWalletCreatedHintNamesTheNextStep(t *testing.T) {
	addr := "0xA849c2b07fFc16195B7FB7A95150014bE6b3a48c"
	hint := walletCreatedHint(addr)
	for _, want := range []string{"can't receive payments", "rail0 wallets tokens add " + addr, "--default"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q lacks %q", hint, want)
		}
	}
}

// Every verb that takes a <wallet> positional accepts an @alias, through the same
// resolver as the address flags. They used to pass "@payee" to the gateway verbatim,
// which answered 404 — so 'wallets balances @payee' failed while the 0x address worked.
func TestWalletPositionalResolvesAlias(t *testing.T) {
	g := newRecordingGateway(t)
	const shop = "0x00000000000000000000000000000000000000aa"
	g.respond = func(r *http.Request) any {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/wallets") {
			return []any{map[string]any{
				"id": "wal-1", "address": shop,
				"tokens": []any{map[string]any{"token_id": "tok-1", "active": false}},
			}}
		}
		return map[string]any{}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Aliases = map[string]string{"shop": shop}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	assumeYes = true
	t.Cleanup(func() { assumeYes, walletTokenToken, updateWalletLabel = false, "", "" })

	walletPath := "/accounts/acc-1/wallets/" + shop
	run(t, walletsGetCmd, "@shop")
	g.last(t, http.MethodGet, walletPath)

	run(t, walletsBalancesCmd, "@shop")
	g.last(t, http.MethodGet, walletPath+"/balances")

	updateWalletLabel = "Shop"
	run(t, walletsUpdateCmd, "@shop")
	g.last(t, http.MethodPatch, walletPath)

	run(t, walletsDeleteCmd, "@shop")
	g.last(t, http.MethodDelete, walletPath)

	walletTokenChainID, walletTokenToken = 5042002, "0x3600000000000000000000000000000000000000"
	t.Cleanup(func() { walletTokenChainID = 0 })
	run(t, walletTokensAddCmd, "@shop")
	g.last(t, http.MethodPost, walletPath+"/tokens")

	// enable/disable match the holding by the wallet's address, so an unresolved
	// alias failed locally as "wallet @shop not found" instead.
	walletTokenChainID = 0
	run(t, walletTokensEnableCmd, "@shop", "tok-1")
	g.last(t, http.MethodPatch, walletPath+"/tokens/tok-1/enable")
	run(t, walletTokensDisableCmd, "@shop", "tok-1")
	g.last(t, http.MethodPatch, walletPath+"/tokens/tok-1/disable")

	// An unknown alias fails locally, before any request is made.
	before := len(g.reqs)
	if err := walletsBalancesCmd.RunE(walletsBalancesCmd, []string{"@nope"}); err == nil || !strings.Contains(err.Error(), "unknown alias") {
		t.Errorf("unknown alias error = %v, want an 'unknown alias' error", err)
	}
	if len(g.reqs) != before {
		t.Error("an unknown <wallet> alias still reached the gateway")
	}
}
