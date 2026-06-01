// Package ui provides terminal rendering for rail0-cli using the Charmbracelet
// library suite (lipgloss, bubbletea, bubbles).
package ui

import "charm.land/lipgloss/v2"

// Terminal 256-colour palette:
//   240 — muted gray for field labels
//    75 — sky blue for 0x-prefixed values (addresses, hashes, nonces)
//   252 — near-white for plain strings
//   215 — amber for numbers
//    82 — green for boolean true
//   203 — coral for boolean false and errors
//   212 — pink for the loading spinner
//
// LabelStyle.Width(22) pads every label to a fixed column so values align
// vertically. Increase the constant if a new response field exceeds 20 chars.
var (
	LabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(22)
	HexStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	StringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	NumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	TrueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	FalseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	ErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SpinStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	HeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)
	DimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// StatusStyle returns a style based on the payment status value.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "authorized", "captured", "charged", "refunded":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	case "partially_captured", "partially_refunded":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	case "signed", "unsigned":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	case "submitted":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	case "confirmed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	}
}
