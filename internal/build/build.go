// Package build drives ESP-IDF (approach A) to produce and flash php-esp32 firmware
// with the extensions a project's config declares. The idf.py invocation lives behind
// an Invoker interface so the argument/orchestration logic is unit-tested without a
// real toolchain.
package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"phpflash/internal/config"
	"phpflash/internal/manifest"
)

type Invoker interface {
	Fetch(script string) error     // run <php-esp32>/<script> (e.g. scripts/fetch-sqlite.sh)
	IDF(args ...string) error      // run idf.py <args> in php-esp32 with the ESP-IDF env
	Parttool(args ...string) error // run parttool.py <args> in php-esp32 with the ESP-IDF env
}

func flagName(f string) string {
	if i := strings.IndexByte(f, '='); i >= 0 {
		return f[:i]
	}
	return f
}

func selectionFromConfig(cfg *config.Config) manifest.Selection {
	sel := manifest.Selection{Enabled: map[string]bool{}, Settings: map[string]map[string]bool{}}
	for key, ext := range cfg.Extensions {
		sel.Enabled[key] = ext.Enabled
		if len(ext.Settings) > 0 {
			sel.Settings[key] = ext.Settings
		}
	}
	return sel
}

// Args computes the deterministic -D argument list for idf.py (BOARD, PHP_VERSION, and
// every optional PHP_EXT_* flag the manifest knows, set to ON/OFF) plus the fetch
// scripts that must run before the build.
func Args(cfg *config.Config, m *manifest.Manifest, phpVersion string) (dargs []string, fetches []string, err error) {
	eff, err := m.Effective(cfg.Type, selectionFromConfig(cfg))
	if err != nil {
		return nil, nil, err
	}
	on := map[string]bool{}
	for _, f := range eff.Flags {
		on[flagName(f)] = true
	}
	names := map[string]bool{}
	for _, e := range m.Extensions {
		if e.Flag != "" {
			names[flagName(e.Flag)] = true
		}
		for _, s := range e.Settings {
			names[flagName(s.Flag)] = true
		}
	}
	var sorted []string
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	dargs = append(dargs, "-DBOARD="+cfg.Board.Target, "-DPHP_VERSION="+phpVersion)
	// Always emit the CPU-freq flag (empty when unset) so a reused build dir can't keep a stale
	// cached value -- same reason the extension flags below are always passed ON/OFF.
	freq := ""
	if cfg.Board.CPUFreqMHz > 0 {
		freq = fmt.Sprintf("%d", cfg.Board.CPUFreqMHz)
	}
	dargs = append(dargs, "-DPHP_CPU_FREQ_MHZ="+freq)
	for _, n := range sorted {
		state := "OFF"
		if on[n] {
			state = "ON"
		}
		dargs = append(dargs, "-D"+n+"="+state)
	}
	// Project-type flags (e.g. web-server -> PHP_PROJECT_WEB_SERVER). Pass every one the
	// manifest declares explicitly ON/OFF -- ON for the selected type -- so a build dir that
	// switched types doesn't keep a stale value in its CMake cache.
	for _, pt := range m.ProjectTypes {
		if pt.Flag == "" {
			continue
		}
		state := "OFF"
		if pt.Key == cfg.Type {
			state = "ON"
		}
		dargs = append(dargs, "-D"+flagName(pt.Flag)+"="+state)
	}
	// microSD support: on unless this is an `embedded` project that didn't opt into a card.
	// A `microsd` project always has the card; embedded defaults to no card (SD drivers dropped).
	microsd := cfg.StorageType != "embedded" || cfg.Storage.Microsd
	msdState := "OFF"
	if microsd {
		msdState = "ON"
	}
	dargs = append(dargs, "-DPHP_STORAGE_MICROSD="+msdState)
	return dargs, eff.Fetches, nil
}

// EmbedArg returns the -DPHP_EMBED_SRC argument that builds the project's PHP source into
// the firmware's read-only image, for an `embedded` storage project. The bool is false for
// a `microsd` project (the source ships on the card, so there is nothing to embed). src is
// the project's PHP source folder ([php].src, default "project-src") resolved to an absolute
// path against projectDir, since ESP-IDF runs from its own build tree.
func EmbedArg(cfg *config.Config, projectDir string) (string, bool) {
	if cfg.StorageType != "embedded" {
		return "", false
	}
	src := cfg.Php.Src
	if src == "" {
		src = "project-src"
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(projectDir, src)
	}
	return "-DPHP_EMBED_SRC=" + src, true
}

