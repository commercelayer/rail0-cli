package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// atWidth draws with the terminal reported as width columns wide (0 = unknown).
func atWidth(t *testing.T, width int) {
	t.Helper()
	old := terminalWidth
	terminalWidth = func() int { return width }
	t.Cleanup(func() { terminalWidth = old })
}

func decode(t *testing.T, raw string) []any {
	t.Helper()
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		t.Fatal(err)
	}
	return arr
}

func plain(s string) string { return ansiRe.ReplaceAllString(s, "") }

// A list that names its columns shows those, in that order, under their headers — and
// none of the row's other fields.
func TestTableShowsTheNamedColumnsOnly(t *testing.T) {
	atWidth(t, 0)
	rows := decode(t, `[{"id":"p1","status":"captured","amount":"2500000","amount_human":"2.5","config_hash":"0xabc","payer":"0xe3DD5ea618F74De4C95B0ff0EbF9921A06a694c4"}]`)
	cols := []Column{
		{Field: "id"},
		{Field: "status"},
		{Header: "amount", Field: "amount_human", Right: true},
		{Header: "payer", Value: func(row map[string]any) string { return ShortHex(row["payer"].(string)) }},
	}
	var notShown []string
	out := plain(captureStdout(t, func() { notShown = printTable(rows, cols) }))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header, rule and one row, got %d lines:\n%s", len(lines), out)
	}
	if got := strings.Fields(lines[0]); strings.Join(got, " ") != "ID STATUS AMOUNT PAYER" {
		t.Errorf("header = %q", lines[0])
	}
	if got := strings.Fields(lines[2]); strings.Join(got, " ") != "p1 captured 2.5 0xe3DD…94c4" {
		t.Errorf("row = %q", lines[2])
	}
	if strings.Contains(out, "config_hash") || strings.Contains(out, "0xabc") || strings.Contains(out, "2500000") {
		t.Errorf("a field that is not a column was printed:\n%s", out)
	}
	if len(notShown) != 0 {
		t.Errorf("nothing was left out, got %v", notShown)
	}
}

// Right-aligned cells end in the same column, and no line carries trailing spaces.
func TestTableAlignment(t *testing.T) {
	atWidth(t, 0)
	rows := decode(t, `[{"n":"2.5","label":"a"},{"n":"1250.75","label":"longer"}]`)
	out := plain(captureStdout(t, func() {
		printTable(rows, []Column{{Field: "n", Right: true}, {Field: "label"}})
	}))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.HasPrefix(lines[2], "      2.5  a") || !strings.HasPrefix(lines[3], "  1250.75  longer") {
		t.Errorf("amounts are not right-aligned:\n%s", out)
	}
	for _, l := range lines[2:] {
		if strings.HasSuffix(l, " ") {
			t.Errorf("trailing spaces in %q", l)
		}
	}
}

// When the terminal is too narrow the table drops columns from the right and says which.
func TestTableDropsColumnsFromTheRight(t *testing.T) {
	rows := decode(t, `[{"a":"aaaaaaaaaa","b":"bbbbbbbbbb","c":"cccccccccc"}]`)
	cols := []Column{{Field: "a"}, {Field: "b"}, {Field: "c"}}

	atWidth(t, 26) // 2 indent + 10 + 2 + 10 = 24 fits, the third column does not
	var notShown []string
	out := plain(captureStdout(t, func() { notShown = printTable(rows, cols) }))
	if strings.Contains(out, "cccc") || !strings.Contains(out, "bbbb") {
		t.Errorf("want columns a and b only:\n%s", out)
	}
	if len(notShown) != 1 || notShown[0] != "c" {
		t.Errorf("notShown = %v, want [c]", notShown)
	}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if n := len([]rune(l)); n > 26 {
			t.Errorf("line is %d wide on a 26-column terminal: %q", n, l)
		}
	}

	atWidth(t, 5) // narrower than any column: the first one is still shown
	out = plain(captureStdout(t, func() { notShown = printTable(rows, cols) }))
	if !strings.Contains(out, "aaaaaaaaaa") || len(notShown) != 2 {
		t.Errorf("the first column must survive any width (notShown %v):\n%s", notShown, out)
	}
}

