package ui

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSortFields(t *testing.T) {
	in := []string{"updated_at", "status", "address", "active", "name", "id", "created_at", "label"}
	sortFields(in)
	// Identity/name first (priority order), then the rest alphabetically, with
	// timestamps (…_at) always trailing — note "status" comes before the dates
	// even though it sorts after "created_at" alphabetically.
	want := []string{"id", "name", "label", "address", "active", "status", "created_at", "updated_at"}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, in[i], want[i], in)
		}
	}
}

func TestPrintStyled(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			"chains (array of objects)",
			`[{"chain_id":44787,"name":"Celo Sepolia","native_symbol":"CELO","network_type":"testnet","explorer_url":"https://celo-alfajores.blockscout.com"},{"chain_id":5042002,"name":"Arc Testnet","native_symbol":"ETH","network_type":"testnet","explorer_url":""}]`,
		},
		{
			"tokens (array)",
			`[{"chain_id":5042002,"symbol":"USDC","address":"0x3600000000000000000000000000000000000000","decimals":6}]`,
		},
		{
			"payment (single object)",
			`{"rail0_id":"0xabc123","status":"authorized","amount":"1000000","payer":"0xBuyer","payee":"0xMerchant"}`,
		},
		{
			"transactions (array with status)",
			`[{"operation":"authorize","status":"confirmed","transaction_hash":"0xaaa"},{"operation":"capture","status":"submitted","transaction_hash":"0xbbb"}]`,
		},
		{
			"wallets (array; each with a tokens sub-table)",
			`[{"id":"019ef568","address":"0xe3dd","active":true,"created_at":"2026-06-23T16:55:15Z","tokens":[{"chain_id":5042002,"symbol":"USDC","address":"0x3600","decimals":6,"active":true,"default":false},{"chain_id":5042002,"symbol":"EURC","address":"0x89B5","decimals":6,"active":true,"default":false}]},{"id":"019ef569","address":"0x2d2b","active":true,"created_at":"2026-06-23T16:55:15Z"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := json.Unmarshal([]byte(tc.raw), &v); err != nil {
				t.Fatal(err)
			}
			fmt.Printf("\n── %s ──\n", tc.name)
			if err := PrintStyled(v); err != nil {
				t.Errorf("PrintStyled error: %v", err)
			}
		})
	}
}
