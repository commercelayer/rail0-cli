package cmd

import (
	"encoding/json"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

// A chain is labelled "Name (id)" when the catalog knows it, and degrades to
// "chain <id>" when it doesn't (unreachable gateway, or an id outside this
// environment's catalog).
func TestFormatChainLabel(t *testing.T) {
	cases := []struct {
		name string
		id   int
		want string
	}{
		{"Base Sepolia", 84532, "Base Sepolia (84532)"},
		{"", 84532, "chain 84532"},
	}
	for _, tc := range cases {
		if got := formatChainLabel(tc.name, tc.id); got != tc.want {
			t.Errorf("formatChainLabel(%q, %d) = %q, want %q", tc.name, tc.id, got, tc.want)
		}
	}
}

// `rail0 blockchains` is an alias of `rail0 chains`.
func TestChainsHasBlockchainsAlias(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"blockchains"})
	if err != nil || sub == nil || sub.Name() != "chains" {
		t.Fatalf("`blockchains` should resolve to the chains command, got %v (%v)", sub, err)
	}
}

// Settlement in words: both percentiles when measured, "n/a" with the sample count
// while they are null, so "not measured yet" never reads as "instant".
func TestSettlementLabel(t *testing.T) {
	n := func(v int) *int { return &v }
	cases := []struct {
		st   rail0.BlockchainSettlement
		want string
	}{
		{rail0.BlockchainSettlement{P50Seconds: n(11), P90Seconds: n(19), SampleSize: 20, WindowDays: 7}, "p50 11s · p90 19s (20 samples, 7d)"},
		{rail0.BlockchainSettlement{P50Seconds: n(95), P90Seconds: n(660), SampleSize: 812}, "p50 1m35s · p90 11m0s (812 samples)"},
		{rail0.BlockchainSettlement{SampleSize: 3, WindowDays: 7}, "n/a (3 samples)"},
		{rail0.BlockchainSettlement{}, "n/a (0 samples)"},
	}
	for _, tc := range cases {
		if got := settlementLabel(tc.st); got != tc.want {
			t.Errorf("settlementLabel(%+v) = %q, want %q", tc.st, got, tc.want)
		}
	}
}

// The view keeps the gateway's chain intact — `settlement` included — and adds one key.
func TestChainViewKeepsTheRawChain(t *testing.T) {
	p90 := 19
	b, err := json.Marshal(chainView{
		Blockchain:      rail0.Blockchain{ChainID: 5042002, Name: "Arc Testnet", Settlement: rail0.BlockchainSettlement{P90Seconds: &p90, SampleSize: 20}},
		SettlementHuman: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	st, _ := out["settlement"].(map[string]any)
	if out["chain_id"] != float64(5042002) || st["p90_seconds"] != float64(19) || out["settlement_human"] != "x" {
		t.Errorf("chain view JSON = %s", b)
	}
}
