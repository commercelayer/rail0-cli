package cmd

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/config"
)

// The interactive create form must prompt only for the required fields not
// already supplied — that decision lives in fieldsToPrompt (pure), so it's tested
// here without a TTY. Mode is never prompted (it defaults to authorize; -C/--charge
// switches it).
func TestFieldsToPrompt(t *testing.T) {
	full := createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", amount: "1.00", chainID: 5042002}

	cases := []struct {
		name  string
		flags createPaymentFlags
		want  []string
	}{
		{
			name:  "nothing provided → all required, chain-id before token (never mode)",
			flags: createPaymentFlags{},
			want:  []string{"from", "to", "chain-id", "token", "amount"},
		},
		{
			name:  "all required provided → prompt nothing",
			flags: full,
			want:  nil,
		},
		{
			name:  "only amount missing → only amount",
			flags: createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", chainID: 5042002},
			want:  []string{"amount"},
		},
		{
			name:  "chainID 0 counts as missing",
			flags: createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", amount: "1.00", chainID: 0},
			want:  []string{"chain-id"},
		},
		{
			name:  "chain and token missing → chain-id asked before token",
			flags: createPaymentFlags{from: "0xF", to: "0xT", amount: "1.00"},
			want:  []string{"chain-id", "token"},
		},
		{
			name:  "order is stable across gaps: from and token missing",
			flags: createPaymentFlags{to: "0xT", amount: "1.00", chainID: 5042002},
			want:  []string{"from", "token"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.flags.fieldsToPrompt(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("fieldsToPrompt() = %v, want %v", got, tc.want)
			}
		})
	}
}

