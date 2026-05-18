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
func SetJSON(on bool) { jsonMode = on }

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
			fmt.Printf("%s  %s\n", label, numStyle.Render(fmt.Sprintf("%v", val)))
		case string:
			fmt.Printf("%s  %s\n", label, renderStr(val))
		default:
			fmt.Printf("%s  %s\n", label, strStyle.Render(fmt.Sprintf("%v", val)))
		}
	}
}

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
