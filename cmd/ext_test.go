package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldExt(t *testing.T) {
	dir := t.TempDir()

	file, err := scaffoldExt(dir, "sensorhub", false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "firmware", "exts", "sensorhub", "sensorhub.c")
	if file != want {
		t.Errorf("path = %q, want %q", file, want)
	}
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"zend_module_entry sensorhub_module_entry",
		"PHP_FUNCTION(sensorhub_hello)",
		"PHP_FUNCTION(sensorhub_add)",
		`"sensorhub"`,
	} {
		if !strings.Contains(string(src), s) {
			t.Errorf("generated source missing %q", s)
		}
	}
}

func TestScaffoldExtRejectsBadName(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"Bad-Name", "9lives", "has space", "UPPER", ""} {
		if _, err := scaffoldExt(dir, bad, false); err == nil {
			t.Errorf("name %q was accepted, want rejected", bad)
		}
	}
}

func TestScaffoldExtExistingNeedsForce(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldExt(dir, "demo", false); err != nil {
		t.Fatal(err)
	}
	if _, err := scaffoldExt(dir, "demo", false); err == nil {
		t.Error("second scaffold without --force succeeded, want error")
	}
	if _, err := scaffoldExt(dir, "demo", true); err != nil {
		t.Errorf("scaffold with force failed: %v", err)
	}
}
