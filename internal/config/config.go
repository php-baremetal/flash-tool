package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const FileName = "php-esp32.config.toml"

// LocalFileName is an optional per-project override, applied on top of FileName if present.
// init never creates it and it's git-ignored, so a developer can keep local tweaks (serial
// port, toolchain paths, ...) out of the committed config.
const LocalFileName = "php-esp32.config.local.toml"

type Config struct {
	Name        string               `toml:"name"`
	StorageType string               `toml:"storage_type"`
	Type        string               `toml:"type"`
	Board       BoardConfig          `toml:"board"`
	EspIdf      SourceConfig         `toml:"esp-idf"`
	PhpEsp32    SourceConfig         `toml:"php-esp32"`
	Storage     StorageConfig        `toml:"storage"`
	Extensions  map[string]Extension `toml:"extensions"`
	Php         PhpConfig            `toml:"php"`
	Network     NetworkConfig        `toml:"network"`
}

// NetworkConfig holds outbound-networking options for boards that have a network. `dns` is an
// optional list of static DNS servers the firmware applies after DHCP (empty = use DHCP's).
type NetworkConfig struct {
	Dns []string `toml:"dns"`
}

// StorageConfig holds options for the chosen storage_type. `microsd` matters only for an
// `embedded` project: with it on, the firmware also mounts a microSD (for writable data)
// alongside the flashed-in source. A `microsd` project always has the card; an `embedded` one
// defaults to no card (the SD drivers aren't even compiled in).
type StorageConfig struct {
	Microsd bool `toml:"microsd"`
}

type BoardConfig struct {
	Target string `toml:"target"`
	Port   string `toml:"port"`
	// CPUFreqMHz overrides the CPU clock (e.g. 400 on the ESP32-P4, default 360). Empty/0 = the
	// board default. Passed to the firmware as -DPHP_CPU_FREQ_MHZ.
	CPUFreqMHz int `toml:"cpu_freq_mhz"`
}

type SourceConfig struct {
	Path    string `toml:"path"`
	Version string `toml:"version"`
}

type Extension struct {
	Enabled  bool              `toml:"enabled"`
	Settings map[string]bool   `toml:"-"`
	Options  map[string]string `toml:"-"` // string-valued settings, e.g. openssl config_path
}

type PhpConfig struct {
	Src   string `toml:"src"`   // folder holding the PHP source (copied to the SD / embedded)
	Entry string `toml:"entry"` // entry file within src
}

// Load reads a php-esp32.config.toml, then overlays an optional sibling
// php-esp32.config.local.toml if it exists: keys the local file sets win, the rest keep the
// base value. The local file is optional -- absent means "just the base config".
func Load(path string) (*Config, error) {
	c := &Config{}
	if err := applyFile(path, c); err != nil {
		return nil, err
	}
	local := filepath.Join(filepath.Dir(path), LocalFileName)
	if _, err := os.Stat(local); err == nil {
		if err := applyFile(local, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// applyFile decodes a config TOML over c. BurntSushi sets only the fields the file actually
// defines, so decoding the local file over the already-populated base gives override semantics
// for the scalar/table fields. Extension settings live in the same [extensions.<key>] table as
// `enabled`, so a second raw pass captures the extra bool keys into Extension.Settings.
func applyFile(path string, c *Config) error {
	if _, err := toml.DecodeFile(path, c); err != nil {
		return err
	}
	var raw struct {
		Extensions map[string]map[string]interface{} `toml:"extensions"`
	}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return err
	}
	for extKey, tbl := range raw.Extensions {
		ext := c.Extensions[extKey]
		for k, v := range tbl {
			if k == "enabled" {
				continue
			}
			switch val := v.(type) {
			case bool:
				if ext.Settings == nil {
					ext.Settings = map[string]bool{}
				}
				ext.Settings[k] = val
			case string:
				if ext.Options == nil {
					ext.Options = map[string]string{}
				}
				ext.Options[k] = val
			}
		}
		c.Extensions[extKey] = ext
	}
	return nil
}

// Save renders the commented config template with c and writes it to path.
func (c *Config) Save(path string) error {
	out, err := c.render()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
