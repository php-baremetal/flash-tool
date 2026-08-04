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
	Fetch(script string) error // run <php-esp32>/<script> (e.g. scripts/fetch-sqlite.sh)
	IDF(args ...string) error  // run idf.py <args> in php-esp32 with the ESP-IDF env
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
