package build

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"phpflash/internal/config"
	"phpflash/internal/manifest"
)

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		ProjectTypes: []manifest.Mode{{Key: "init-loop", Available: true}},
		Extensions: []manifest.Extension{
			{Key: "date", Flag: "PHP_EXT_DATE=ON", Settings: []manifest.Setting{
				{Key: "minimal_tz", Flag: "PHP_EXT_DATE_MINIMAL_TZ=ON"},
			}},
			{Key: "sqlite", Flag: "PHP_EXT_SQLITE=ON", Fetch: "scripts/fetch-sqlite.sh"},
		},
	}
}

func TestArgsDeterministic(t *testing.T) {
	cfg := &config.Config{
		Type:  "init-loop",
		Board: config.BoardConfig{Target: "esp32-p4-pico"},
		Extensions: map[string]config.Extension{
			"sqlite": {Enabled: true},
		},
	}
	dargs, fetches, err := Args(cfg, testManifest(), "8.3.32")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-DBOARD=esp32-p4-pico", "-DPHP_VERSION=8.3.32",
		"-DPHP_EXT_DATE=OFF", "-DPHP_EXT_DATE_MINIMAL_TZ=OFF", "-DPHP_EXT_SQLITE=ON",
	}
	if !reflect.DeepEqual(dargs, want) {
		t.Errorf("dargs = %v\nwant %v", dargs, want)
	}
	if !reflect.DeepEqual(fetches, []string{"scripts/fetch-sqlite.sh"}) {
		t.Errorf("fetches = %v", fetches)
	}
}

type fakeInvoker struct {
	fetches []string
	idf     [][]string
}

func (f *fakeInvoker) Fetch(s string) error       { f.fetches = append(f.fetches, s); return nil }
func (f *fakeInvoker) IDF(args ...string) error   { f.idf = append(f.idf, args); return nil }

func TestBuildRunsFetchesThenIDF(t *testing.T) {
	f := &fakeInvoker{}
	if err := Build(f, io.Discard, "/proj/build", "/proj/build/sdkconfig", []string{"-DBOARD=x"}, []string{"scripts/fetch-sqlite.sh"}); err != nil {
		t.Fatal(err)
	}
	if len(f.fetches) != 1 || f.fetches[0] != "scripts/fetch-sqlite.sh" {
		t.Errorf("fetches = %v", f.fetches)
	}
	want := []string{"-B", "/proj/build/compiled", "-DSDKCONFIG=/proj/build/sdkconfig", "-DBOARD=x", "build"}
	if len(f.idf) != 1 || !reflect.DeepEqual(f.idf[0], want) {
		t.Errorf("idf call = %v, want %v", f.idf, want)
	}
}

func TestFlashAppendsPortAndFlash(t *testing.T) {
	f := &fakeInvoker{}
	if err := Flash(f, io.Discard, "/proj/build", []string{"-DBOARD=x"}, "/dev/ttyACM0"); err != nil {
		t.Fatal(err)
	}
	got := f.idf[0]
	want := []string{"-B", "/proj/build/compiled", "-DBOARD=x", "-p", "/dev/ttyACM0", "flash"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("flash args = %v, want %v", got, want)
	}
}

func TestMonitorNoPort(t *testing.T) {
	f := &fakeInvoker{}
	if err := Monitor(f, io.Discard, "/proj/build", ""); err != nil {
		t.Fatal(err)
	}
	want := []string{"-B", "/proj/build/compiled", "monitor"}
	if !reflect.DeepEqual(f.idf[0], want) {
		t.Errorf("monitor args = %v, want %v", f.idf[0], want)
	}
}

func TestExtractBins(t *testing.T) {
	buildDir := t.TempDir()
	compiled := filepath.Join(buildDir, "compiled")
	// fake a built ESP-IDF tree
	os.MkdirAll(filepath.Join(compiled, "bootloader"), 0o755)
	os.MkdirAll(filepath.Join(compiled, "partition_table"), 0o755)
	os.WriteFile(filepath.Join(compiled, "php-esp32.bin"), []byte("app"), 0o644)
	os.WriteFile(filepath.Join(compiled, "bootloader", "bootloader.bin"), []byte("bl"), 0o644)
	os.WriteFile(filepath.Join(compiled, "partition_table", "partition-table.bin"), []byte("pt"), 0o644)

	bins, err := extractBins(compiled, buildDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != 3 {
		t.Fatalf("extracted %d bins, want 3: %v", len(bins), bins)
	}
	for _, name := range []string{"php-esp32.bin", "bootloader.bin", "partition-table.bin"} {
		if _, err := os.Stat(filepath.Join(buildDir, name)); err != nil {
			t.Errorf("missing extracted %s: %v", name, err)
		}
	}
}
