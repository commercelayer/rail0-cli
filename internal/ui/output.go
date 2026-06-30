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

var jsonMode bool

// SetJSON forces plain JSON output regardless of TTY state.
func SetJSON(on bool) { jsonMode = on }

func isTerminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// RunAPI executes fn and prints its result as JSON. On a TTY it shows a
// bubbletea spinner on stderr while the call is in flight, then prints
// syntax-highlighted JSON to stdout. When stdout is piped or --json is set it
// calls fn directly and emits plain indented JSON (no ANSI), suitable for jq
// and shell pipelines. A nil result (e.g. a 204 delete) prints nothing.
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

// printStyled prints v as colorized, indented JSON for a terminal. It marshals
// v (a typed SDK value) to JSON, decodes it into a generic tree, then walks the
// tree emitting JSON syntax with the package styles applied to keys and scalar
// values. Object keys are ordered by sortFields (identity first, timestamps
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

// renderObject renders a JSON object across multiple lines with sorted keys.
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
		b.WriteString(KeyStyle.Render(quote(k)))
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
