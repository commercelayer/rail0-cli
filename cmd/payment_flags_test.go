package cmd

import (
	"reflect"
	"testing"

	rail0 "github.com/rail0/go-sdk"
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
			name:  "nothing provided → all required, in order (never mode)",
			flags: createPaymentFlags{},
			want:  []string{"from", "to", "token", "amount", "chain-id"},
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