// EntryArg returns the -DPHP_ENTRY argument naming the entry script within the source ([php] entry),
// so a framework with a nested front controller runs (Laravel: "public/index.php"). The bool is
// false when the entry is the firmware's default ("index.php"/empty), which needs no flag.
func EntryArg(cfg *config.Config) (string, bool) {
	entry := cfg.Php.Entry
	if entry == "" || entry == "index.php" {
		return "", false
	}
	return "-DPHP_ENTRY=" + entry, true
}

// openSSLConf is the minimal openssl.cnf the full openssl build reads at startup (it activates
// the default provider). OpenSSL 3.0 on the chip needs a readable config to fully bring its
// providers up; the firmware points OPENSSL_CONF at this file shipped next to index.php.
const openSSLConf = `# Minimal OpenSSL config for php-esp32's full openssl build. The firmware sets OPENSSL_CONF to
# this file (shipped alongside index.php); OpenSSL 3.0 reads it to bring up the default provider.
openssl_conf = openssl_init

[openssl_init]
providers = provider_sect

[provider_sect]
default = default_sect

[default_sect]
activate = 1
`

// openSSLConfName is the config's file name/path for the full openssl build, from
// [extensions.openssl] config_path (default "openssl.cnf"). A relative value is taken against the
// PHP source folder (so it ships with the source); an absolute one is the on-device path verbatim.
func openSSLConfName(ext config.Extension) string {
	if p := ext.Options["config_path"]; p != "" {
		return p
	}
	return "openssl.cnf"
}

// EnsureOpenSSLConf writes the openssl.cnf into the project's PHP source folder when the project
// builds the full openssl WITHOUT the no_load_config setting -- so the file ships to the card (or
// the embedded image) with the source, where the firmware reads it. The name/path comes from
// config_path (default "openssl.cnf"). No-op when openssl isn't full/enabled, when no_load_config
// is set, or when config_path is absolute (an on-device path the developer manages themselves);
// it never overwrites an existing file.
func EnsureOpenSSLConf(cfg *config.Config, projectDir string) error {
	ext, ok := cfg.Extensions["openssl"]
	if !ok || !ext.Enabled || !ext.Settings["full"] || ext.Settings["no_load_config"] {
		return nil
	}
	conf := openSSLConfName(ext)
	if filepath.IsAbs(conf) {
		return nil // an absolute on-device path -- the developer ships the file themselves
	}
	src := cfg.Php.Src
	if src == "" {
		src = "project-src"
	}
	base := src
	if !filepath.IsAbs(base) {
		base = filepath.Join(projectDir, src)
	}
	path := filepath.Join(base, conf)
	if _, err := os.Stat(path); err == nil {
		return nil // already present -- don't clobber the developer's config
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(openSSLConf), 0o644)
}

// OpenSSLConfArg returns the -DPHP_OPENSSL_CONF build argument telling the firmware where to read
// the openssl.cnf, when the project sets a non-default config_path for a full openssl build. The
// bool is false when the firmware's built-in default ("openssl.cnf") is fine, so no flag is needed.
func OpenSSLConfArg(cfg *config.Config) (string, bool) {
	ext, ok := cfg.Extensions["openssl"]
	if !ok || !ext.Enabled || !ext.Settings["full"] {
		return "", false
	}
	conf := ext.Options["config_path"]
	if conf == "" {
		return "", false // firmware default applies
	}
	return "-DPHP_OPENSSL_CONF=" + conf, true
}

// DefaultCAFile is where the TLS client's CA bundle ships, relative to the PHP source folder,
// unless [extensions.openssl] certs_path overrides it.
const DefaultCAFile = "certs/ca-bundle.crt"

// tlsEnabled reports whether the project builds the TLS client (full openssl + the tls setting).
func tlsEnabled(cfg *config.Config) bool {
	ext, ok := cfg.Extensions["openssl"]
	return ok && ext.Enabled && ext.Settings["full"] && ext.Settings["tls"]
}

// caFilePath is the CA bundle location relative to the source folder (certs_path or the default).
func caFilePath(cfg *config.Config) string {
	if p := cfg.Extensions["openssl"].Options["certs_path"]; p != "" {
		return p
	}
	return DefaultCAFile
}

// DNSArg returns -DPHP_NET_DNS with the static DNS servers ([network] dns) comma-joined for the
// firmware, or false when none are configured (DHCP-provided DNS then stands). Comma, not ';':
// CMake treats ';' as a list separator and would split the define.
func DNSArg(cfg *config.Config) (string, bool) {
	if len(cfg.Network.Dns) == 0 {
		return "", false
	}
	return "-DPHP_NET_DNS=" + strings.Join(cfg.Network.Dns, ","), true
}

