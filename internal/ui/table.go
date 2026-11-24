package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// The --pretty view: a table for a list, key-value rows for one record. It is the view
// for a person at a terminal and nothing else reads it — --json, --quiet, a pipe and
// agent mode all print the result itself — so it is free to choose, shorten and drop,
// which the other views must never do.

// Column is one column of a list's --pretty table.
//
// A list that names its columns (Table) shows those and only those, in that order,
// which is also their order of importance: when the terminal is too narrow, columns are
// dropped from the right. A list that names none gets one column per field.
type Column struct {
	// Header is printed in capitals above the column. Empty means the Field's name.
	Header string
	// Field is the row's key whose value the cell shows. It also picks the colour: a
	// "status" field is painted like every other status in the CLI.
	Field string
	// Value computes the cell from the whole row, for a cell a person reads better in
	// another form than the field's (a shortened address, a token's symbol). It is
	// called only when the table is drawn, so whatever it looks up costs nothing in the
	// other output modes.
	Value func(row map[string]any) string
	// Right aligns the column to the right: amounts, so the decimals line up.
	Right bool
	// Max is the longest cell in characters; a longer one is cut and ends in "…". Zero
	// means no limit.
	Max int
}

func (c Column) title() string {
	h := c.Header
	if h == "" {
		h = c.Field
	}
	return strings.ToUpper(h)
}

func (c Column) cell(row map[string]any) string {
	var s string
	if c.Value != nil {
		s = c.Value(row)
	} else {
		s = cellStr(row[c.Field])
	}
	return cut(oneLine(s), c.Max)
}

// Table is a list result together with the columns its --pretty table shows. A command
// returns it from RunAPI in place of the bare list (inside a Paged when the list is
// paged). The columns reach the --pretty view only: every other output mode prints the
// rows whole, exactly as it would have printed the bare list.
type Table struct {
	Rows    any
	Columns []Column
}

// MarshalJSON prints the rows alone, so a Table that reaches an encoder without being
// unwrapped is still the list and never a {"Rows":…,"Columns":…} object.
func (t Table) MarshalJSON() ([]byte, error) { return json.Marshal(t.Rows) }

func unwrapTable(v any) (any, []Column) {
	if t, ok := v.(Table); ok {
		return t.Rows, t.Columns
	}
	return v, nil
}

// ShortHex shortens a 0x value for a table cell: "0x2D2B…57D5" for an address, the same
// for a hash. The start and the end are what a person compares by eye; the whole value
// is one command away (the record's own view, or --json). Anything else is returned as
// it is.
func ShortHex(s string) string {
	if !strings.HasPrefix(s, "0x") || len(s) <= 13 {
		return s
	}
	return s[:6] + "…" + s[len(s)-4:]
}

// ShortTime renders an RFC 3339 timestamp for a table cell: "2026-10-22 14:28", UTC like
// the timestamp itself, to the minute. A value that is not a timestamp is returned as it
// is, and an empty one stays empty.
func ShortTime(s string) string { return shortTime(s, "2006-01-02 15:04") }

// ShortTimeSeconds is ShortTime to the second, for rows that follow each other within
// one minute (a payment's history).
func ShortTimeSeconds(s string) string { return shortTime(s, "2006-01-02 15:04:05") }

func shortTime(s, layout string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(layout)
}

// terminalWidth is the width of the terminal the table is drawn on, 0 when it cannot be
// read (then nothing is dropped). A variable so a test can set it.
var terminalWidth = func() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		return 0
	}
	return w
}

// printPretty renders v as a human-oriented key-value (object) or table (array)
// view — the --pretty mode. It marshals v to JSON and decodes it generically so
// any SDK type is handled uniformly. columns are the ones a list asked for (Table), nil
// for one column per field.
func printPretty(v any, columns []Column) error {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}

	// Try array first.
	var arr []any
	if json.Unmarshal(b, &arr) == nil && len(arr) > 0 {
		fmt.Println()
		notShown := printTable(arr, columns)
		fmt.Println()
		printNotShown(notShown)
		return nil
	}

	// Try object.
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		fmt.Println()
		printMap(m, 2)
		fmt.Println()
		return nil
	}

	fmt.Println(string(b))
	return nil
}

