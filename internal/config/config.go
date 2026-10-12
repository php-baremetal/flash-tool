package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const FileName = "php-baremetal.config.toml"

// LocalFileName is an optional per-project override, applied on top of FileName if present.
// init never creates it and it's git-ignored, so a developer can keep local tweaks (serial
// port, toolchain paths, ...) out of the committed config.
const LocalFileName = "php-baremetal.config.local.toml"

// PartitionsFileName is the optional per-project partition table. When a file with this name
// sits next to the config, `phpflash build` uses it as the fixed-partition spec instead of the
// board's committed table (the auto-generated `storage`/`phpstore` partitions are still
// appended). Create one from the board's defaults with `phpflash partitions publish`.
const PartitionsFileName = "partitions.csv"

// Config is a backend-neutral project config. The portable header (name, type, [php], [env],
// [extensions], [store], [web-server]) describes the app itself; a per-backend block carries board
// selection, toolchain and build-time hardware bring-up. Today the only backend block is [esp32];
// the same project can grow an [stm32]/[nxp] block later, and only the matching backend's tool reads
// its block. The portable extensions are the app's intent -- each backend provides them through its
// own HAL and validates availability.
type Config struct {
	// Portable -- the project, any backend.
	Name       string               `toml:"name"`
	Type       string               `toml:"type"`
	Php        PhpConfig            `toml:"php"`
	Env        EnvConfig            `toml:"env"`
	Extensions map[string]Extension `toml:"extensions"`
	Store      StoreConfig          `toml:"store"`
	WebServer  WebServerConfig      `toml:"web-server"`

	// Per-backend.
	Esp32 Esp32Config `toml:"esp32"`
}

// Esp32Config is the ESP32 backend block: board selection, toolchain checkouts, build-time hardware
// bring-up and security/power -- everything specific to building this project for Espressif chips.
type Esp32Config struct {
	// Board selection: by Preset (a vendor-model preset over a chip/mode) or by Family (the mode's
	// generic preset). Exactly one is used, Preset first.
	Preset string `toml:"preset"`
	Family string `toml:"family"`
	Port   string `toml:"port"`
	// CPUFreqMHz overrides the CPU clock (e.g. 400 on the ESP32-P4, default 360). Empty/0 = the
	// board default. Passed to the firmware as -DPHP_CPU_FREQ_MHZ.
	CPUFreqMHz int `toml:"cpu_freq_mhz"`

	StorageType string `toml:"storage_type"`
	PowerSave   bool   `toml:"power_save"` // esp_pm automatic light sleep (default false)
	Secure      bool   `toml:"secure"`     // Flash Encryption (default false)

	// Secure Boot v2: when SecureBoot is set, the build auto-provisions a per-unit signing key at
	// SecureBootKeysDir/<MAC>.pem (default ./deploys) and enables secure boot pointing at it.
	SecureBoot        bool   `toml:"secure_boot"`
	SecureBootKeysDir string `toml:"secure_boot_keys_dir"`

	Idf     SourceConfig  `toml:"idf"`     // ESP-IDF checkout
	Sdk     SourceConfig  `toml:"sdk"`     // php-esp32 backend checkout
	Storage StorageConfig `toml:"storage"` // build-time storage bring-up (microSD mount)
	Network NetworkConfig `toml:"network"` // build-time network bring-up + outbound DNS
}

// WebServerConfig holds options for the `web-server` project type. Init is an optional PHP script
// (relative to [php] src) the firmware runs once, before the HTTP server starts, for one-time setup
// -- bringing hardware up or seeding mem_*/store_*. Empty (or a non-web-server project) means none.
type WebServerConfig struct {
	Init string `toml:"init"`
}

// StoreConfig sizes the reboot-persistent key-value store (the `store` extension, backed by NVS;
// see docs/store.md). SizeKB > 0 adds a dedicated NVS partition of that size; 0 (or no [store]
// section) means no persistence -- store_available() is false.
type StoreConfig struct {
	SizeKB int `toml:"size_kb"`
}

// EnvConfig controls baking a project's .env into the firmware as $_ENV / getenv() (see
// docs/environment.md). Enabled is a pointer so an absent [env] section (nil) means "on when the
// file exists"; set it false to turn the feature off. File is the env file, relative to the project
// (default ".env").
type EnvConfig struct {
	Enabled *bool  `toml:"enabled"`
	File    string `toml:"file"`
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
	// ReserveKB pads the embedded `storage` partition: the firmware sizes it to the source plus FAT
	// overhead, and adds this many KB on top. Only relevant for an `embedded` project; 0 = just the
	// automatic fit. (The image is read-only, so this is headroom for a bigger source, not runtime
	// growth.)
	ReserveKB int `toml:"reserve_kb"`
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
	// Version pins the PHP language version to build (e.g. "8.5.9"), one of the directories under
	// components/php/versions/. Empty = the repo default (default_version in php-esp32.toml).
	Version string `toml:"version"`
}

// Load reads a php-baremetal.config.toml, then overlays an optional sibling
// php-baremetal.config.local.toml if it exists: keys the local file sets win, the rest keep the
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
			case int64:
				// Numeric settings (e.g. an extension's `pin = 5`) land in Options as a string.
				if ext.Options == nil {
					ext.Options = map[string]string{}
				}
				ext.Options[k] = strconv.FormatInt(val, 10)
			case []interface{}:
				// Array settings (e.g. i2c drivers = ["ssd1306","mpu6050"]) land in Options as a
				// semicolon-joined string, the form the build passes on as a CMake list.
				var parts []string
				for _, e := range val {
					if s, ok := e.(string); ok {
						parts = append(parts, s)
					}
				}
				if ext.Options == nil {
					ext.Options = map[string]string{}
				}
				ext.Options[k] = strings.Join(parts, ";")
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
