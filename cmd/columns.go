package cmd

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/rail0/cli/internal/ui"
)

// The columns each list shows in the --pretty table.
//
// A list used to get one column per field: thirty for a payment, with its nested
// objects printed as JSON inside a cell, on a row several screens wide. These are the
// few a person scans a list for, most important first — the table drops columns from
// the right when the terminal is narrow. Everything else is in the record's own view
// ('get') and in --json, which these columns never touch: they exist in the --pretty
// view only.

// table gives a list the columns its --pretty view shows (ui.Table). It wraps the rows
// before paged / wholeList, so the result is still the same list in every other mode.
func table(rows any, cols []ui.Column) any { return ui.Table{Rows: rows, Columns: cols} }

// str reads a string field of a decoded row, "" when it is absent or not a string.
func str(row map[string]any, key string) string {
	s, _ := row[key].(string)
	return s
}

// num reads an integer field of a decoded row (JSON numbers decode as float64), 0 when
// it is absent.
func num(row map[string]any, key string) int {
	f, _ := row[key].(float64)
	return int(f)
}

// amountCell is a row's amount in token units, or — when the scale is unknown and there
// is no amount_human — the base units marked as such, for the reason in humanize.go: a
// bare 4050000 beside a 4.05 reads as four million.
func amountCell(row map[string]any) string {
	if h := str(row, "amount_human"); h != "" {
		return h
	}
	if base := str(row, "amount"); base != "" {
		return base + " (base units)"
	}
	return ""
}

func shortHexOf(key string) func(map[string]any) string {
	return func(row map[string]any) string { return ui.ShortHex(str(row, key)) }
}

func shortTimeOf(key string) func(map[string]any) string {
	return func(row map[string]any) string { return ui.ShortTime(str(row, key)) }
}

// tokenCell is a payment's token by symbol, from the cached token catalogue (one request
// for the whole table, made only when the table is drawn), or its shortened address
// when the catalogue does not list it or cannot be read.
func tokenCell(row map[string]any) string {
	address := str(row, "token")
	if symbol := tokenSymbol(context.Background(), num(row, "chain_id"), address); symbol != "" {
		return symbol
	}
	return ui.ShortHex(address)
}

// chainCell is a row's chain by slug — what --chain takes — from the cached chain
// catalogue, or its id when the catalogue does not list it.
func chainCell(row map[string]any) string {
	id := num(row, "chain_id")
	if id == 0 {
		return ""
	}
	for _, c := range chainCatalog() {
		if c.ChainID == id && c.Slug != "" {
			return c.Slug
		}
	}
	return intOrEmpty(id)
}

var paymentColumns = []ui.Column{
	{Field: "id"},
	{Field: "status"},
	{Header: "amount", Value: amountCell, Right: true},
	{Header: "token", Value: tokenCell},
	{Header: "payer", Value: shortHexOf("payer")},
	{Header: "payee", Value: shortHexOf("payee")},
	{Header: "created", Value: shortTimeOf("created_at")},
	{Header: "chain", Value: chainCell},
	{Field: "description", Max: 32},
}

var transactionColumns = []ui.Column{
	{Field: "id"},
	{Field: "operation"},
	{Field: "status"},
	{Header: "amount", Value: amountCell, Right: true},
	{Header: "block", Field: "block_number"},
	{Header: "tx hash", Value: shortHexOf("transaction_hash")},
	{Header: "created", Value: shortTimeOf("created_at")},
	{Header: "confirmed", Value: shortTimeOf("confirmed_at")},
}

var historyColumns = []ui.Column{
	{Header: "at", Value: func(row map[string]any) string { return ui.ShortTimeSeconds(str(row, "at")) }},
	{Field: "event"},
	{Field: "status"},
	{Header: "amount", Value: amountCell, Right: true},
	{Field: "block"},
	{Header: "tx hash", Value: shortHexOf("tx_hash")},
}

// disputeColumns is one payment's dispute history ('payments disputes'): the payment is
// the one that was asked for, so it is not a column.
var disputeColumns = []ui.Column{
	{Field: "status"},
	{Header: "reason", Field: "reason_code"},
	{Header: "opened", Value: shortTimeOf("opened_at")},
	{Header: "closed", Value: shortTimeOf("closed_at")},
	{Header: "close reason", Field: "close_reason_code"},
	{Header: "closed by", Field: "closed_by"},
	{Field: "id"},
}

// accountDisputeColumns is the account-wide list ('disputes'): the payment comes first,
// because it is what every dispute command takes.
var accountDisputeColumns = append([]ui.Column{{Header: "payment", Field: "payment_id"}}, disputeColumns...)

var walletColumns = []ui.Column{
	{Field: "id"},
	{Field: "label", Max: 28},
	{Field: "address"},
	{Field: "active"},
	{Header: "tokens", Value: walletTokensCell, Max: 40},
}

