package ui

import (
	"reflect"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestStatusStyle asserts that StatusStyle maps a status to the intended
// foreground colour: terminal-success states are green (82), the failed state
// is red (203), and any non-terminal state is high-visibility bright yellow (226).
func TestStatusStyle(t *testing.T) {
	cases := []struct {
		status string
		want   string // 256-colour palette index
	}{
		{"confirmed", "82"},   // terminal success (transaction)
		{"released", "82"},    // terminal success (payment closed state)
		{"failed", "203"},     // error state
		{"authorized", "226"}, // non-terminal / in-progress
		{"", "226"},           // unknown → default non-terminal
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			got := StatusStyle(tc.status).GetForeground()
			want := lipgloss.Color(tc.want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("StatusStyle(%q) foreground = %#v, want %#v", tc.status, got, want)
			}
		})
	}
}