// TLSCAArg returns -DPHP_TLS_CAFILE with the CA bundle path, when the project builds the TLS
// client. main.c sets $PHP_TLS_CAFILE from it so the transport verifies peers.
func TLSCAArg(cfg *config.Config) (string, bool) {
	if !tlsEnabled(cfg) {
		return "", false
	}
	return "-DPHP_TLS_CAFILE=" + caFilePath(cfg), true
}

// hostCABundles are the usual system trust-store locations, tried in order when the project
// doesn't set certs_source explicitly (Fedora/RHEL first, then Debian/Ubuntu, then others).
var hostCABundles = []string{
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/ssl/certs/ca-bundle.crt",
	"/etc/ssl/cert.pem",
}

// caBundleDest returns the absolute path the CA bundle ships to (caFilePath resolved against the
// project's source folder). ok is false when certs_path is absolute -- an on-device path the
// developer manages, which phpflash neither writes nor refreshes.
func caBundleDest(cfg *config.Config, projectDir string) (dest string, ok bool) {
	rel := caFilePath(cfg)
	if filepath.IsAbs(rel) {
		return "", false
	}
	src := cfg.Php.Src
	if src == "" {
		src = "project-src"
	}
	base := src
	if !filepath.IsAbs(base) {
		base = filepath.Join(projectDir, src)
	}
	return filepath.Join(base, rel), true
}

