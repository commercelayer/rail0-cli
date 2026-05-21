package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// fetchResult carries the outcome of an async API call back to the fetchModel.
type fetchResult struct {
	data any
	err  error
}

// fetchModel is a minimal bubbletea model that shows a spinner while an API
// call runs in a goroutine, then quits as soon as the result arrives.
type fetchModel struct {
	spinner spinner.Model
	fn      func() (any, error)
	result  *fetchResult
}

func newFetchModel(fn func() (any, error)) fetchModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return fetchModel{spinner: s, fn: fn}
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
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m fetchModel) View() string {
	if m.result != nil {
		return ""
	}
	return SpinStyle.Render(m.spinner.View()) + " fetching…\n"
}
