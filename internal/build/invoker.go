package build

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecInvoker is the real Invoker: it runs the php-esp32 fetch scripts and idf.py,
// sourcing ESP-IDF's export.sh so idf.py and the toolchain are on PATH. Out is where
// the subprocess output goes (default os.Stdout); build passes a *Progress here.
type ExecInvoker struct {
	PhpEsp32Dir string
	IdfPath     string
	Out         io.Writer
}

func (e ExecInvoker) out() io.Writer {
	if e.Out != nil {
		return e.Out
	}
	return os.Stdout
}

func shquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (e ExecInvoker) Fetch(script string) error {
	c := exec.Command(filepath.Join(e.PhpEsp32Dir, script))
	c.Dir = e.PhpEsp32Dir
	c.Stdout, c.Stderr = e.out(), e.out()
	return c.Run()
}

// Parttool runs ESP-IDF's parttool.py (partition read/write/erase) in the same env as idf.py.
func (e ExecInvoker) Parttool(args ...string) error {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shquote(a)
	}
	script := ". " + shquote(filepath.Join(e.IdfPath, "export.sh")) +
		" >/dev/null 2>&1 && parttool.py " + strings.Join(quoted, " ")
	c := exec.Command("bash", "-c", script)
	c.Dir = e.PhpEsp32Dir
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, e.out(), e.out()
	return c.Run()
}

func (e ExecInvoker) IDF(args ...string) error {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shquote(a)
	}
	script := ". " + shquote(filepath.Join(e.IdfPath, "export.sh")) +
		" >/dev/null 2>&1 && idf.py " + strings.Join(quoted, " ")
	c := exec.Command("bash", "-c", script)
	c.Dir = e.PhpEsp32Dir
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, e.out(), e.out()
	err := c.Run()
	// If output is being rendered as a progress bar, close its dangling line.
	if pg, ok := e.Out.(*Progress); ok {
		pg.Finish()
	}
	return err
}
