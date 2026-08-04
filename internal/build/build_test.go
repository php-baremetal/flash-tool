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
		"-DPHP_STORAGE_MICROSD=ON",
	}
	if !reflect.DeepEqual(dargs, want) {
		t.Errorf("dargs = %v\nwant %v", dargs, want)
	}
	if !reflect.DeepEqual(fetches, []string{"scripts/fetch-sqlite.sh"}) {
		t.Errorf("fetches = %v", fetches)
	}
}

func TestArgsProjectTypeFlag(t *testing.T) {
	m := &manifest.Manifest{
		ProjectTypes: []manifest.Mode{
			{Key: "init-loop", Available: true},
			{Key: "web-server", Available: true, Flag: "PHP_PROJECT_WEB_SERVER=ON"},
		},
		Extensions: []manifest.Extension{},
	}
	// web-server selected -> flag ON
	ws := &config.Config{Type: "web-server", Board: config.BoardConfig{Target: "esp32-p4-eth"}}
	dargs, _, err := Args(ws, m, "8.3.32")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(dargs, "-DPHP_PROJECT_WEB_SERVER=ON") {
		t.Errorf("web-server: want PHP_PROJECT_WEB_SERVER=ON, got %v", dargs)
	}
	// init-loop selected -> the same flag explicitly OFF (deterministic)
	il := &config.Config{Type: "init-loop", Board: config.BoardConfig{Target: "esp32-p4-pico"}}
	dargs, _, _ = Args(il, m, "8.3.32")
	if !contains(dargs, "-DPHP_PROJECT_WEB_SERVER=OFF") {
		t.Errorf("init-loop: want PHP_PROJECT_WEB_SERVER=OFF, got %v", dargs)
	}
}

func TestArgsMicrosdFlag(t *testing.T) {
	m := &manifest.Manifest{
		ProjectTypes: []manifest.Mode{{Key: "init-loop", Available: true}},
		Extensions:   []manifest.Extension{},
	}
	cases := []struct {
		name    string
		cfg     *config.Config
		wantOn  bool
	}{
		{"microsd type -> ON", &config.Config{Type: "init-loop", StorageType: "microsd"}, true},
		{"embedded default -> OFF", &config.Config{Type: "init-loop", StorageType: "embedded"}, false},
		{"embedded opt-in -> ON", &config.Config{Type: "init-loop", StorageType: "embedded", Storage: config.StorageConfig{Microsd: true}}, true},
		{"unset -> ON (safe default)", &config.Config{Type: "init-loop"}, true},
	}
	for _, c := range cases {
		dargs, _, err := Args(c.cfg, m, "8.3.32")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		want := "-DPHP_STORAGE_MICROSD=OFF"
		if c.wantOn {
			want = "-DPHP_STORAGE_MICROSD=ON"
		}
		if !contains(dargs, want) {
			t.Errorf("%s: want %q in %v", c.name, want, dargs)
		}
	}
}

