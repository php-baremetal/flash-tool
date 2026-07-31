package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

const FileName = "php-esp32.config.toml"

type Config struct {
	Name        string               `toml:"name"`
	StorageType string               `toml:"storage_type"`
	Type        string               `toml:"type"`
	Board       BoardConfig          `toml:"board"`
	EspIdf      SourceConfig         `toml:"esp-idf"`
	PhpEsp32    SourceConfig         `toml:"php-esp32"`
	Extensions  map[string]Extension `toml:"extensions"`
	Php         PhpConfig            `toml:"php"`
}

type BoardConfig struct {
	Target string `toml:"target"`
	Port   string `toml:"port"`
}

type SourceConfig struct {
	Path    string `toml:"path"`
	Version string `toml:"version"`
}

type Extension struct {
	Enabled  bool            `toml:"enabled"`
	Settings map[string]bool `toml:"-"`
}

type PhpConfig struct {
	Src   string `toml:"src"`   // folder holding the PHP source (copied to the SD / embedded)
	Entry string `toml:"entry"` // entry file within src
}

// Load reads a php-esp32.config.toml. Extension settings live in the same
// [extensions.<key>] table as `enabled`, so a second pass over the raw tree captures
// the remaining bool keys into Extension.Settings (BurntSushi only fills mapped fields).
func Load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, err
	}
	var raw struct {
		Extensions map[string]map[string]interface{} `toml:"extensions"`
	}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, err
	}
	for extKey, tbl := range raw.Extensions {
		ext := c.Extensions[extKey]
		for k, v := range tbl {
			if k == "enabled" {
				continue
			}
			if b, ok := v.(bool); ok {
				if ext.Settings == nil {
					ext.Settings = map[string]bool{}
				}
				ext.Settings[k] = b
			}
		}
		c.Extensions[extKey] = ext
	}
	return &c, nil
}

// Save renders the commented config template with c and writes it to path.
func (c *Config) Save(path string) error {
	out, err := c.render()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
