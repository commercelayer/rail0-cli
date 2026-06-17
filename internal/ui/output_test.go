package ui

import (
	"encoding/json"
	"fmt"
	"testing"
)

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
