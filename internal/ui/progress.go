package ui

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ProgressBar draws a single-line download bar on a terminal:
//
//	Downloading rail0 1.4.0  ███████████░░░░░░░░░  56%  5.2/9.3 MB  2.1 MB/s
//
// It redraws in place with "\r" (on stderr, like the spinner, so stdout stays clean),
// at most every 100ms: a download delivers thousands of small reads, and redrawing on
// each one costs more than the copy. When the total is unknown (no Content-Length) the
// bar is replaced by the byte count alone.
type ProgressBar struct {
	w       io.Writer
	label   string
	width   int
	start   time.Time
	last    time.Time
	now     func() time.Time
	drawn   bool
	lastLen int
}

// NewProgressBar returns a bar that draws on w, prefixed with label.
func NewProgressBar(w io.Writer, label string) *ProgressBar {
	return &ProgressBar{w: w, label: label, width: 24, now: time.Now}
}

// Update records progress and redraws when due. It matches selfupdate.Progress.
func (p *ProgressBar) Update(done, total int64) {
	now := p.now()
	if p.start.IsZero() {
		p.start = now
	}
	finished := total > 0 && done >= total
	if p.drawn && !finished && now.Sub(p.last) < 100*time.Millisecond {
		return
	}
	p.last = now
	p.draw(done, total, now)
}

// Done ends the bar's line, leaving the final state on screen.
func (p *ProgressBar) Done() {
	if p.drawn {
		fmt.Fprintln(p.w)
	}
}

func (p *ProgressBar) draw(done, total int64, now time.Time) {
	var b strings.Builder
	b.WriteString(p.label)
	b.WriteString("  ")
	if total > 0 {
		frac := min(float64(done)/float64(total), 1)
		filled := int(frac * float64(p.width))
		b.WriteString(SpinStyle.Render(strings.Repeat("█", filled)))
		b.WriteString(DimStyle.Render(strings.Repeat("░", p.width-filled)))
		fmt.Fprintf(&b, " %3d%%  %s/%s", int(frac*100), megabytes(done), megabytes(total))
	} else {
		b.WriteString(megabytes(done))
	}
	b.WriteString(" MB")
	if elapsed := now.Sub(p.start).Seconds(); elapsed > 0.2 {
		fmt.Fprintf(&b, "  %s MB/s", megabytes(int64(float64(done)/elapsed)))
	}
	line := b.String()
	// Pad over whatever the previous, possibly longer, frame left behind.
	pad := max(p.lastLen-len(line), 0)
	fmt.Fprintf(p.w, "\r%s%s", line, strings.Repeat(" ", pad))
	p.lastLen = len(line)
	p.drawn = true
}

func megabytes(n int64) string {
	return fmt.Sprintf("%.1f", float64(n)/(1<<20))
}