// validateAddress and validateAmount gate a value at entry — as a flag before any
// prompt, and inline in the huh form. The literal-address and amount branches are
// pure (no config/network), so they're tested here directly.
func TestValidateAddress(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"well-formed 0x address", "0x1234567890abcdef1234567890ABCDEF12345678", false},
		{"empty is rejected", "", true},
		{"too short", "0x1234", true},
		{"missing 0x prefix", "1234567890abcdef1234567890abcdef12345678", true},
		{"non-hex character", "0x1234567890abcdefg234567890abcdef123456789", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAddress(tc.in); (err != nil) != tc.wantErr {
				t.Errorf("validateAddress(%q) error = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"integer", "1", false},
		{"decimal", "1.00", false},
		{"leading zero decimal", "0.25", false},
		{"empty is rejected", "", true},
		{"zero", "0", true},
		{"zero with decimals", "0.00", true},
		{"negative", "-1.00", true},
		{"non-numeric", "abc", true},
		{"trailing token symbol", "1.00 USDC", true},
		// #61: the slips that used to reach the gateway from capture/refund, since
		// validateAmount was wired only into `create`.
		{"comma decimal separator", "1,50", true},
		{"scientific notation", "1e6", true},
		{"two decimal points", "1.2.3", true},
		{"whitespace only", " ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAmount(tc.in); (err != nil) != tc.wantErr {
				t.Errorf("validateAmount(%q) error = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

// acceptedMethods flattens discovered payee wallets to the distinct active
// (symbol, chain) pairs a merchant accepts, narrowed by any token/chain the
// buyer already supplied. Pure, so it's tested without a TTY or a gateway.
func TestAcceptedMethods(t *testing.T) {
	wallets := []rail0.WalletWithTokens{
		{Address: "0xw1", Tokens: []rail0.WalletTokenHolding{
			{Active: true, Token: rail0.Token{Symbol: "USDC", ChainID: 5042002}},
			{Active: true, Token: rail0.Token{Symbol: "EURC", ChainID: 5042002}},
			{Active: false, Token: rail0.Token{Symbol: "OLD", ChainID: 5042002}}, // inactive → dropped
		}},
		{Address: "0xw2", Tokens: []rail0.WalletTokenHolding{
			{Active: true, Token: rail0.Token{Symbol: "USDC", ChainID: 8453}},
			{Active: true, Token: rail0.Token{Symbol: "USDC", ChainID: 5042002}}, // dup of w1 → deduped
		}},
	}

	cases := []struct {
		name    string
		token   string
		chainID int
		want    []acceptedMethod
	}{
		{
			name: "no filter → distinct active pairs, first-seen order",
			want: []acceptedMethod{{"USDC", 5042002}, {"EURC", 5042002}, {"USDC", 8453}},
		},
		{
			name:  "filter by token (case-insensitive)",
			token: "usdc",
			want:  []acceptedMethod{{"USDC", 5042002}, {"USDC", 8453}},
		},
		{
			name:    "filter by chain",
			chainID: 5042002,
			want:    []acceptedMethod{{"USDC", 5042002}, {"EURC", 5042002}},
		},
		{
			name:    "filter by both narrows to one",
			token:   "EURC",
			chainID: 5042002,
			want:    []acceptedMethod{{"EURC", 5042002}},
		},
		{
			name:  "no match → empty",
			token: "DAI",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := acceptedMethods(wallets, tc.token, tc.chainID); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("acceptedMethods(%q, %d) = %v, want %v", tc.token, tc.chainID, got, tc.want)
			}
		})
	}
}

// build() must reject a payment whose payer and payee are the same address (a
// payment to oneself), compared case-insensitively. Exercised via --payment-file
// so the check runs without touching the network (token resolution).
func TestBuildRejectsSelfPayment(t *testing.T) {
	dir := t.TempDir()
	write := func(payer, payee string) string {
		p := filepath.Join(dir, "pay.json")
		body := `{"chain_id":5042002,"mode":"authorize","amount":"1.00",` +
			`"token":"0x3600000000000000000000000000000000000000",` +
			`"payer":"` + payer + `","payee":"` + payee + `"}`
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	self := "0x" + strings.Repeat("ab", 20)
	other := "0x" + strings.Repeat("cd", 20)

	// payer == payee (upper-cased → still equal, case-insensitively) → rejected.
	f := createPaymentFlags{paymentFile: write(self, strings.ToUpper(self))}
	if _, err := f.build(); err == nil {
		t.Fatal("expected an error when payer == payee, got nil")
	}

	// distinct payer/payee → accepted.
	f = createPaymentFlags{paymentFile: write(self, other)}
	if _, err := f.build(); err != nil {
		t.Fatalf("unexpected error for distinct payer/payee: %v", err)
	}
}

// An @alias in a payment file passes validation, so it must also be RESOLVED: the
// request used to go out with the literal "@buyer", and the signer check then said
// the key "does not match the payer @buyer" — which reads like a wrong key.
func TestBuildResolvesAliasesInAPaymentFile(t *testing.T) {
	isolate(t)
	buyer := "0x" + strings.Repeat("ab", 20)
	merchant := "0x" + strings.Repeat("cd", 20)
	cfg := &config.Config{Aliases: map[string]string{"buyer": buyer, "merchant": merchant}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save aliases: %v", err)
	}
	p := filepath.Join(t.TempDir(), "pay.json")
	body := `{"chain_id":5042002,"mode":"authorize","amount":"1.00",` +
		`"token":"0x3600000000000000000000000000000000000000","payer":"@buyer","payee":"@merchant"}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	req, err := (&createPaymentFlags{paymentFile: p}).build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if string(req.Payer) != buyer || string(req.Payee) != merchant {
		t.Errorf("payer/payee = %s/%s, want the aliases resolved to %s/%s", req.Payer, req.Payee, buyer, merchant)
	}
}

// assertAmountPrecision rejects an amount with more decimals than the token
// supports (the gateway would 422); no decimals or within the limit passes.
func TestAssertAmountPrecision(t *testing.T) {
	cases := []struct {
		amount   string
		decimals int
		wantErr  bool
	}{
		{"10", 6, false},
		{"10.5", 6, false},
		{"10.123456", 6, false}, // exactly at the limit
		{"10.1234567", 6, true}, // one too many
		{"1.00", 0, true},       // a 0-decimal token rejects any fraction
		{"1", 0, false},
		{" 2.50 ", 2, false}, // trimmed
	}
	for _, tc := range cases {
		if err := assertAmountPrecision(tc.amount, tc.decimals); (err != nil) != tc.wantErr {
			t.Errorf("assertAmountPrecision(%q, %d) error = %v, wantErr = %v", tc.amount, tc.decimals, err, tc.wantErr)
		}
	}
}

// resetTokenCatalog empties the process-wide token catalogue for one test and again
// after it, so no test sees another's tokens.
func resetTokenCatalog(t *testing.T) {
	t.Helper()
	reset := func() {
		tokenCatalogMu.Lock()
		tokenCatalogAll = nil
		tokenCatalogMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
}

// An interactive create consulted the token list three to four times for the SAME
// chain — assertTokenKnown, the huh validator (re-run on every submit attempt),
// resolve, and the picker — each on a fresh client, so the form stalled between
// prompts (#69). activeTokens serves from the per-process catalogue.
func TestActiveTokensServesFromCacheWithoutRefetching(t *testing.T) {
	resetTokenCatalog(t)

	// Point the client at a port nothing is listening on: any real fetch fails.
	t.Setenv("RAIL0_BASE_URL", "http://127.0.0.1:1")

	// Cold: no catalogue, so it must actually try — and fail.
	if _, err := activeTokens(context.Background(), 84532); err == nil {
		t.Fatal("expected a fetch error with no catalogue and no reachable gateway")
	}

	// A failure must NOT be cached — unlike chainCatalog, an empty token list would
	// leave the user unable to recover inside the same command.
	tokenCatalogMu.Lock()
	cached := tokenCatalogAll != nil
	tokenCatalogMu.Unlock()
	if cached {
		t.Error("a failed fetch must stay retryable, not be cached as an empty list")
	}

	// Warm: seed the catalogue and assert it is served without touching the network
	// (which is still unreachable, so a refetch would error) — for EVERY chain, and
	// filtered to that chain's ACTIVE tokens, as the gateway's ?chain_id&active=true was.
	tokenCatalogMu.Lock()
	tokenCatalogAll = []rail0.Token{
		{Symbol: "USDC", Decimals: 6, ChainID: 84532, Active: true},
		{Symbol: "EURC", Decimals: 6, ChainID: 84532, Active: false},
		{Symbol: "USDC", Decimals: 6, ChainID: 11155420, Active: true},
	}
	tokenCatalogMu.Unlock()

	got, err := activeTokens(context.Background(), 84532)
	if err != nil {
		t.Fatalf("cached call must not hit the network: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "USDC" || got[0].ChainID != 84532 {
		t.Errorf("got %+v, want only Base Sepolia's active USDC", got)
	}
	if got, err := activeTokens(context.Background(), 11155420); err != nil || len(got) != 1 {
		t.Errorf("another chain: got %+v, %v; want its token from the same catalogue, no fetch", got, err)
	}
}

// One GET /tokens per process answers both questions a create asks — "is this an active
// token on the chain?" and "what are its decimals?" — and every chain a list renders.
// It used to be one ?chain_id&active=true call plus one ?chain_id call per chain.
func TestTokenCatalogIsFetchedOnceForEveryChainAndBothPaths(t *testing.T) {
	resetTokenCatalog(t)
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if r.URL.Path == "/tokens" {
			return []any{
				map[string]any{"symbol": "USDC", "address": "0xAAaa", "chain_id": 84532, "decimals": 6, "active": true},
				map[string]any{"symbol": "OLD", "address": "0xbbbb", "chain_id": 84532, "decimals": 18, "active": false},
				map[string]any{"symbol": "USDC", "address": "0xcccc", "chain_id": 11155420, "decimals": 6, "active": true},
			}
		}
		return nil
	}

	ctx := context.Background()
	if got, err := activeTokens(ctx, 84532); err != nil || len(got) != 1 {
		t.Fatalf("activeTokens = %+v, %v; want the one active token", got, err)
	}
	if d, ok := tokenDecimals(ctx, 84532, "0xaaAA"); !ok || d != 6 {
		t.Errorf("decimals of the active token = %d, %v; want 6", d, ok)
	}
	// Retired tokens stay resolvable for rendering: a settled payment keeps its token.
	if d, ok := tokenDecimals(ctx, 84532, "0xBBBB"); !ok || d != 18 {
		t.Errorf("decimals of the retired token = %d, %v; want 18", d, ok)
	}
	if d, ok := tokenDecimals(ctx, 11155420, "0xcccc"); !ok || d != 6 {
		t.Errorf("decimals on another chain = %d, %v; want 6", d, ok)
	}
	// An address is unique only per chain.
	if _, ok := tokenDecimals(ctx, 11155420, "0xaaaa"); ok {
		t.Error("a token must not resolve on a chain it is not on")
	}

	var calls []recorded
	for _, r := range g.reqs {
		if r.path == "/tokens" {
			calls = append(calls, r)
		}
	}
	if len(calls) != 1 || len(calls[0].query) != 0 {
		t.Errorf("GET /tokens calls = %+v; want exactly one, unfiltered", calls)
	}
}
