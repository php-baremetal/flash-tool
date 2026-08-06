package build

import (
	"errors"
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
		"-DBOARD=esp32-p4-pico", "-DPHP_VERSION=8.3.32", "-DPHP_CPU_FREQ_MHZ=",
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

func TestArgsCPUFreq(t *testing.T) {
	m := &manifest.Manifest{
		ProjectTypes: []manifest.Mode{{Key: "init-loop", Available: true}},
		Extensions:   []manifest.Extension{},
	}
	// set -> -DPHP_CPU_FREQ_MHZ=400
	set := &config.Config{Type: "init-loop", Board: config.BoardConfig{Target: "esp32-p4-eth", CPUFreqMHz: 400}}
	dargs, _, err := Args(set, m, "8.3.32")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(dargs, "-DPHP_CPU_FREQ_MHZ=400") {
		t.Errorf("cpu_freq_mhz=400: want -DPHP_CPU_FREQ_MHZ=400, got %v", dargs)
	}
	// unset -> flag still emitted, empty (so a reused build dir can't keep a stale value)
	unset := &config.Config{Type: "init-loop", Board: config.BoardConfig{Target: "esp32-p4-eth"}}
	dargs, _, _ = Args(unset, m, "8.3.32")
	if !contains(dargs, "-DPHP_CPU_FREQ_MHZ=") {
		t.Errorf("unset: want empty -DPHP_CPU_FREQ_MHZ=, got %v", dargs)
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

func TestDNSArg(t *testing.T) {
	// no dns -> no flag
	if _, ok := DNSArg(&config.Config{}); ok {
		t.Errorf("no dns should emit no flag")
	}
	// comma-joined (never ';', which CMake would split)
	arg, ok := DNSArg(&config.Config{Network: config.NetworkConfig{Dns: []string{"1.1.1.1", "8.8.8.8"}}})
	if !ok || arg != "-DPHP_NET_DNS=1.1.1.1,8.8.8.8" {
		t.Errorf("DNSArg = %q, %v", arg, ok)
	}
}

func tlsCfg(opts map[string]string) *config.Config {
	return &config.Config{
		Php: config.PhpConfig{Src: "project-src"},
		Extensions: map[string]config.Extension{
			"openssl": {Enabled: true, Settings: map[string]bool{"full": true, "tls": true}, Options: opts},
		},
	}
}

func TestTLSCAArg(t *testing.T) {
	// tls off -> no flag
	if _, ok := TLSCAArg(&config.Config{Extensions: map[string]config.Extension{
		"openssl": {Enabled: true, Settings: map[string]bool{"full": true}}}}); ok {
		t.Errorf("tls off should emit no flag")
	}
	// tls on, default path
	arg, ok := TLSCAArg(tlsCfg(nil))
	if !ok || arg != "-DPHP_TLS_CAFILE="+DefaultCAFile {
		t.Errorf("TLSCAArg default = %q, %v", arg, ok)
	}
	// custom certs_path
	arg, _ = TLSCAArg(tlsCfg(map[string]string{"certs_path": "ca/roots.pem"}))
	if arg != "-DPHP_TLS_CAFILE=ca/roots.pem" {
		t.Errorf("TLSCAArg custom = %q", arg)
	}
}

func TestEnsureTLSCerts(t *testing.T) {
	// make a fake host bundle and point certs_source at it
	host := t.TempDir()
	bundle := filepath.Join(host, "ca-bundle.crt")
	if err := os.WriteFile(bundle, []byte("-----BEGIN CERTIFICATE-----\nX\n-----END CERTIFICATE-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// tls on -> copies bundle to project-src/certs/ca-bundle.crt
	d1 := t.TempDir()
	src, err := EnsureTLSCerts(tlsCfg(map[string]string{"certs_source": bundle}), d1)
	if err != nil {
		t.Fatal(err)
	}
	if src != bundle {
		t.Errorf("source = %q, want %q", src, bundle)
	}
	if _, err := os.Stat(filepath.Join(d1, "project-src", "certs", "ca-bundle.crt")); err != nil {
		t.Errorf("bundle not copied: %v", err)
	}

	// tls off -> nothing copied
	d2 := t.TempDir()
	off := &config.Config{Php: config.PhpConfig{Src: "project-src"},
		Extensions: map[string]config.Extension{"openssl": {Enabled: true, Settings: map[string]bool{"full": true}}}}
	if _, err := EnsureTLSCerts(off, d2); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(filepath.Join(d2, "project-src")); len(entries) != 0 {
		t.Errorf("tls off: nothing should be copied")
	}

	// existing bundle is not clobbered
	d3 := t.TempDir()
	dst := filepath.Join(d3, "project-src", "certs", "ca-bundle.crt")
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.WriteFile(dst, []byte("KEEP"), 0o644)
	if _, err := EnsureTLSCerts(tlsCfg(map[string]string{"certs_source": bundle}), d3); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "KEEP" {
		t.Errorf("existing bundle was overwritten")
	}
}

func TestRefreshTLSCerts(t *testing.T) {
	host := t.TempDir()
	bundle := filepath.Join(host, "roots.pem")
	if err := os.WriteFile(bundle, []byte("NEW-ROOTS"), 0o644); err != nil {
		t.Fatal(err)
	}

	// overwrites an existing bundle (the whole point vs EnsureTLSCerts)
	d1 := t.TempDir()
	dst := filepath.Join(d1, "project-src", "certs", "ca-bundle.crt")
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.WriteFile(dst, []byte("OLD"), 0o644)
	src, gotDest, err := RefreshTLSCerts(tlsCfg(map[string]string{"certs_source": bundle}), d1)
	if err != nil {
		t.Fatal(err)
	}
	if src != bundle || gotDest != dst {
		t.Errorf("src=%q dest=%q", src, gotDest)
	}
	if b, _ := os.ReadFile(dst); string(b) != "NEW-ROOTS" {
		t.Errorf("bundle not refreshed: %q", b)
	}

	// tls off -> error (not a silent no-op)
	off := &config.Config{Php: config.PhpConfig{Src: "project-src"},
		Extensions: map[string]config.Extension{"openssl": {Enabled: true, Settings: map[string]bool{"full": true}}}}
	if _, _, err := RefreshTLSCerts(off, t.TempDir()); err == nil {
		t.Errorf("tls off should error")
	}

	// absolute certs_path -> error (developer-managed)
	abs := tlsCfg(map[string]string{"certs_path": "/sdcard/ca.pem", "certs_source": bundle})
	if _, _, err := RefreshTLSCerts(abs, t.TempDir()); err == nil {
		t.Errorf("absolute certs_path should error")
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

func TestEntryArg(t *testing.T) {
	// default entry -> no flag
	if _, ok := EntryArg(&config.Config{Php: config.PhpConfig{Entry: "index.php"}}); ok {
		t.Errorf("default entry should emit no flag")
	}
	if _, ok := EntryArg(&config.Config{}); ok {
		t.Errorf("empty entry should emit no flag")
	}
	// nested front controller (Laravel) -> flag
	arg, ok := EntryArg(&config.Config{Php: config.PhpConfig{Entry: "public/index.php"}})
	if !ok || arg != "-DPHP_ENTRY=public/index.php" {
		t.Errorf("EntryArg = %q, %v", arg, ok)
	}
}

type fakeInvoker struct {
	fetches  []string
	idf      [][]string
	parttool [][]string
	partErr  error
}

func (f *fakeInvoker) Fetch(s string) error     { f.fetches = append(f.fetches, s); return nil }
func (f *fakeInvoker) IDF(args ...string) error { f.idf = append(f.idf, args); return nil }
func (f *fakeInvoker) Parttool(args ...string) error {
	f.parttool = append(f.parttool, args)
	return f.partErr
}

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

func TestEraseStoragePartition(t *testing.T) {
	f := &fakeInvoker{}
	EraseStoragePartition(f, io.Discard, "/dev/ttyACM0")
	want := []string{"--port", "/dev/ttyACM0", "erase_partition", "--partition-name", "storage"}
	if len(f.parttool) != 1 || !reflect.DeepEqual(f.parttool[0], want) {
		t.Errorf("parttool call = %v, want %v", f.parttool, want)
	}
}

func TestEraseStoragePartitionNoPort(t *testing.T) {
	f := &fakeInvoker{}
	EraseStoragePartition(f, io.Discard, "")
	want := []string{"erase_partition", "--partition-name", "storage"}
	if len(f.parttool) != 1 || !reflect.DeepEqual(f.parttool[0], want) {
		t.Errorf("parttool call = %v, want %v", f.parttool, want)
	}
}

func TestEraseStoragePartitionErrorIsNonFatal(t *testing.T) {
	f := &fakeInvoker{partErr: errors.New("no such partition")}
	// Must not panic and has no error to return -- a missing storage partition is harmless.
	EraseStoragePartition(f, io.Discard, "/dev/ttyACM0")
	if len(f.parttool) != 1 {
		t.Errorf("expected one parttool attempt, got %d", len(f.parttool))
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
