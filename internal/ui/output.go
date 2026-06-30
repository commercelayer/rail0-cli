package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

var (
	jsonMode   bool
	prettyMode bool
)

// SetJSON forces plain JSON output regardless of TTY state.
func SetJSON(on bool) { jsonMode = on }

// SetPretty selects the key-value/table view (--pretty) on a TTY instead of the
// default colorized JSON.
func SetPretty(on bool) { prettyMode = on }

func isTerminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// RunAPI executes fn and prints its result. On a TTY it shows a bubbletea
// spinner on stderr while the call is in flight, then renders to stdout:
// syntax-highlighted JSON by default, or the key-value/table view when --pretty
// is set. When stdout is piped or --json is set it skips the spinner and emits
// plain indented JSON (no ANSI), suitable for jq and shell pipelines. A nil
// result (e.g. a 204 delete) prints nothing.
func RunAPI(fn func() (any, error)) error {
	if jsonMode || !isTerminal(os.Stdout) {
		data, err := fn()
		if err != nil {
			return err
		}
		return printJSON(data)
	}
	p := tea.NewProgram(newFetchModel(fn), tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return err
	}
	fm := final.(fetchModel)
	if fm.result.err != nil {
		return fm.result.err
	}
	if prettyMode {
		return printPretty(fm.result.data)
	}
	return printStyled(fm.result.data)
}

