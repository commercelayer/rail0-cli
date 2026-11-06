package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

// testChains is a testnet gateway's catalog: one chain per slug.
var testChains = []rail0.Blockchain{
	{ChainID: 5042002, Name: "Arc Testnet", Slug: "arc", NativeSymbol: "USDC"},
	{ChainID: 84532, Name: "Base Sepolia", Slug: "base", NativeSymbol: "ETH"},
}

// newChainFlagCmd builds a throwaway command carrying the --chain/--chain-id pair,
// parses args into it and returns the target the command would read.
func newChainFlagCmd(t *testing.T, args ...string) (*cobra.Command, *int, error) {
	t.Helper()
	var id int
	c := &cobra.Command{Use: "x"}
	addChainFlag(c, &id, "c", "Chain")
	err := c.ParseFlags(args)
	return c, &id, err
}

func TestChainFlagNumericNeedsNoLookup(t *testing.T) {
	// No catalog seeded and no gateway: a number must not need either, and an id
	// the catalog does not list keeps working as --chain-id always did.
	for _, args := range [][]string{{"--chain", "999999"}, {"-c", "999999"}, {"--chain-id", "999999"}} {
		c, id, err := newChainFlagCmd(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if *id != 999999 {
			t.Errorf("%v: target = %d before resolution, want 999999", args, *id)
		}
		if err := resolveChainFlags(c); err != nil || *id != 999999 {
			t.Errorf("%v: resolve = %d, %v", args, *id, err)
		}
	}
}

func TestChainFlagSlugResolvesAgainstTheCatalog(t *testing.T) {
	seedChainCatalog(t, testChains...)
	c, id, err := newChainFlagCmd(t, "--chain", "ARC")
	if err != nil {
		t.Fatal(err)
	}
	if *id != 0 {
		t.Errorf("a slug must wait for resolution, target = %d", *id)
	}
	if err := resolveChainFlags(c); err != nil || *id != 5042002 {
		t.Errorf("resolve = %d, %v; want 5042002", *id, err)
	}
}

func TestChainFlagUnknownSlugListsTheAvailableOnes(t *testing.T) {
	seedChainCatalog(t, testChains...)
	c, _, _ := newChainFlagCmd(t, "--chain", "solana")
	err := resolveChainFlags(c)
	if err == nil || !isUsageError(err) {
		t.Fatalf("want a usage error, got %v", err)
	}
	for _, want := range []string{"solana", "arc (5042002)", "base (84532)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestChainFlagBothMustAgree(t *testing.T) {
	seedChainCatalog(t, testChains...)
	c, id, _ := newChainFlagCmd(t, "--chain", "arc", "--chain-id", "5042002")
	if err := resolveChainFlags(c); err != nil || *id != 5042002 {
		t.Errorf("agreeing pair: %d, %v", *id, err)
	}
	c, _, _ = newChainFlagCmd(t, "--chain", "base", "--chain-id", "5042002")
	if err := resolveChainFlags(c); err == nil || !isUsageError(err) || !strings.Contains(err.Error(), "different chains") {
		t.Errorf("conflicting pair must be a usage error, got %v", err)
	}
}

func TestChainFlagRejectsMalformedValues(t *testing.T) {
	for _, args := range [][]string{{"--chain", "0"}, {"--chain", "-5"}, {"--chain", "arc testnet"}, {"--chain-id", "arc"}} {
		if _, _, err := newChainFlagCmd(t, args...); err == nil {
			t.Errorf("%v: want a parse error", args)
		}
	}
}

func TestChainFlagUnreachableGatewayIsReported(t *testing.T) {
	chainCatalogOnce.Do(func() {})
	prevList, prevErr := chainCatalogList, chainCatalogErr
	chainCatalogList, chainCatalogErr = nil, errors.New("dial tcp: connection refused")
	t.Cleanup(func() { chainCatalogList, chainCatalogErr = prevList, prevErr })

	_, err := resolveChainRef(context.Background(), "arc")
	if err == nil || !strings.Contains(err.Error(), "cannot fetch the chain list") || !strings.Contains(err.Error(), "numeric chain id") {
		t.Errorf("want an unreachable-gateway error, got %v", err)
	}
	if id, err := resolveChainRef(context.Background(), "5042002"); err != nil || id != 5042002 {
		t.Errorf("a numeric id must not need the gateway: %d, %v", id, err)
	}
}

func TestChainFlagRegisteredWithHiddenLegacyAlias(t *testing.T) {
	c, _, _ := newChainFlagCmd(t)
	if f := c.Flags().Lookup("chain"); f == nil || f.Shorthand != "c" {
		t.Fatalf("--chain/-c missing: %+v", f)
	}
	if f := c.Flags().Lookup("chain-id"); f == nil || !f.Hidden {
		t.Errorf("--chain-id must stay accepted, hidden: %+v", f)
	}
}

// Every command that took --chain-id now takes --chain, with --chain-id still accepted.
func TestEveryChainCommandTakesChain(t *testing.T) {
	for _, c := range []*cobra.Command{
		paymentsCreateCmd, paymentsListCmd, tokensCmd, walletsCmd, walletsBalancesCmd,
		walletTokensAddCmd, walletTokensEnableCmd, walletTokensDisableCmd,
		analyticsSummaryCmd, analyticsTimeseriesCmd, analyticsBreakdownCmd,
		payoutDestinationsAddCmd, keysBalanceCmd,
	} {
		if c.Flags().Lookup("chain") == nil || c.Flags().Lookup("chain-id") == nil {
			t.Errorf("%s: --chain and --chain-id must both be registered", c.CommandPath())
		}
	}
}

func TestConfiguredChainResolvesASlugAndReadsTheLegacyKey(t *testing.T) {
	clearConfigEnv(t)
	seedChainCatalog(t, testChains...)

	if _, _, ok, err := configuredChainID(context.Background()); ok || err != nil {
		t.Fatalf("nothing set: ok=%v err=%v", ok, err)
	}

	// A config written before slugs: chain-id alone.
	if err := (&config.Config{Settings: map[string]string{"chain-id": "84532"}}).Save(); err != nil {
		t.Fatal(err)
	}
	if id, origin, ok, err := configuredChainID(context.Background()); !ok || err != nil || id != 84532 || origin != "config file" {
		t.Errorf("legacy chain-id: %d %q %v %v", id, origin, ok, err)
	}

	// chain wins over chain-id in the same place, and a slug is resolved.
	if err := (&config.Config{Settings: map[string]string{"chain": "arc", "chain-id": "84532"}}).Save(); err != nil {
		t.Fatal(err)
	}
	if id, _, _, err := configuredChainID(context.Background()); err != nil || id != 5042002 {
		t.Errorf("chain arc: %d %v", id, err)
	}

	// ...but an exported RAIL0_CHAIN_ID outranks the stored chain, as env outranks file.
	t.Setenv("RAIL0_CHAIN_ID", "84532")
	if id, origin, _, err := configuredChainID(context.Background()); err != nil || id != 84532 || !strings.Contains(origin, "RAIL0_CHAIN_ID") {
		t.Errorf("env chain-id over file chain: %d %q %v", id, origin, err)
	}

	// An unknown stored slug is an error naming where it came from, not a silent "required".
	t.Setenv("RAIL0_CHAIN_ID", "")
	t.Setenv("RAIL0_CHAIN", "solana")
	if _, _, _, err := configuredChainID(context.Background()); err == nil || !strings.Contains(err.Error(), "RAIL0_CHAIN") {
		t.Errorf("unknown default slug: %v", err)
	}
}

func TestValidateChainRef(t *testing.T) {
	for _, ok := range []string{"arc", "Base", "5042002", "arbitrum-one"} {
		if err := validateChainRef(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "0", "-1", "arc testnet", "1.5"} {
		if err := validateChainRef(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}
