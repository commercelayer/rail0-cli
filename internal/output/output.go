// Package output handles terminal rendering for rail0-cli.
//
// On a TTY it uses lipgloss to produce styled, human-readable key-value output.
// When stdout is redirected (pipe, file) or --json is set it falls back to
// plain indented JSON so the output remains machine-parseable.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

var jsonMode bool

// SetJSON forces plain JSON output regardless of TTY state.
// Called by the root command after it parses the --json flag.
func SetJSON(on bool) { jsonMode = on }

// Style palette. Terminal 256-colour codes:
//   240 — muted gray for labels
//    75 — sky blue for 0x-prefixed hex values (addresses, hashes, nonces)
//   252 — near-white for plain strings
//   215 — amber for numbers
//    82 — green for boolean true
//   203 — coral for boolean false and errors
//
// keyStyle.Width(22) pads every label to a fixed column so values align
// vertically. Increase this constant if a new response field exceeds 20 chars.
var (
	keyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(22)
	hexStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	strStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	numStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	trueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	falseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	errMark    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true).Render("✗")
	errBody    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func isTerminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// Print outputs v as styled key-value text on a TTY, or as indented JSON otherwise.
func Print(v any) error {
	if jsonMode || !isTerminal(os.Stdout) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal output: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}
	// Round-trip through JSON to get a generic map so we can walk the
	// structure without importing every response type from the SDK.
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		fmt.Println(string(b))
		return nil
	}
	fmt.Println()
	printMap(m, 2)
	fmt.Println()
	return nil
}

// printMap renders a JSON object as indented key-value rows. Nested objects are
// printed recursively with an additional 4-space indent per level.
func printMap(m map[string]any, indent int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		label := keyStyle.Render(pad + k)
		switch val := m[k].(type) {
		case map[string]any:
			fmt.Println(label)
			printMap(val, indent+4)
		case bool:
			if val {
				fmt.Printf("%s  %s\n", label, trueStyle.Render("true"))
			} else {
				fmt.Printf("%s  %s\n", label, falseStyle.Render("false"))
			}
		case float64:
			// JSON numbers unmarshal as float64; format without trailing ".0"
			// for integers (all RAIL0 numeric fields are integers).
			fmt.Printf("%s  %s\n", label, numStyle.Render(fmt.Sprintf("%v", val)))
		case string:
			fmt.Printf("%s  %s\n", label, renderStr(val))
		default:
			fmt.Printf("%s  %s\n", label, strStyle.Render(fmt.Sprintf("%v", val)))
		}
	}
}

// renderStr applies hex colouring to 0x-prefixed strings (addresses, hashes, nonces).
func renderStr(s string) string {
	if strings.HasPrefix(s, "0x") {
		return hexStyle.Render(s)
	}
	return strStyle.Render(s)
}

// Fatal prints a styled error to stderr and exits with code 1.
func Fatal(err error) {
	if isTerminal(os.Stderr) {
		fmt.Fprintf(os.Stderr, "\n  %s  %s\n\n", errMark, errBody.Render(err.Error()))
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(1)
}