// printTable renders a JSON array. If its elements are objects it prints a table —
// the columns asked for, or one per field — and otherwise one line per item. It returns
// the names of the columns it left out: the ones that did not fit the terminal, and,
// for a list that named no columns, the fields that hold a nested value.
func printTable(arr []any, cols []Column) []string {
	var notShown []string
	if len(cols) == 0 {
		cols, notShown = fieldColumns(arr)
	}
	if len(cols) == 0 {
		for _, item := range arr {
			fmt.Printf("  %v\n", item)
		}
		return nil
	}

	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = lipgloss.Width(c.title())
	}
	rows := make([][]string, len(arr))
	for r, item := range arr {
		m, _ := item.(map[string]any)
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = c.cell(m)
			if w := lipgloss.Width(row[i]); w > widths[i] {
				widths[i] = w
			}
		}
		rows[r] = row
	}

	shown := fitColumns(cols, widths, terminalWidth())
	for _, c := range cols[shown:] {
		notShown = append(notShown, strings.ToLower(c.title()))
	}

	var header, sep strings.Builder
	header.WriteString("  ")
	sep.WriteString("  ")
	for i, c := range cols[:shown] {
		if i > 0 {
			header.WriteString("  ")
			sep.WriteString("  ")
		}
		header.WriteString(HeaderStyle.Render(pad(c.title(), widths[i], c.Right)))
		sep.WriteString(strings.Repeat("─", widths[i]))
	}
	fmt.Println(header.String())
	fmt.Println(DimStyle.Render(sep.String()))

	for _, row := range rows {
		var line strings.Builder
		line.WriteString("  ")
		for i, c := range cols[:shown] {
			if i > 0 {
				line.WriteString("  ")
			}
			width := widths[i]
			if i == shown-1 && !c.Right {
				width = 0 // nothing follows the last column: no trailing spaces
			}
			line.WriteString(renderCell(c, cut(row[i], widths[i]), width))
		}
		fmt.Println(line.String())
	}
	return notShown
}

// printNotShown says, under a table, which columns it left out and where they are. On
// stderr like the page note, so stdout stays the table; silent when nothing was left
// out.
func printNotShown(names []string) {
	if len(names) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, NullStyle.Render(
		"  not shown: "+strings.Join(names, ", ")+" — --json has every field"))
}

// minElastic is the narrowest a column with a Max is squeezed to before a column is
// dropped instead: below it a cut value says too little to be worth its place.
const minElastic = 16

// fitColumns fits the table to a terminal of width columns and returns how many of the
// columns, counted from the left, it shows. widths holds each column's natural width and
// is narrowed in place where a column was squeezed.
//
// Two things give way, in this order. A column with a Max holds free text (a
// description, a URL): it is squeezed, down to minElastic, and its cells end in "…". Only
// when squeezing every such column is not enough are columns dropped, from the right —
// the list put its least important ones there. Each column takes its width, two spaces
// part it from the next, and the table is indented by two. At least one column is always
// shown, and an unknown width (0) changes nothing.
func fitColumns(cols []Column, widths []int, width int) int {
	n := len(widths)
	if width <= 0 || n == 0 {
		return n
	}
	elastic := func(i int) bool { return cols[i].Max > 0 && widths[i] > minElastic }
	// The fewest columns to drop: the table at its narrowest must fit.
	for ; n > 1; n-- {
		least := 2 + 2*(n-1)
		for i := 0; i < n; i++ {
			if elastic(i) {
				least += minElastic
			} else {
				least += widths[i]
			}
		}
		if least <= width {
			break
		}
	}
	// Then squeeze only as much as needed, the widest column first.
	total := 2 + 2*(n-1)
	for i := 0; i < n; i++ {
		total += widths[i]
	}
	for total > width {
		widest := -1
		for i := 0; i < n; i++ {
			if elastic(i) && (widest < 0 || widths[i] > widths[widest]) {
				widest = i
			}
		}
		if widest < 0 {
			break
		}
		widths[widest]--
		total--
	}
	return n
}

// fieldColumns is the table of a list that named no columns: one column per field, in
// sortFields order. A field that holds an object, or a list of objects, in some row is
// left out and named in the second result: its JSON does not fit a cell, and printed
// there it pushed every other column off the screen. When every field is nested they
// are all kept, so the table never comes out empty.
func fieldColumns(arr []any) ([]Column, []string) {
	seen := map[string]bool{}
	nested := map[string]bool{}
	var names []string
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range m {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
			if isNested(v) {
				nested[k] = true
			}
		}
	}
	sortFields(names)

	var cols []Column
	var left []string
	for _, k := range names {
		if nested[k] {
			left = append(left, k)
			continue
		}
		cols = append(cols, Column{Field: k})
	}
	if len(cols) == 0 {
		cols, left = nil, nil
		for _, k := range names {
			cols = append(cols, Column{Field: k})
		}
	}
	return cols, left
}

