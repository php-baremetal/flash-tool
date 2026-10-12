package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"phpflash/internal/discover"
)

// writeFakeBuild lays out build/compiled/flasher_args.json + the referenced image files (sized to the
// given byte counts), mirroring an ESP-IDF build tree, and returns the buildDir.
func writeFakeBuild(t *testing.T, files map[string]struct {
	off  string
	size int
}) string {
	t.Helper()
	buildDir := t.TempDir()
	compiled := filepath.Join(buildDir, "compiled")
	if err := os.MkdirAll(filepath.Join(compiled, "bootloader"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(compiled, "partition_table"), 0o755); err != nil {
		t.Fatal(err)
	}
	flashFiles := "{\n"
	first := true
	for rel, f := range files {
		if err := os.WriteFile(filepath.Join(compiled, rel), make([]byte, f.size), 0o644); err != nil {
			t.Fatal(err)
		}
		if !first {
			flashFiles += ",\n"
		}
		first = false
		flashFiles += "    \"" + f.off + "\": \"" + rel + "\""
	}
	flashFiles += "\n  }"
	json := "{ \"flash_files\": " + flashFiles + " }\n"
	if err := os.WriteFile(filepath.Join(compiled, "flasher_args.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	return buildDir
}

func TestRequiredFlashBytes(t *testing.T) {
	buildDir := writeFakeBuild(t, map[string]struct {
		off  string
		size int
	}{
		"bootloader/bootloader.bin":           {"0x0", 20000},
		"partition_table/partition-table.bin": {"0x8000", 3000},
		"php-esp32.bin":                       {"0x10000", 100000},  // ends at 0x10000+100000
		"storage.bin":                         {"0x370000", 131072}, // the highest: 0x370000 + 131072
	})
	got := requiredFlashBytes(buildDir)
	want := int64(0x370000 + 131072)
	if got != want {
		t.Errorf("requiredFlashBytes = %d, want %d", got, want)
	}

	// Missing / unreadable build dir -> 0 (lenient).
	if got := requiredFlashBytes(t.TempDir()); got != 0 {
		t.Errorf("requiredFlashBytes(empty) = %d, want 0", got)
	}
}

func TestCheckFlashFits(t *testing.T) {
	// storage at 0x370000 + 128K ends at ~3.6MB -> fits 4MB, not 2MB.
	buildDir := writeFakeBuild(t, map[string]struct {
		off  string
		size int
	}{
		"bootloader/bootloader.bin": {"0x0", 20000},
		"php-esp32.bin":             {"0x10000", 100000},
		"storage.bin":               {"0x370000", 131072},
	})
	bc := &buildContext{buildDir: buildDir}

	if err := checkFlashFits(bc, discover.ChipInfo{Flash: "4MB"}, io.Discard); err != nil {
		t.Errorf("4MB should fit: %v", err)
	}
	if err := checkFlashFits(bc, discover.ChipInfo{Flash: "2MB"}, io.Discard); err == nil {
		t.Error("2MB should NOT fit, expected an error")
	}
	// No probe info -> lenient (no error).
	if err := checkFlashFits(bc, discover.ChipInfo{}, io.Discard); err != nil {
		t.Errorf("empty ChipInfo should be lenient: %v", err)
	}
}