// walletTokensCell names the tokens a wallet accepts: the symbols of its active
// holdings, once each. The chain and the default of each are in 'wallets get'.
func walletTokensCell(row map[string]any) string {
	holdings, _ := row["tokens"].([]any)
	var symbols []string
	seen := map[string]bool{}
	for _, h := range holdings {
		holding, _ := h.(map[string]any)
		if active, ok := holding["active"].(bool); ok && !active {
			continue
		}
		token, _ := holding["token"].(map[string]any)
		symbol := str(token, "symbol")
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		symbols = append(symbols, symbol)
	}
	return strings.Join(symbols, ", ")
}

var webhookColumns = []ui.Column{
	{Field: "id"},
	{Field: "name", Max: 28},
	{Field: "active"},
	{Header: "circuit", Field: "circuit_state"},
	{Header: "topics", Value: func(row map[string]any) string {
		// How many, not which: the names alone are wider than the rest of the row,
		// and 'webhooks get' lists them.
		topics, _ := row["topics"].([]any)
		return intOrEmpty(len(topics))
	}, Right: true},
	{Header: "url", Field: "callback_url", Max: 60},
}

// eventCallbackColumns leaves the payload out: it is the whole request body, and it is
// what 'webhooks event-callbacks --json' is for.
var eventCallbackColumns = []ui.Column{
	{Field: "id"},
	{Field: "topic"},
	{Field: "status"},
	{Header: "code", Field: "response_code"},
	{Header: "created", Value: shortTimeOf("created_at")},
	{Header: "about", Value: func(row map[string]any) string {
		return firstNonEmptyStr(str(row, "payment_id"), str(row, "payout_id"), str(row, "payout_destination_id"))
	}},
	{Header: "error", Field: "error_reason", Max: 40},
}

var chainColumns = []ui.Column{
	{Field: "slug"},
	{Field: "name"},
	{Header: "chain id", Field: "chain_id"},
	{Header: "type", Field: "network_type"},
	{Header: "native", Field: "native_symbol"},
	{Header: "contract", Value: func(row map[string]any) string {
		contract, _ := row["contract"].(map[string]any)
		return str(contract, "address")
	}},
	{Header: "version", Value: func(row map[string]any) string {
		contract, _ := row["contract"].(map[string]any)
		return str(contract, "version")
	}},
	{Header: "settlement", Field: "settlement_human"},
}

// ── Operator lists ('rail0 admin …') ─────────────────────────────────────────

// paymentOf reads a field of the payment summary an operator's row carries.
func paymentOf(row map[string]any, key string) any {
	payment, _ := row["payment"].(map[string]any)
	return payment[key]
}

var operatorTransactionColumns = []ui.Column{
	{Field: "id"},
	{Field: "operation"},
	{Field: "status"},
	{Header: "amount", Value: amountCell, Right: true},
	{Header: "chain", Value: func(row map[string]any) string {
		payment, _ := row["payment"].(map[string]any)
		return chainCell(payment)
	}},
	{Header: "payee", Value: func(row map[string]any) string {
		s, _ := paymentOf(row, "payee").(string)
		return ui.ShortHex(s)
	}},
	{Header: "tx hash", Value: shortHexOf("transaction_hash")},
	{Header: "created", Value: shortTimeOf("created_at")},
	{Header: "error", Value: func(row map[string]any) string {
		return firstNonEmptyStr(str(row, "error_reason"), str(row, "error_code"))
	}, Max: 40},
	{Header: "payment", Field: "payment_id"},
}

var syncErrorColumns = []ui.Column{
	{Header: "created", Value: shortTimeOf("created_at")},
	{Field: "reason"},
	{Field: "outcome"},
	{Header: "event", Field: "event_type"},
	{Header: "chain", Value: chainCell},
	{Header: "block", Field: "block_number"},
	{Header: "payment", Value: shortHexOf("payment_id")},
	{Header: "tx hash", Value: shortHexOf("transaction_hash")},
	{Field: "id"},
}

// operatorAccountColumns is the operator's account list: who, in what state, and the id
// every other command takes.
var operatorAccountColumns = []ui.Column{
	{Field: "name", Max: 32},
	{Field: "email", Max: 36},
	{Field: "status"},
	{Header: "operator", Value: func(row map[string]any) string {
		if admin, _ := row["admin"].(bool); admin {
			return "yes"
		}
		return ""
	}},
	{Header: "created", Value: shortTimeOf("created_at")},
	{Field: "id"},
}

// operatorWalletColumns is a merchant's wallet list with who switched a wallet off: what
// decides who may switch it back on.
var operatorWalletColumns = []ui.Column{
	{Field: "id"},
	{Field: "label", Max: 28},
	{Field: "address"},
	{Field: "active"},
	{Header: "off by", Field: "deactivated_by"},
	{Header: "tokens", Value: walletTokensCell, Max: 40},
}

