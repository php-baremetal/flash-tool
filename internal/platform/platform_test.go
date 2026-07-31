package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPortPrefersFirstPatternThenLowestNumber(t *testing.T) {
	dir := t.TempDir()
	// Two ACM nodes and a USB node; ACM0 must win (first pattern, lowest number).
	for _, n := range []string{"ttyACM1", "ttyACM0", "ttyUSB0"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	globs := []string{filepath.Join(dir, "ttyACM*"), filepath.Join(dir, "ttyUSB*")}
	if got := detectPort(globs); got != filepath.Join(dir, "ttyACM0") {
		t.Errorf("detectPort = %q, want ttyACM0", got)
	}
}

func TestDetectPortFallsBackToSecondPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ttyUSB0"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	globs := []string{filepath.Join(dir, "ttyACM*"), filepath.Join(dir, "ttyUSB*")}
	if got := detectPort(globs); got != filepath.Join(dir, "ttyUSB0") {
		t.Errorf("detectPort = %q, want ttyUSB0", got)
	}
}

func TestDetectPortEmptyWhenNoDevice(t *testing.T) {
	dir := t.TempDir()
	globs := []string{filepath.Join(dir, "ttyACM*"), filepath.Join(dir, "ttyUSB*")}
	if got := detectPort(globs); got != "" {
		t.Errorf("detectPort = %q, want empty", got)
	}
}
