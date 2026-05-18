package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Print writes v as indented JSON to stdout.
func Print(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

// Fatal prints err to stderr and exits with code 1.
func Fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
