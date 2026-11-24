package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"

	"github.com/rail0/cli/internal/ui"
)

func row(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// An amount whose scale is unknown is never shown bare beside decimal ones.
func TestAmountCell(t *testing.T) {
	if got := amountCell(row(t, `{"amount":"2500000","amount_human":"2.5"}`)); got != "2.5" {
		t.Errorf("got %q", got)
	}
	if got := amountCell(row(t, `{"amount":"2500000"}`)); got != "2500000 (base units)" {
		t.Errorf("got %q", got)
	}
	if got := amountCell(row(t, `{"operation":"void"}`)); got != "" {
		t.Errorf("no amount should be an empty cell, got %q", got)
	}
}

// The token column reads the cached catalogue; a token it does not list falls back to
// the shortened address, never to an empty cell.
func TestTokenCell(t *testing.T) {
	tokenCatalogMu.Lock()
	old := tokenCatalogAll
	tokenCatalogAll = []rail0.Token{{ChainID: 5042002, Address: "0x3600000000000000000000000000000000000000", Symbol: "USDC", Decimals: 6}}
	tokenCatalogMu.Unlock()
	t.Cleanup(func() {
		tokenCatalogMu.Lock()
		tokenCatalogAll = old
		tokenCatalogMu.Unlock()
	})

	if got := tokenCell(row(t, `{"chain_id":5042002,"token":"0x3600000000000000000000000000000000000000"}`)); got != "USDC" {
		t.Errorf("got %q", got)
	}
	if got := tokenCell(row(t, `{"chain_id":1,"token":"0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238"}`)); got != "0x1c7D…7238" {
		t.Errorf("got %q", got)
	}
}

func TestWalletTokensCell(t *testing.T) {
	w := row(t, `{"tokens":[
		{"active":true,"token":{"symbol":"USDC","chain_id":5042002}},
		{"active":true,"token":{"symbol":"USDC","chain_id":11155111}},
		{"active":false,"token":{"symbol":"PYUSD"}},
		{"active":true,"token":{"symbol":"EURC"}}]}`)
	if got := walletTokensCell(w); got != "USDC, EURC" {
		t.Errorf("got %q", got)
	}
	if got := walletTokensCell(row(t, `{}`)); got != "" {
		t.Errorf("no holdings should be an empty cell, got %q", got)
	}
}

// Every column set is well formed: a header for each column, no header twice, and a
// computed column says what it is called.
func TestColumnSetsAreWellFormed(t *testing.T) {
	sets := map[string][]ui.Column{
		"payments": paymentColumns, "transactions": transactionColumns, "history": historyColumns,
		"disputes": disputeColumns, "account disputes": accountDisputeColumns, "wallets": walletColumns,
		"webhooks": webhookColumns, "event callbacks": eventCallbackColumns, "chains": chainColumns,
	}
	for name, cols := range sets {
		seen := map[string]bool{}
		for i, c := range cols {
			title := strings.ToUpper(c.Header)
			if title == "" {
				title = strings.ToUpper(c.Field)
			}
			if title == "" {
				t.Errorf("%s: column %d has neither a header nor a field", name, i)
			}
			if c.Value == nil && c.Field == "" {
				t.Errorf("%s: column %q has nothing to show", name, title)
			}
			if seen[title] {
				t.Errorf("%s: header %q twice", name, title)
			}
			seen[title] = true
		}
	}
	// The account-wide list adds the payment and must not have changed the one-payment set.
	if accountDisputeColumns[0].Field != "payment_id" || disputeColumns[0].Field != "status" {
		t.Errorf("dispute column sets are entangled: %v / %v", accountDisputeColumns[0], disputeColumns[0])
	}
}

// The columns never reach a script: the list a command returns marshals as its rows.
func TestTableMarshalsAsTheRows(t *testing.T) {
	rows := []map[string]any{{"id": "p1", "token": "0x36"}}
	b, err := json.Marshal(paged(table(rows, paymentColumns), rail0.PageMeta{Page: 1, PerPage: 25, Total: 1, TotalPages: 1}).(ui.Paged).Items)
	if err != nil || string(b) != `[{"id":"p1","token":"0x36"}]` {
		t.Errorf("got %s (%v)", b, err)
	}
}
