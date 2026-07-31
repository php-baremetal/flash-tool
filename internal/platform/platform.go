package platform

import "os"

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
