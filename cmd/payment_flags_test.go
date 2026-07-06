package cmd

import (
	"reflect"
	"testing"
)

// The interactive create form must prompt only for what wasn't already supplied
// on the command line — that decision lives in fieldsToPrompt (pure), so it's
// tested here without a TTY.
func TestFieldsToPrompt(t *testing.T) {
	full := createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", amount: "1.00", chainID: 5042002}

	cases := []struct {
		name         string
		flags        createPaymentFlags
		modeProvided bool
		want         []string
	}{
		{
			name:         "nothing provided → all required + mode, in order",
			flags:        createPaymentFlags{},
			modeProvided: false,
			want:         []string{"from", "to", "token", "amount", "chain-id", "mode"},
		},
		{
			name:         "all required provided and --mode set → prompt nothing",
			flags:        full,
			modeProvided: true,
			want:         nil,
		},
		{
			name:         "only amount missing → only amount",
			flags:        createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", chainID: 5042002},
			modeProvided: true,
			want:         []string{"amount"},
		},
		{
			name:         "mode not passed is prompted even when required are complete",
			flags:        full,
			modeProvided: false,
			want:         []string{"mode"},
		},
		{
			name:         "chainID 0 counts as missing",
			flags:        createPaymentFlags{from: "0xF", to: "0xT", token: "USDC", amount: "1.00", chainID: 0},
			modeProvided: true,
			want:         []string{"chain-id"},
		},
		{
			name:         "order is stable across gaps: from and token missing",
			flags:        createPaymentFlags{to: "0xT", amount: "1.00", chainID: 5042002},
			modeProvided: true,
			want:         []string{"from", "token"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.flags.fieldsToPrompt(tc.modeProvided); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("fieldsToPrompt(%v) = %v, want %v", tc.modeProvided, got, tc.want)
			}
		})
	}
}
