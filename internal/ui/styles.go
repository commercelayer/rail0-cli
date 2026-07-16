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
//	 82 — green for a terminal success status (see StatusStyle)
//	226 — bright yellow for a non-terminal `status` (high-visibility); terminal states use 82/203 (see StatusStyle)
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

// StatusStyle renders a resource's `status` value with a colour that mirrors the
// rail0-admin palette, so the CLI and the dashboard read the same at a glance:
//
//	green  — captured / partially_captured / confirmed / active (settled OK)
//	blue   — authorized (funds held in escrow)
//	indigo — charged (one-shot paid)
//	orange — refunded / partially_refunded (funds returned)
//	rose   — voided (authorization cancelled)
//	amber  — open / disputed (needs attention)
//	gray   — released / closed (neutral terminal)
//	yellow — in flight (unsigned / signed / submitting / submitted / pending)
//	red    — failed (error)
func StatusStyle(status string) lipgloss.Style {
	fg := func(code string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(code)) }
	switch status {
	case "captured", "partially_captured", "confirmed", "active":
		return fg("42") // green
	case "authorized":
		return fg("39") // blue
	case "charged":
		return fg("99") // indigo
	case "refunded", "partially_refunded":
		return fg("214") // orange
	case "voided":
		return fg("204") // rose
	case "open", "disputed":
		return fg("208") // amber — attention
	case "released", "closed":
		return fg("244") // gray — neutral terminal
	case "unsigned", "signed", "submitting", "submitted", "pending":
		return fg("220") // yellow — in flight
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true) // red
	default:
		return fg("244") // gray
	}
}