// A free-text column is squeezed before any column is dropped, and only as far as
// needed.
func TestTableSqueezesFreeTextBeforeDropping(t *testing.T) {
	rows := decode(t, `[{"id":"0123456789","note":"a description that goes on for a while","state":"ok"}]`)
	cols := []Column{{Field: "id"}, {Field: "note", Max: 60}, {Field: "state"}}

	// 2 + 10 + 2 + note + 2 + 5 ("STATE") = 21 + note: 20 left for the note on 41 columns.
	atWidth(t, 41)
	var notShown []string
	out := plain(captureStdout(t, func() { notShown = printTable(rows, cols) }))
	if len(notShown) != 0 {
		t.Fatalf("no column should be dropped, got %v:\n%s", notShown, out)
	}
	if !strings.Contains(out, "a description that …") || !strings.Contains(out, "ok") {
		t.Errorf("want the note cut to 20 characters and the last column kept:\n%s", out)
	}

	// Below minElastic for the note, the last column goes instead.
	atWidth(t, 30)
	out = plain(captureStdout(t, func() { notShown = printTable(rows, cols) }))
	if len(notShown) != 1 || notShown[0] != "state" {
		t.Errorf("notShown = %v, want [state]:\n%s", notShown, out)
	}
}

func TestFitColumnsLeavesAnUnknownWidthAlone(t *testing.T) {
	cols := []Column{{Field: "a"}, {Field: "b", Max: 80}}
	widths := []int{40, 80}
	if n := fitColumns(cols, widths, 0); n != 2 || widths[1] != 80 {
		t.Errorf("n = %d, widths = %v", n, widths)
	}
}

// A list that names no columns gets one per field, minus the fields holding nested
// values, which are named instead of printed as JSON in a cell.
func TestFieldColumnsLeaveNestedValuesOut(t *testing.T) {
	rows := decode(t, `[{"id":"w1","label":"Main","topics":["a","b"],"tokens":[{"symbol":"USDC"}],"payment":{"id":"p1"}}]`)
	cols, left := fieldColumns(rows)
	var names []string
	for _, c := range cols {
		names = append(names, c.Field)
	}
	if strings.Join(names, ",") != "id,label,topics" {
		t.Errorf("columns = %v", names)
	}
	if strings.Join(left, ",") != "payment,tokens" {
		t.Errorf("left out = %v", left)
	}
	// A list of plain values prints joined.
	if got := cols[2].cell(rows[0].(map[string]any)); got != "a, b" {
		t.Errorf("topics cell = %q", got)
	}
	// Every field nested: all are kept, so the table is not empty.
	cols, left = fieldColumns(decode(t, `[{"contract":{"address":"0x1"}}]`))
	if len(cols) != 1 || len(left) != 0 {
		t.Errorf("cols = %v, left = %v", cols, left)
	}
}