// isNested reports whether a decoded JSON value is an object or a list holding one —
// what a table cell cannot show. A list of plain values is not: it prints joined.
func isNested(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		return true
	case []any:
		for _, item := range t {
			switch item.(type) {
			case map[string]any, []any:
				return true
			}
		}
	}
	return false
}

func cellStr(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return numStr(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any:
		if isNested(t) {
			break
		}
		parts := make([]string, len(t))
		for i, item := range t {
			parts[i] = cellStr(item)
		}
		return strings.Join(parts, ", ")
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func renderCell(c Column, cell string, width int) string {
	padded := pad(cell, width, c.Right)
	switch {
	case c.Field == "status":
		return StatusStyle(cell).Render(padded)
	case strings.HasPrefix(cell, "0x"):
		return HexStyle.Render(padded)
	case cell == "true":
		return TrueStyle.Render(padded)
	case cell == "false":
		return FalseStyle.Render(padded)
	default:
		return StringStyle.Render(padded)
	}
}

// pad fills s with spaces up to width terminal cells, on the left for a right-aligned
// column. Cells, not bytes: "…" and an accented letter are one cell each.
func pad(s string, width int, right bool) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	if right {
		return strings.Repeat(" ", gap) + s
	}
	return s + strings.Repeat(" ", gap)
}

// cut shortens s to max characters, the last one being "…" when something was dropped.
func cut(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// oneLine keeps a cell on its row: a line break inside a value becomes a space.
func oneLine(s string) string {
	if !strings.ContainsAny(s, "\r\n\t") {
		return s
	}
	return strings.Join(strings.Fields(s), " ")
}

// printMap renders a JSON object as indented key-value rows. The values of one object
// start in the same column, set by the longest name among them: a fixed label width
// either wasted space or, for a name longer than it, wrapped the name mid-word.
func printMap(m map[string]any, indent int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortFields(keys)

	// Only the names that have a value beside them set the column: a nested object's
	// name stands alone on its line.
	width := 0
	for _, k := range keys {
		switch m[k].(type) {
		case map[string]any, []any:
			continue
		}
		if w := lipgloss.Width(k); w > width {
			width = w
		}
	}

	indentStr := strings.Repeat(" ", indent)
	for _, k := range keys {
		label := LabelStyle.Render(indentStr + pad(k, width, false))
		switch val := m[k].(type) {
		case map[string]any:
			fmt.Println(LabelStyle.Render(indentStr + k))
			printMap(val, indent+4)
		case []any:
			fmt.Println(LabelStyle.Render(indentStr + k))
			for _, item := range val {
				if sub, ok := item.(map[string]any); ok {
					printMap(sub, indent+4)
					fmt.Println()
				} else {
					fmt.Printf("%s    %s\n", indentStr, StringStyle.Render(cellStr(item)))
				}
			}
		case bool:
			if val {
				fmt.Printf("%s  %s\n", label, TrueStyle.Render("true"))
			} else {
				fmt.Printf("%s  %s\n", label, FalseStyle.Render("false"))
			}
		case float64:
			fmt.Printf("%s  %s\n", label, NumberStyle.Render(numStr(val)))
		case string:
			fmt.Printf("%s  %s\n", label, renderPlainStr(k, val))
		case nil:
			fmt.Printf("%s  %s\n", label, NullStyle.Render("—"))
		default:
			fmt.Printf("%s  %s\n", label, StringStyle.Render(fmt.Sprintf("%v", val)))
		}
	}
}

// renderPlainStr colours a bare string value for the --pretty key-value view.
// A "status" field gets the state-aware StatusStyle (matching the table and
// colorized-JSON renderers); 0x-prefixed values use HexStyle; everything else
// uses the plain string colour.
func renderPlainStr(key, s string) string {
	switch {
	case key == "status":
		return StatusStyle(s).Render(s)
	case strings.HasPrefix(s, "0x"):
		return HexStyle.Render(s)
	default:
		return StringStyle.Render(s)
	}
}