// onboardingColumns is the operator's list of onboarding requests: the state, how the
// last email went and what the request clashes with come before who it is for.
var onboardingColumns = []ui.Column{
	{Header: "created", Value: shortTimeOf("created_at")},
	{Field: "status"},
	{Field: "origin"},
	{Field: "name", Max: 28},
	{Field: "email", Max: 32},
	{Header: "address", Value: shortHexOf("address")},
	{Header: "sends", Field: "send_count", Right: true},
	{Header: "send error", Field: "last_send_error"},
	{Header: "clashes", Value: onboardingConflictsCell},
	{Header: "expires", Value: shortTimeOf("expires_at")},
	{Field: "id"},
}

// onboardingConflictsCell names the fields a request clashes on, once each: the email an
// account already holds, the address of another open request.
func onboardingConflictsCell(row map[string]any) string {
	conflicts, _ := row["conflicts"].([]any)
	var fields []string
	for _, c := range conflicts {
		conflict, _ := c.(map[string]any)
		if f := str(conflict, "field"); f != "" && !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	return strings.Join(fields, ", ")
}

// accountName is the name of the account nested under key (a subscription's owner, an
// action's actor or target), or its id when the row carries no name.
func accountName(row map[string]any, key, idKey string) string {
	account, _ := row[key].(map[string]any)
	return firstNonEmptyStr(str(account, "name"), str(row, idKey))
}

// operatorWebhookColumns is every account's subscriptions: whether it is receiving comes
// first (circuit, failures, last attempt), then whose it is, then where it points.
var operatorWebhookColumns = []ui.Column{
	{Header: "circuit", Field: "circuit_state"},
	{Field: "active"},
	{Header: "failures", Field: "circuit_failure_count", Right: true},
	{Header: "last attempt", Value: func(row map[string]any) string {
		at := ui.ShortTime(str(row, "last_attempt_at"))
		if at == "" {
			return ""
		}
		return str(row, "last_attempt_status") + " " + at
	}},
	{Header: "account", Value: func(row map[string]any) string { return accountName(row, "account", "account_id") }, Max: 24},
	{Field: "name", Max: 24},
	{Header: "url", Field: "callback_url", Max: 48},
	{Field: "id"},
}

// operatorDeliveryColumns is the delivery log across accounts: the outcome first, then
// which subscription of which account, then what it was about.
var operatorDeliveryColumns = []ui.Column{
	{Header: "created", Value: shortTimeOf("created_at")},
	{Field: "topic"},
	{Field: "status"},
	{Header: "code", Field: "response_code"},
	{Header: "account", Value: func(row map[string]any) string {
		webhook, _ := row["webhook"].(map[string]any)
		return accountName(webhook, "account", "account_id")
	}, Max: 24},
	{Header: "webhook", Value: func(row map[string]any) string {
		webhook, _ := row["webhook"].(map[string]any)
		return firstNonEmptyStr(str(webhook, "name"), str(row, "webhook_id"))
	}, Max: 24},
	{Header: "about", Value: func(row map[string]any) string {
		return firstNonEmptyStr(str(row, "payment_id"), str(row, "payout_id"), str(row, "payout_destination_id"))
	}},
	{Header: "error", Field: "error_reason", Max: 32},
	{Field: "id"},
}

// auditColumns is the operator audit log: when, what, by whom, to whom, on what, and the
// detail the gateway recorded, as compact JSON.
var auditColumns = []ui.Column{
	{Header: "at", Value: func(row map[string]any) string { return ui.ShortTimeSeconds(str(row, "created_at")) }},
	{Field: "action"},
	{Header: "actor", Value: func(row map[string]any) string {
		return firstNonEmptyStr(accountName(row, "actor", ""), ui.ShortHex(str(row, "actor_address")), str(row, "actor_account_id"))
	}, Max: 24},
	{Header: "target", Value: func(row map[string]any) string { return accountName(row, "target_account", "target_account_id") }, Max: 24},
	{Header: "resource", Value: func(row map[string]any) string {
		return strings.TrimSpace(str(row, "resource_type") + " " + str(row, "resource_id"))
	}, Max: 46},
	{Header: "detail", Value: func(row map[string]any) string {
		if row["detail"] == nil {
			return ""
		}
		b, err := json.Marshal(row["detail"])
		if err != nil {
			return ""
		}
		return string(b)
	}, Max: 60},
}

var traceTimelineColumns = []ui.Column{
	{Header: "at", Value: func(row map[string]any) string { return ui.ShortTimeSeconds(str(row, "at")) }},
	{Field: "kind"},
	{Field: "event"},
	{Field: "status"},
	{Header: "amount", Value: amountCell, Right: true},
	{Field: "code"},
	{Header: "detail", Value: func(row map[string]any) string { return ui.ShortHex(str(row, "detail")) }, Max: 48},
}
