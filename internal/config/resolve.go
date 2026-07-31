package config

import (
	"path/filepath"

	"phpflash/internal/platform"
)

type Source struct {
	Value string
	From  string // flag | config | env | default
}

type ResolveInputs struct {
	Flag, Config, Env, Default string
}

// Resolve returns the first non-empty input by precedence: flag > config > env > default.
func Resolve(in ResolveInputs) Source {
	switch {
	case in.Flag != "":
		return Source{in.Flag, "flag"}
	case in.Config != "":
		return Source{in.Config, "config"}
	case in.Env != "":
		return Source{in.Env, "env"}
	default:
		return Source{in.Default, "default"}
	}
}

func DefaultIdfPath() string      { return filepath.Join(platform.HomeDir(), "esp", "esp-idf") }
func DefaultPhpEsp32Path() string { return filepath.Join(platform.HomeDir(), "esp", "php-esp32") }
