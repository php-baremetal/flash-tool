package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Option struct {
	Label    string
	Disabled bool
}

type Prompter interface {
	Input(label, def string) (string, error)
	Confirm(label string, def bool) (bool, error)
	Select(label string, options []Option, def int) (int, error)
	MultiSelect(label string, options []Option) ([]int, error)
}

type Terminal struct {
	In  io.Reader
	Out io.Writer

	reader *bufio.Reader
}

func (t *Terminal) line() (string, error) {
	if t.reader == nil {
		t.reader = bufio.NewReader(t.In)
	}
	s, err := t.reader.ReadString('\n')
	return strings.TrimSpace(s), err
}

func (t *Terminal) Input(label, def string) (string, error) {
	fmt.Fprintf(t.Out, "%s [%s]: ", label, def)
	s, _ := t.line()
	if s == "" {
		return def, nil
	}
	return s, nil
}

func (t *Terminal) Confirm(label string, def bool) (bool, error) {
	d := "y/N"
	if def {
		d = "Y/n"
	}
	fmt.Fprintf(t.Out, "%s [%s]: ", label, d)
	s, _ := t.line()
	switch strings.ToLower(s) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (t *Terminal) Select(label string, options []Option, def int) (int, error) {
	fmt.Fprintln(t.Out, label+":")
	for i, o := range options {
		mark := ""
		if o.Disabled {
			mark = " (unavailable)"
		}
		fmt.Fprintf(t.Out, "  %d) %s%s\n", i+1, o.Label, mark)
	}
	fmt.Fprintf(t.Out, "choice [%d]: ", def+1)
	s, _ := t.line()
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > len(options) || options[n-1].Disabled {
		return def, nil
	}
	return n - 1, nil
}

func (t *Terminal) MultiSelect(label string, options []Option) ([]int, error) {
	fmt.Fprintln(t.Out, label+" (comma-separated numbers, empty = none):")
	for i, o := range options {
		fmt.Fprintf(t.Out, "  %d) %s\n", i+1, o.Label)
	}
	fmt.Fprint(t.Out, "choices: ")
	s, _ := t.line()
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= len(options) {
			out = append(out, n-1)
		}
	}
	return out, nil
}
