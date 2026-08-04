package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListPortsOrdersAcmThenUsbThenNumber(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"ttyUSB0", "ttyACM1", "ttyACM0"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	globs := []string{filepath.Join(dir, "ttyACM*"), filepath.Join(dir, "ttyUSB*")}
	got := listPorts(globs)
	want := []string{
		filepath.Join(dir, "ttyACM0"),
		filepath.Join(dir, "ttyACM1"),
		filepath.Join(dir, "ttyUSB0"),
	}
	if len(got) != len(want) {
		t.Fatalf("listPorts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("listPorts[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListPortsEmpty(t *testing.T) {
	if got := listPorts([]string{filepath.Join(t.TempDir(), "ttyACM*")}); len(got) != 0 {
		t.Errorf("expected no ports, got %v", got)
	}
}