// resolveCABundleSource picks the host CA bundle to copy: [extensions.openssl] certs_source, or the
// first system trust store found. Errors when none exists (so the caller can tell the user).
func resolveCABundleSource(cfg *config.Config) (string, error) {
	if s := cfg.Extensions["openssl"].Options["certs_source"]; s != "" {
		if _, err := os.Stat(s); err != nil {
			return "", fmt.Errorf("certs_source %s: %w", s, err)
		}
		return s, nil
	}
	for _, cand := range hostCABundles {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no CA bundle found on this host; set [extensions.openssl] certs_source")
}

// copyBundle reads source and writes it to dest (0644), creating dest's directory. It removes any
// existing dest first: host trust stores are often read-only (0444), and copying one in would make
// the shipped bundle read-only too, so a later overwrite (update-certs) would fail on the truncate.
// Removing needs only a writable directory, so this refreshes a read-only bundle cleanly.
func copyBundle(source, dest string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read CA bundle %s: %w", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	_ = os.Remove(dest)
	return os.WriteFile(dest, data, 0o644)
}

// EnsureTLSCerts copies the host's root-CA bundle into the project's source folder (at caFilePath)
// so it ships to the device, when the project builds the TLS client. No-op when TLS isn't built or
// certs_path is absolute; it never overwrites an existing bundle (use `phpflash update-certs`, or
// RefreshTLSCerts, to refresh). Returns the source copied from, or "" if nothing was copied.
func EnsureTLSCerts(cfg *config.Config, projectDir string) (string, error) {
	if !tlsEnabled(cfg) {
		return "", nil
	}
	dest, ok := caBundleDest(cfg, projectDir)
	if !ok {
		return "", nil // absolute on-device path -- developer-managed
	}
	if _, err := os.Stat(dest); err == nil {
		return "", nil // already shipped -- don't clobber
	}
	source, err := resolveCABundleSource(cfg)
	if err != nil {
		return "", err
	}
	return source, copyBundle(source, dest)
}

// RefreshTLSCerts (re)copies the host's root-CA bundle into the project at certs_path, OVERWRITING
// any existing bundle -- this is what `phpflash update-certs` runs to pick up new/renewed roots
// from the host trust store. It errors (rather than silently no-op'ing) when the project doesn't
// build the TLS client or certs_path is an absolute on-device path. Returns (source, dest).
func RefreshTLSCerts(cfg *config.Config, projectDir string) (source, dest string, err error) {
	if !tlsEnabled(cfg) {
		return "", "", fmt.Errorf("this project doesn't build the TLS client " +
			"([extensions.openssl] full + tls); there are no certificates to update")
	}
	dest, ok := caBundleDest(cfg, projectDir)
	if !ok {
		return "", "", fmt.Errorf("certs_path is an absolute on-device path; phpflash can't manage it -- update it yourself")
	}
	source, err = resolveCABundleSource(cfg)
	if err != nil {
		return "", "", err
	}
	if err := copyBundle(source, dest); err != nil {
		return "", "", err
	}
	return source, dest, nil
}

// compiledDir is where the full ESP-IDF build tree lives, under the project's build/.
func compiledDir(buildDir string) string { return filepath.Join(buildDir, "compiled") }

// Build runs the needed fetch scripts, then builds the ESP-IDF tree under
// <buildDir>/compiled (with sdkconfig there too), and finally copies the flashable
// .bin images up into <buildDir>. So the php-esp32 source stays clean and each project
// keeps an isolated build:
//
//	idf.py -B <buildDir>/compiled -DSDKCONFIG=<sdkconfig> <dargs> build
func Build(inv Invoker, out io.Writer, buildDir, sdkconfig string, dargs, fetches []string) error {
	for _, f := range fetches {
		fmt.Fprintf(out, "==> fetch %s\n", f)
		if err := inv.Fetch(f); err != nil {
			return fmt.Errorf("fetch %s: %w", f, err)
		}
	}
	fmt.Fprintln(out, "==> idf.py build")
	compiled := compiledDir(buildDir)
	args := []string{"-B", compiled, "-DSDKCONFIG=" + sdkconfig}
	args = append(args, dargs...)
	args = append(args, "build")
	if err := inv.IDF(args...); err != nil {
		return err
	}
	bins, err := extractBins(compiled, buildDir)
	if err != nil {
		return err
	}
	for _, b := range bins {
		fmt.Fprintf(out, "==> %s\n", b)
	}
	return nil
}

// Flash runs `idf.py -B <buildDir>/compiled <dargs> [-p port] flash` (builds first if needed).
func Flash(inv Invoker, out io.Writer, buildDir string, dargs []string, port string) error {
	args := []string{"-B", compiledDir(buildDir)}
	args = append(args, dargs...)
	if port != "" {
		args = append(args, "-p", port)
	}
	args = append(args, "flash")
	fmt.Fprintln(out, "==> idf.py flash")
	return inv.IDF(args...)
}

// EraseStoragePartition wipes the board's `storage` partition -- the read-only FAT-image slot that
// `embedded` builds flash the PHP source into. A `microsd` project never writes that partition, so an
// image left there by an earlier `embedded` build (of this or another project) survives a microsd
// flash and still mounts at /app. Because the firmware prefers /app over /sdcard, the board would then
// silently run that stale source instead of the card -- with no error. Erasing it on a microsd flash
// guarantees the embedded mount fails and the firmware falls back to the microSD.
//
// It is best-effort: a board with no `storage` partition (or a device that can't be reached) is not a
// flash failure, so an error here is reported and swallowed rather than returned.
func EraseStoragePartition(inv Invoker, out io.Writer, port string) {
	fmt.Fprintln(out, "==> erasing 'storage' partition (microsd project: stops a leftover embedded image from shadowing the card)")
	args := []string{}
	if port != "" {
		args = append(args, "--port", port)
	}
	args = append(args, "erase_partition", "--partition-name", "storage")
	if err := inv.Parttool(args...); err != nil {
		fmt.Fprintf(out, "    note: could not erase 'storage' (harmless if the board has no such partition): %v\n", err)
	}
}

// Monitor runs `idf.py -B <buildDir>/compiled [-p port] monitor`.
func Monitor(inv Invoker, out io.Writer, buildDir, port string) error {
	args := []string{"-B", compiledDir(buildDir)}
	if port != "" {
		args = append(args, "-p", port)
	}
	args = append(args, "monitor")
	fmt.Fprintln(out, "==> idf.py monitor")
	return inv.IDF(args...)
}

// extractBins copies the flashable .bin images from the ESP-IDF build tree (compiled)
// up into buildDir. Missing files are skipped (e.g. a partial build). Returns the
// destination paths written.
func extractBins(compiled, buildDir string) ([]string, error) {
	// The app image is the single .bin at the build root; bootloader and partition
	// table sit in their own subdirs.
	apps, _ := filepath.Glob(filepath.Join(compiled, "*.bin"))
	srcs := append([]string{}, apps...)
	srcs = append(srcs,
		filepath.Join(compiled, "bootloader", "bootloader.bin"),
		filepath.Join(compiled, "partition_table", "partition-table.bin"),
	)
	var out []string
	for _, src := range srcs {
		data, err := os.ReadFile(src)
		if err != nil {
			continue // not built (yet) -- skip
		}
		dst := filepath.Join(buildDir, filepath.Base(src))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return out, err
		}
		out = append(out, dst)
	}
	return out, nil
}