func TestShortHex(t *testing.T) {
	for in, want := range map[string]string{
		"0x2D2B3d4E72370A7256633D426518a372a53557D5":                         "0x2D2B…57D5",
		"0xed7c607d5240f23106e8c32eb5b53a3326ab7ba8822622f6e7a4fc92596438a2": "0xed7c…38a2",
		"0xabc":                                "0xabc",
		"":                                     "",
		"01a16403-398f-7f15-a8e5-8a9b78900e1a": "01a16403-398f-7f15-a8e5-8a9b78900e1a",
	} {
		if got := ShortHex(in); got != want {
			t.Errorf("ShortHex(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortTime(t *testing.T) {
	if got := ShortTime("2026-10-22T14:28:15.364Z"); got != "2026-10-22 14:28" {
		t.Errorf("got %q", got)
	}
	// Another offset is shown in UTC, like every timestamp the gateway returns.
	if got := ShortTimeSeconds("2026-10-22T16:28:15+02:00"); got != "2026-10-22 14:28:15" {
		t.Errorf("got %q", got)
	}
	if got := ShortTime(""); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := ShortTime("soon"); got != "soon" {
		t.Errorf("not a timestamp = %q", got)
	}
}

func TestCutAndOneLine(t *testing.T) {
	if got := cut("payment demo", 8); got != "payment…" {
		t.Errorf("cut = %q", got)
	}
	if got := cut("città", 5); got != "città" { // characters, not bytes
		t.Errorf("cut = %q", got)
	}
	if got := oneLine("first\nsecond\tthird"); got != "first second third" {
		t.Errorf("oneLine = %q", got)
	}
}

// The values of a record start in one column, set by the longest name, and no name is
// broken over two lines — which a fixed 22-character label did to every longer one.
func TestRecordViewAlignsOnTheLongestName(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(`{"id":"p1","authorization_expires_in":"expired","capturable_amount_human":"1.5","empty":null}`), &m); err != nil {
		t.Fatal(err)
	}
	out := plain(captureStdout(t, func() { printMap(m, 2) }))
	want := []string{
		"  id                        p1",
		"  authorization_expires_in  expired",
		"  capturable_amount_human   1.5",
		"  empty                     —", // a null reads as a dash, not as <nil>
	}
	if got := strings.Split(strings.TrimRight(out, "\n"), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", out, strings.Join(want, "\n"))
	}
}

// A nested object's name stands alone and does not widen the column of its siblings.
func TestRecordViewNestedNameDoesNotSetTheColumn(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(`{"id":"a1","a_very_long_nested_object_name":{"min":1}}`), &m); err != nil {
		t.Fatal(err)
	}
	out := plain(captureStdout(t, func() { printMap(m, 2) }))
	if !strings.Contains(out, "  id  a1\n") {
		t.Errorf("id should be padded to its own length only:\n%s", out)
	}
}

// The columns belong to the --pretty view alone: every other mode prints the rows whole.
func TestTableIsTheBareListEverywhereElse(t *testing.T) {
	rows := []map[string]any{{"id": "p1", "config_hash": "0xabc"}}
	result := Paged{
		Items: Table{Rows: rows, Columns: []Column{{Field: "id"}}},
		Page:  PageInfo{Page: 1, PerPage: 25, Total: 1, TotalPages: 1},
	}

	b, err := json.Marshal(Table{Rows: rows, Columns: []Column{{Field: "id"}}})
	if err != nil || string(b) != `[{"config_hash":"0xabc","id":"p1"}]` {
		t.Errorf("a Table must marshal as its rows, got %s (%v)", b, err)
	}

	SetAgent(true)
	out := captureStdout(t, func() {
		if err := RunAPI(func() (any, error) { return result, nil }); err != nil {
			t.Error(err)
		}
	})
	SetAgent(false)
	if !strings.Contains(out, `"data":[{"config_hash":"0xabc","id":"p1"}]`) || !strings.Contains(out, `"page":{`) {
		t.Errorf("agent envelope = %s", out)
	}

	SetJSON(true)
	out = captureStdout(t, func() {
		if err := RunAPI(func() (any, error) { return result, nil }); err != nil {
			t.Error(err)
		}
	})
	SetJSON(false)
	var back []map[string]any
	if err := json.Unmarshal([]byte(out), &back); err != nil || len(back) != 1 || back[0]["config_hash"] != "0xabc" {
		t.Errorf("--json = %s (%v)", out, err)
	}

	SetQuiet(true)
	out = captureStdout(t, func() {
		if err := RunAPI(func() (any, error) { return result, nil }); err != nil {
			t.Error(err)
		}
	})
	SetQuiet(false)
	if strings.TrimSpace(out) != "p1" {
		t.Errorf("--quiet = %q", out)
	}
}
