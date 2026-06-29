package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

var jsonMode bool

// SetJSON forces plain JSON output regardless of TTY state.
func SetJSON(on bool) { jsonMode = on }

func isTerminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// RunAPI executes fn and prints the result. On a TTY it shows a bubbletea
// spinner on stderr while the call is in flight, then prints styled output to
// stdout. When stdout is piped or --json is set it calls fn directly and emits
// plain indented JSON.
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
	return printStyled(fm.result.data)
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func printStyled(v any) error {
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

// fieldPriority lists the identity / human-name fields that should lead a table
// or object view.
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
// (consistently across every resource), and the rest sit alphabetically between
// — applied to both table columns and object-view keys.
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

// printArray renders a JSON array. When elements carry nested arrays/objects
// (e.g. wallets with their tokens), a flat table can't show them, so each
// element is printed as its own record block with sub-tables; otherwise it
// prints a flat table.
func printArray(arr []any) {
	if hasNestedObjects(arr) {
		printRecords(arr)
		return
	}
	printTable(arr, 2)
}

// hasNestedObjects reports whether any element has a value that is itself an
// array or object — i.e. it won't fit a single table cell.
func hasNestedObjects(arr []any) bool {
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, v := range m {
			switch v.(type) {
			case []any, map[string]any:
				return true
			}
		}
	}
	return false
}

// printRecords prints each element as an indented key/value block (nested
// collections become sub-tables via printMap), separated by a blank line.
func printRecords(arr []any) {
	for i, item := range arr {
		if m, ok := item.(map[string]any); ok {
			printMap(m, 2)
		} else {
			fmt.Printf("  %v\n", item)
		}
		if i < len(arr)-1 {
			fmt.Println()
		}
	}
}

// isObjectArray reports whether a is a non-empty array whose every element is an
// object (so it can be rendered as a sub-table).
func isObjectArray(a []any) bool {
	if len(a) == 0 {
		return false
	}
	for _, x := range a {
		if _, ok := x.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// printTable renders an array of objects as an aligned table, left-indented by
// `indent` spaces, with columns ordered by sortFields.
func printTable(arr []any, indent int) {
	pad := strings.Repeat(" ", indent)
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
			fmt.Printf("%s%v\n", pad, item)
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
	header := pad
	sep := pad
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
		line := pad
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
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
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
			if isObjectArray(val) {
				printTable(val, indent+4) // nested collection → sub-table
			} else {
				for _, item := range val {
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
			fmt.Printf("%s  %s\n", label, NumberStyle.Render(fmt.Sprintf("%v", val)))
		case string:
			fmt.Printf("%s  %s\n", label, renderStr(val))
		default:
			fmt.Printf("%s  %s\n", label, StringStyle.Render(fmt.Sprintf("%v", val)))
		}
	}
}

func renderStr(s string) string {
	if strings.HasPrefix(s, "0x") {
		return HexStyle.Render(s)
	}
	return StringStyle.Render(s)
}

// PrintStyled is exported for testing.
func PrintStyled(v any) error { return printStyled(v) }

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
