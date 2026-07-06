package cmd

import (
	"reflect"
	"testing"
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
