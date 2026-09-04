package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
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

// CheckPortAccess reports whether the serial port can be read and written. It uses access(2), so it
// never opens the device (opening would toggle DTR/RTS and reset the board). An empty port (the
// autodetect path) is treated as OK -- esptool handles it. On a permission failure it returns an
// actionable error: on Linux the serial node is group 'dialout', and a first-time user is usually
// not in that group.
func CheckPortAccess(port string) error {
	if port == "" {
		return nil
	}
	if _, err := os.Stat(port); err != nil {
		return nil // gone or not a real path -- let esptool report it
	}
	// R_OK|W_OK = 4|2
	if err := syscall.Access(port, 0x6); err != nil {
		if err == syscall.EACCES || err == syscall.EPERM {
			return fmt.Errorf("serial port %s is not accessible (permission denied).\n"+
				"Your user is probably not in the 'dialout' group that owns it.\n"+
				"  Right now (this port only):  sudo chmod a+rw %s\n"+
				"  Permanent (all boards):      sudo usermod -aG dialout $USER   # then log out/in, or: newgrp dialout",
				port, port)
		}
	}
	return nil
}
