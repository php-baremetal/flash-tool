package setup

import (
	"bytes"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls   []string
	present map[string]bool
}

func (f *fakeRunner) Git(args ...string) error {
	f.calls = append(f.calls, "git "+strings.Join(args, " "))
	return nil
}
func (f *fakeRunner) Shell(dir, script string, args ...string) error {
	f.calls = append(f.calls, "sh "+dir+" "+script+" "+strings.Join(args, " "))
	return nil
}
func (f *fakeRunner) Exists(path string) bool { return f.present[path] }

func TestRunClonesWhenAbsentAndRunsSteps(t *testing.T) {
	r := &fakeRunner{present: map[string]bool{}}
	var out bytes.Buffer
	plan := Plan{
		IdfPath: "/idf", IdfVersion: "v5.5.5",
		PhpEsp32Path: "/php", PhpEsp32Version: "",
	}
	if err := Run(r, &out, plan); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(r.calls, "\n")
	if !strings.Contains(joined, "git clone") || !strings.Contains(joined, "/idf") {
		t.Errorf("expected an idf clone, got:\n%s", joined)
	}
	if !strings.Contains(joined, "install.sh") {
		t.Errorf("expected install.sh, got:\n%s", joined)
	}
	if !strings.Contains(joined, "fetch-php.sh") {
		t.Errorf("expected fetch-php.sh, got:\n%s", joined)
	}
	if strings.Index(joined, "/idf") > strings.Index(joined, "fetch-php.sh") {
		t.Errorf("idf setup should precede php-esp32 setup:\n%s", joined)
	}
}

func TestRunChecksOutWhenPresent(t *testing.T) {
	r := &fakeRunner{present: map[string]bool{"/idf": true, "/php": true}}
	var out bytes.Buffer
	if err := Run(r, &out, Plan{IdfPath: "/idf", IdfVersion: "v5.5.5", PhpEsp32Path: "/php"}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(r.calls, "\n")
	if strings.Contains(joined, "git clone") {
		t.Errorf("should checkout, not clone, when present:\n%s", joined)
	}
	if !strings.Contains(joined, "checkout v5.5.5") {
		t.Errorf("expected idf checkout of the version:\n%s", joined)
	}
}
