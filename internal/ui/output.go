package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"charm.land/lipgloss/v2"
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
		label := LabelStyle.Render(pad + k)
		switch val := m[k].(type) {
		case map[string]any:
			fmt.Println(label)
			printMap(val, indent+4)
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
