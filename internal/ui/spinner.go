package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// spinnerFrames drives the animation. The Unicode set is the braille pattern
// cycle, which is a single column wide and so never shifts the text after it.
var (
	spinnerFramesUnicode = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerFramesASCII   = []string{"|", "/", "-", "\\"}
)

// spinnerInterval is the frame rate. 80ms reads as smooth without spending
// meaningful CPU on redraws.
const spinnerInterval = 80 * time.Millisecond

// Spinner shows indeterminate progress on an interactive terminal.
//
// When output is not a terminal the spinner degrades to a single line printed
// at start and a result line at stop, so CI logs stay readable rather than
// filling with escape sequences.
type Spinner struct {
	r        *Renderer
	message  string
	frames   []string
	animated bool

	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	stopped bool
	// lastWidth tracks how many columns the previous frame occupied so the
	// erase sequence clears exactly that much.
	lastWidth int
}

// Spin starts a spinner with the given message. The caller must call one of
// Stop, Succeed, or Fail on the returned spinner, conventionally by defer.
func (r *Renderer) Spin(format string, args ...any) *Spinner {
	message := fmt.Sprintf(format, args...)

	s := &Spinner{
		r:        r,
		message:  message,
		frames:   spinnerFramesASCII,
		animated: r.color && isTerminal(r.err) && !r.quiet,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	if r.unicode {
		s.frames = spinnerFramesUnicode
	}

	r.mu.Lock()
	r.activeSpn = s
	r.mu.Unlock()

	if !s.animated {
		// Non-interactive: announce once, no animation.
		if !r.quiet {
			r.mu.Lock()
			fmt.Fprintf(r.err, "%s %s\n", r.Symbols().Pending, message)
			r.mu.Unlock()
		}
		close(s.doneCh)
		return s
	}

	go s.run()
	return s
}

// run drives the animation loop until stopped.
func (s *Spinner) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.draw(s.frames[frame%len(s.frames)])
			frame++
		}
	}
}

// draw renders one frame, erasing the previous one first.
func (s *Spinner) draw(glyph string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}

	line := fmt.Sprintf("%s %s", s.r.Apply(StyleInfo, glyph), s.message)

	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	// Carriage return, write, then clear to end of line. Clearing after the
	// write rather than before avoids a visible flicker on slow terminals.
	fmt.Fprintf(s.r.err, "\r%s\x1b[K", line)
	s.lastWidth = displayWidth(line)
}

// erase clears the spinner line so a final message can take its place.
func (s *Spinner) erase() {
	if !s.animated {
		return
	}
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	fmt.Fprintf(s.r.err, "\r%s\r", strings.Repeat(" ", s.lastWidth))
}

// Update changes the message shown alongside the animation.
func (s *Spinner) Update(format string, args ...any) {
	s.mu.Lock()
	s.message = fmt.Sprintf(format, args...)
	s.mu.Unlock()
}

// halt stops the animation loop and clears the line exactly once.
func (s *Spinner) halt() bool {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return false
	}
	s.stopped = true
	s.mu.Unlock()

	if s.animated {
		close(s.stopCh)
		<-s.doneCh
	}
	s.erase()

	s.r.mu.Lock()
	if s.r.activeSpn == s {
		s.r.activeSpn = nil
	}
	s.r.mu.Unlock()
	return true
}

// Stop halts the spinner without printing a result line. Safe to call twice,
// which makes `defer spinner.Stop()` correct even after an explicit Succeed.
func (s *Spinner) Stop() { s.halt() }

// Succeed halts the spinner and reports success.
func (s *Spinner) Succeed(format string, args ...any) {
	if s.halt() {
		s.r.Success(format, args...)
	}
}

// Fail halts the spinner and reports failure.
func (s *Spinner) Fail(format string, args ...any) {
	if s.halt() {
		s.r.Failure(format, args...)
	}
}

// Progress renders a determinate progress bar for a known total.
//
// Like Spinner it degrades on non-terminals, where it prints a line at each
// decile rather than redrawing, keeping CI output bounded.
type Progress struct {
	r       *Renderer
	label   string
	total   int
	current int
	width   int
	// animated selects in-place redraw over milestone lines.
	animated bool
	// lastDecile tracks the last milestone reported in non-animated mode.
	lastDecile int
	mu         sync.Mutex
}

// NewProgress creates a progress bar for total units of work.
func (r *Renderer) NewProgress(label string, total int) *Progress {
	return &Progress{
		r:          r,
		label:      label,
		total:      total,
		width:      30,
		animated:   r.color && isTerminal(r.err) && !r.quiet,
		lastDecile: -1,
	}
}

// Increment advances the bar by n units and redraws.
func (p *Progress) Increment(n int) {
	p.mu.Lock()
	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	current, total := p.current, p.total
	p.mu.Unlock()

	p.render(current, total)
}

// render draws the bar, or reports a milestone on a non-interactive stream.
func (p *Progress) render(current, total int) {
	if total <= 0 || p.r.quiet {
		return
	}
	percent := current * 100 / total

	if !p.animated {
		decile := percent / 10
		p.mu.Lock()
		report := decile > p.lastDecile
		if report {
			p.lastDecile = decile
		}
		p.mu.Unlock()
		if report {
			p.r.mu.Lock()
			fmt.Fprintf(p.r.err, "%s %d%% (%d/%d)\n", p.label, percent, current, total)
			p.r.mu.Unlock()
		}
		return
	}

	bar := p.r.Bar(float64(current), float64(total), p.width, StyleInfo)
	p.r.mu.Lock()
	fmt.Fprintf(p.r.err, "\r%s %s %3d%% (%d/%d)\x1b[K", p.label, bar, percent, current, total)
	p.r.mu.Unlock()
}

// Done finalises the bar, leaving the cursor on a fresh line.
func (p *Progress) Done() {
	if p.animated && !p.r.quiet {
		p.r.mu.Lock()
		fmt.Fprint(p.r.err, "\r\x1b[K")
		p.r.mu.Unlock()
	}
}
