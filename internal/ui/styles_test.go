package ui

import (
	"reflect"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestStatusStyle asserts that StatusStyle maps a status to the intended
// foreground colour, mirroring the rail0-admin palette (green=settled, blue=
// authorized, indigo=charged, orange=refunded, rose=voided, gray=released/closed,
// yellow=in-flight, red=failed).
func TestStatusStyle(t *testing.T) {
	cases := []struct {
		status string
		want   string // 256-colour palette index
	}{
		{"captured", "42"},   // settled success
		{"confirmed", "42"},  // transaction success
		{"authorized", "39"}, // funds held (blue)
		{"charged", "99"},    // one-shot paid (indigo)
		{"refunded", "214"},  // funds returned (orange)
		{"voided", "204"},    // cancelled (rose)
		{"released", "244"},  // neutral terminal (gray)
		{"submitted", "220"}, // in flight (yellow)
		{"failed", "203"},    // error state (red)
		{"", "244"},          // unknown → default gray
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
