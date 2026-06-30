// Package ui provides terminal rendering for rail0-cli using the Charmbracelet
// library suite (lipgloss, bubbletea, bubbles).
package ui

import "charm.land/lipgloss/v2"

// Terminal 256-colour palette, shared by the colorized-JSON renderer (default)
// and the --pretty key-value/table view:
//
//	 81 — cyan for object keys (JSON view)
//	240 — muted gray for field labels (--pretty) and for null
//	 75 — sky blue for 0x-prefixed values (addresses, hashes, nonces)
//	252 — near-white for plain strings
//	215 — amber for numbers
//	 77 — medium green for boolean true
//	203 — coral for boolean false and errors
//	201 — magenta for any `status` field (uniform, see StatusStyle)
//	212 — pink for the loading spinner
//	238 — dim gray for the --pretty table separator
//
// LabelStyle.Width(22) pads every --pretty label to a fixed column so values
// align vertically. Increase the constant if a new response field exceeds 20
// chars.
var (
	KeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	LabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(22)
	HexStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	StringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	NumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	TrueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	FalseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	NullStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SpinStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	HeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)
	DimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// StatusStyle renders a resource's `status` value. Every status — for any
// resource and any value — is shown in the same magenta, so a status field
// always reads consistently at a glance, regardless of its value.
func StatusStyle(_ string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
}
