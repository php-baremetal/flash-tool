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
	return dargs, eff.Fetches, nil
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
