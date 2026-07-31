package platform

import (
	"os"
	"path/filepath"
	"sort"
)

// HomeDir returns the user's home directory ("" if it can't be determined).
func HomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// InstallScript is the ESP-IDF installer to run for this OS.
// Windows shim later: return "install.bat".
func InstallScript() string { return "install.sh" }

// serialGlobs are the serial-device patterns to probe, in order, when no port was given.
// The ESP32-P4-Pico enumerates as /dev/ttyACM* (its on-board CH343P bridge), so that is
// tried first; /dev/ttyUSB* is the usual fallback for other USB-serial adapters.
var serialGlobs = []string{"/dev/ttyACM*", "/dev/ttyUSB*"}

// DetectPort returns the first existing serial device matching the known patterns, or ""
// if none is present -- in which case the caller lets ESP-IDF autodetect. Within a pattern
// the lowest-numbered device wins (ttyACM0 before ttyACM1).
func DetectPort() string { return detectPort(serialGlobs) }

func detectPort(globs []string) string {
	for _, g := range globs {
		matches, _ := filepath.Glob(g)
		sort.Strings(matches)
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
				return m
			}
		}
	}
	return ""
}
