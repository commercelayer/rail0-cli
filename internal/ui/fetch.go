package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// minSpinner keeps the spinner on screen for at least this long, so a fast API
// call still shows a visible "working" indicator instead of flashing by (or
// never painting a frame at all).
const minSpinner = 400 * time.Millisecond

// fetchResult carries the outcome of an async API call back to the fetchModel.
type fetchResult struct {
	data any
	err  error
}

// doneMsg is delivered once both the result has arrived and minSpinner has
// elapsed, at which point the model quits.
type doneMsg struct{}

// fetchModel is a minimal bubbletea model that shows a spinner while an API
// call runs in a goroutine, keeps it up for at least minSpinner, then quits.
type fetchModel struct {
	spinner  spinner.Model
	fn       func() (any, error)
	msg      string
	start    time.Time
	result   *fetchResult
	quitting bool
}

func newFetchModel(fn func() (any, error), msg string) fetchModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	if msg == "" {
		msg = "fetching…"
	}
	return fetchModel{spinner: s, fn: fn, msg: msg, start: time.Now()}
}

func (m fetchModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			data, err := m.fn()
			return fetchResult{data: data, err: err}
		},
	)
}

func (m fetchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchResult:
		m.result = &msg
		// Hold the spinner until minSpinner has elapsed so it's always perceptible;
		// the spinner keeps animating via its own tick in the meantime.
		if remaining := minSpinner - time.Since(m.start); remaining > 0 {
			return m, tea.Tick(remaining, func(time.Time) tea.Msg { return doneMsg{} })
		}
		m.quitting = true
		return m, tea.Quit
	case doneMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m fetchModel) View() string {
	if m.quitting {
		return ""
	}
	return SpinStyle.Render(m.spinner.View()) + " " + m.msg + "\n"
}
