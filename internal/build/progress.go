package build

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// ninjaRe matches ninja's "[done/total] action" progress lines.
var ninjaRe = regexp.MustCompile(`^\[(\d+)/(\d+)\]`)

// Progress turns the raw idf.py build stream into tidy phases and a compile progress
// bar, while buffering the whole output so it can be replayed if the build fails.
// It is an io.Writer, so it's plugged in as the command's stdout/stderr sink.
type Progress struct {
	dst      io.Writer
	raw      bytes.Buffer
	partial  []byte
	total    int
	done     int
	phase    string
	finished bool
}

func NewProgress(dst io.Writer) *Progress { return &Progress{dst: dst} }

func (p *Progress) Write(b []byte) (int, error) {
	p.raw.Write(b) // keep everything for a possible error dump
	p.partial = append(p.partial, b...)
	for {
		i := bytes.IndexByte(p.partial, '\n')
		if i < 0 {
			break
		}
		p.line(strings.TrimRight(string(p.partial[:i]), "\r"))
		p.partial = p.partial[i+1:]
	}
	return len(b), nil
}

func (p *Progress) line(l string) {
	if m := ninjaRe.FindStringSubmatch(l); m != nil {
		p.done, _ = strconv.Atoi(m[1])
		p.total, _ = strconv.Atoi(m[2])
		p.setPhase("compiling")
		p.render()
		return
	}
	switch {
	case strings.Contains(l, "Building ESP-IDF components"),
		strings.Contains(l, "Running cmake"),
		strings.Contains(l, "Configuring done"),
		strings.Contains(l, "Generating done"):
		p.setPhase("configuring")
	case strings.Contains(l, "Generating binary image"),
		strings.Contains(l, "Creating esp32"),
		strings.Contains(l, "Successfully created"):
		p.setPhase("linking")
	}
}

func (p *Progress) setPhase(ph string) {
	if p.phase == ph {
		return
	}
	if p.phase == "compiling" && p.total > 0 {
		fmt.Fprintln(p.dst) // end the bar line before moving on
	}
	p.phase = ph
	if ph != "compiling" {
		fmt.Fprintf(p.dst, "  %s...\n", ph)
	}
}

func (p *Progress) render() {
	if p.total == 0 {
		return
	}
	pct := p.done * 100 / p.total
	const w = 28
	filled := pct * w / 100
	bar := strings.Repeat("#", filled) + strings.Repeat("-", w-filled)
	fmt.Fprintf(p.dst, "\r  compiling [%s] %3d%% (%d/%d)", bar, pct, p.done, p.total)
}

// Finish closes off a dangling progress-bar line with a newline (idempotent).
func (p *Progress) Finish() {
	if !p.finished && p.total > 0 {
		fmt.Fprintln(p.dst)
	}
	p.finished = true
}

// DumpRaw replays the full captured build output -- used when the build failed so the
// user sees the real error instead of just the progress bar.
func (p *Progress) DumpRaw(w io.Writer) {
	fmt.Fprintln(w, "\n--- full build output ---")
	w.Write(p.raw.Bytes())
}