func TestEnsureOpenSSLConf(t *testing.T) {
	base := &config.Config{Php: config.PhpConfig{Src: "project-src"}}
	full := func(extra map[string]bool) *config.Config {
		s := map[string]bool{"full": true}
		for k, v := range extra {
			s[k] = v
		}
		c := *base
		c.Extensions = map[string]config.Extension{"openssl": {Enabled: true, Settings: s}}
		return &c
	}
	cnf := func(dir string) string { return filepath.Join(dir, "project-src", "openssl.cnf") }

	// full openssl -> writes openssl.cnf
	d1 := t.TempDir()
	if err := EnsureOpenSSLConf(full(nil), d1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cnf(d1)); err != nil {
		t.Errorf("full: openssl.cnf not written: %v", err)
	}

	// full + no_load_config -> no file
	d2 := t.TempDir()
	_ = EnsureOpenSSLConf(full(map[string]bool{"no_load_config": true}), d2)
	if _, err := os.Stat(cnf(d2)); err == nil {
		t.Errorf("no_load_config: openssl.cnf should not be written")
	}

	// subset (no full) -> no file
	d3 := t.TempDir()
	sub := *base
	sub.Extensions = map[string]config.Extension{"openssl": {Enabled: true}}
	_ = EnsureOpenSSLConf(&sub, d3)
	if _, err := os.Stat(cnf(d3)); err == nil {
		t.Errorf("subset: openssl.cnf should not be written")
	}

	// custom relative config_path -> file written at that path under project-src
	d4 := t.TempDir()
	c4 := *base
	c4.Extensions = map[string]config.Extension{"openssl": {Enabled: true,
		Settings: map[string]bool{"full": true}, Options: map[string]string{"config_path": "etc/ssl.cnf"}}}
	if err := EnsureOpenSSLConf(&c4, d4); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d4, "project-src", "etc", "ssl.cnf")); err != nil {
		t.Errorf("custom config_path: file not written: %v", err)
	}

	// absolute config_path -> developer-managed, nothing written
	d5 := t.TempDir()
	c5 := *base
	c5.Extensions = map[string]config.Extension{"openssl": {Enabled: true,
		Settings: map[string]bool{"full": true}, Options: map[string]string{"config_path": "/sdcard/openssl.cnf"}}}
	_ = EnsureOpenSSLConf(&c5, d5)
	if entries, _ := os.ReadDir(filepath.Join(d5, "project-src")); len(entries) != 0 {
		t.Errorf("absolute config_path: nothing should be written under project-src")
	}
}

func TestOpenSSLConfArg(t *testing.T) {
	mk := func(opts map[string]string, full bool) *config.Config {
		return &config.Config{Extensions: map[string]config.Extension{
			"openssl": {Enabled: true, Settings: map[string]bool{"full": full}, Options: opts},
		}}
	}
	// default path -> no flag (firmware default applies)
	if _, ok := OpenSSLConfArg(mk(nil, true)); ok {
		t.Errorf("default config_path should emit no flag")
	}
	// custom path -> flag
	arg, ok := OpenSSLConfArg(mk(map[string]string{"config_path": "etc/ssl.cnf"}, true))
	if !ok || arg != "-DPHP_OPENSSL_CONF=etc/ssl.cnf" {
		t.Errorf("OpenSSLConfArg = %q, %v", arg, ok)
	}
	// subset (not full) -> no flag even with a path set
	if _, ok := OpenSSLConfArg(mk(map[string]string{"config_path": "etc/ssl.cnf"}, false)); ok {
		t.Errorf("subset should emit no flag")
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func TestEmbedArg(t *testing.T) {
	// microsd: nothing to embed.
	msd := &config.Config{StorageType: "microsd", Php: config.PhpConfig{Src: "project-src"}}
	if arg, ok := EmbedArg(msd, "/proj"); ok {
		t.Errorf("microsd should not embed, got %q", arg)
	}

	// embedded: source dir resolved absolute against the project dir.
	emb := &config.Config{StorageType: "embedded", Php: config.PhpConfig{Src: "project-src"}}
	arg, ok := EmbedArg(emb, "/proj")
	if !ok || arg != "-DPHP_EMBED_SRC=/proj/project-src" {
		t.Errorf("EmbedArg = %q, %v", arg, ok)
	}

	// embedded with an empty src falls back to project-src.
	def := &config.Config{StorageType: "embedded"}
	if arg, _ := EmbedArg(def, "/proj"); arg != "-DPHP_EMBED_SRC=/proj/project-src" {
		t.Errorf("default src arg = %q", arg)
	}

	// an absolute src is used as-is.
	abs := &config.Config{StorageType: "embedded", Php: config.PhpConfig{Src: "/abs/src"}}
	if arg, _ := EmbedArg(abs, "/proj"); arg != "-DPHP_EMBED_SRC=/abs/src" {
		t.Errorf("absolute src arg = %q", arg)
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