// printJSON emits plain, indented JSON with no colour — the machine-readable
// form used when piped or when --json is passed.
func printJSON(v any) error {
	if v == nil {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

// printStyled prints v as colorized, indented JSON-like text for a terminal. It
// marshals v (a typed SDK value) to JSON, decodes it into a generic tree, then
// walks the tree emitting that syntax with the package styles applied to keys
// and scalar values. Object keys are emitted unquoted (colour sets them apart),
// so the result is intentionally not strict JSON — use printJSON (piped /
// --json) for that. Keys are ordered by sortFields (identity first, timestamps
// last) for a stable, readable layout rather than Go's map iteration order.
func printStyled(v any) error {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	var tree any
	if err := json.Unmarshal(b, &tree); err != nil {
		// Not a JSON object/array/scalar we can colorize — fall back to raw bytes.
		fmt.Println(string(b))
		return nil
	}
	fmt.Println(renderJSON(tree, 0, ""))
	return nil
}

// indentUnit matches encoding/json's two-space indentation so the colorized and
// plain (--json) forms line up visually.
const indentUnit = "  "

// renderJSON returns the colorized JSON text for a decoded value. depth drives
// indentation of nested objects/arrays; key is the field name this value sits
// under (empty at the root or inside arrays) and lets scalars pick a
// context-aware style — notably status values via StatusStyle.
func renderJSON(v any, depth int, key string) string {
	switch t := v.(type) {
	case nil:
		return NullStyle.Render("null")
	case bool:
		if t {
			return TrueStyle.Render("true")
		}
		return FalseStyle.Render("false")
	case float64:
		return NumberStyle.Render(numStr(t))
	case string:
		return renderString(t, key)
	case []any:
		return renderArray(t, depth, key)
	case map[string]any:
		return renderObject(t, depth)
	default:
		// Should not happen for json.Unmarshal output; encode defensively.
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// renderObject renders an object across multiple lines with sorted keys. Keys
// are emitted WITHOUT surrounding quotes (relying on colour to set them apart)
// for readability — so this colorized view is JSON-like, not strict JSON. The
// machine-readable strict-JSON form is printJSON (piped / --json).
func renderObject(m map[string]any, depth int) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortFields(keys)

	inner := strings.Repeat(indentUnit, depth+1)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		b.WriteString(inner)
		b.WriteString(KeyStyle.Render(k))
		b.WriteString(": ")
		b.WriteString(renderJSON(m[k], depth+1, k))
		if i < len(keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte('}')
	return b.String()
}

// renderArray renders a JSON array across multiple lines. key is threaded to the
// elements so a field like "transactions" still colors each element's status.
func renderArray(arr []any, depth int, key string) string {
	if len(arr) == 0 {
		return "[]"
	}
	inner := strings.Repeat(indentUnit, depth+1)
	var b strings.Builder
	b.WriteString("[\n")
	for i, item := range arr {
		b.WriteString(inner)
		b.WriteString(renderJSON(item, depth+1, key))
		if i < len(arr)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte(']')
	return b.String()
}

// renderString returns a quoted, colorized JSON string. A "status" field uses
// the state-aware StatusStyle; 0x-prefixed values (addresses, hashes) use
// HexStyle; everything else uses the plain string colour.
func renderString(s, key string) string {
	q := quote(s)
	switch {
	case key == "status":
		return StatusStyle(s).Render(q)
	case strings.HasPrefix(s, "0x"):
		return HexStyle.Render(q)
	default:
		return StringStyle.Render(q)
	}
}

// quote renders s as a JSON string literal (with surrounding quotes and proper
// escaping) using encoding/json, so embedded quotes/newlines stay valid.
func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return string(b)
}

// numStr formats a JSON number the way encoding/json would: whole values that
// fit in an int64 print without a decimal point or exponent (e.g. chain ids),
// the rest use the shortest round-tripping float form.
func numStr(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// fieldPriority lists the identity / human-name fields that should lead an
// object view.
var fieldPriority = map[string]int{
	"id": 0, "slug": 1, "name": 2, "label": 3, "symbol": 4, "address": 5, "title": 6,
}

// fieldGroup buckets a field for ordering: identity/name fields first (0),
// timestamps (…_at, e.g. created_at/updated_at) always last (2), everything
// else in between (1).
func fieldGroup(name string) int {
	if _, ok := fieldPriority[name]; ok {
		return 0
	}
	if strings.HasSuffix(name, "_at") {
		return 2
	}
	return 1
}

// sortFields orders field names so id and slug/name/label lead, timestamps trail
// (consistently across every resource), and the rest sit alphabetically between.
func sortFields(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		gi, gj := fieldGroup(names[i]), fieldGroup(names[j])
		if gi != gj {
			return gi < gj
		}
		if gi == 0 { // identity group: by defined priority
			return fieldPriority[names[i]] < fieldPriority[names[j]]
		}
		return names[i] < names[j] // middle & timestamps: alphabetical
	})
}

// PrintStyled is exported for testing.
func PrintStyled(v any) error { return printStyled(v) }

// printPretty renders v as a human-oriented key-value (object) or table (array)
// view — the --pretty mode. It marshals v to JSON and decodes it generically so
// any SDK type is handled uniformly.
func printPretty(v any) error {
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
		printArray(arr)
		fmt.Println()
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

// printArray renders a JSON array. If every element is an object it prints a
// table; otherwise it falls back to one-line-per-item.
func printArray(arr []any) {
	// Collect all keys in order of first appearance.
	seen := map[string]bool{}
	var cols []string
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			for k := range m {
				if !seen[k] {
					seen[k] = true
					cols = append(cols, k)
				}
			}
		}
	}
	sortFields(cols)
	if len(cols) == 0 {
		for _, item := range arr {
			fmt.Printf("  %v\n", item)
		}
		return
	}

	// Compute column widths.
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	rows := make([][]string, len(arr))
	for r, item := range arr {
		m, _ := item.(map[string]any)
		row := make([]string, len(cols))
		for i, c := range cols {
			cell := cellStr(m[c])
			row[i] = cell
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
		rows[r] = row
	}

	// Header.
	header := "  "
	sep := "  "
	for i, c := range cols {
		header += HeaderStyle.Render(padRight(strings.ToUpper(c), widths[i]))
		sep += strings.Repeat("─", widths[i])
		if i < len(cols)-1 {
			header += "  "
			sep += "  "
		}
	}
	fmt.Println(header)
	fmt.Println(DimStyle.Render(sep))

	// Rows.
	for _, row := range rows {
		line := "  "
		for i, cell := range row {
			line += renderCell(cols[i], cell, widths[i])
			if i < len(row)-1 {
				line += "  "
			}
		}
		fmt.Println(line)
	}
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
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func renderCell(col, cell string, width int) string {
	padded := padRight(cell, width)
	switch {
	case strings.HasPrefix(cell, "0x"):
		return HexStyle.Render(padded)
	case cell == "true":
		return TrueStyle.Render(padded)
	case cell == "false":
		return FalseStyle.Render(padded)
	case col == "status":
		return StatusStyle(cell).Render(padded)
	default:
		return StringStyle.Render(padded)
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// printMap renders a JSON object as indented key-value rows.
func printMap(m map[string]any, indent int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortFields(keys)

	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		label := LabelStyle.Render(pad + k)
		switch val := m[k].(type) {
		case map[string]any:
			fmt.Println(label)
			printMap(val, indent+4)
		case []any:
			fmt.Println(label)
			for _, item := range val {
				if sub, ok := item.(map[string]any); ok {
					printMap(sub, indent+4)
					fmt.Println()
				} else {
					fmt.Printf("%s    %v\n", pad, item)
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
			fmt.Printf("%s  %s\n", label, renderPlainStr(val))
		default:
			fmt.Printf("%s  %s\n", label, StringStyle.Render(fmt.Sprintf("%v", val)))
		}
	}
}

func renderPlainStr(s string) string {
	if strings.HasPrefix(s, "0x") {
		return HexStyle.Render(s)
	}
	return StringStyle.Render(s)
}

// Fatal prints a styled error to stderr and exits with code 1.
func Fatal(err error) {
	if isTerminal(os.Stderr) {
		mark := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true).Render("✗")
		body := ErrStyle.Render(err.Error())
		fmt.Fprintf(os.Stderr, "\n  %s  %s\n\n", mark, body)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(1)
}
