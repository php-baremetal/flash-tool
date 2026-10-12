package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	c := &Config{
		Name: "demo",
		Type: "init-loop",
		Esp32: Esp32Config{
			StorageType: "microsd",
			Preset:      "waveshare-esp32-p4-pico",
			Port:        "",
			Idf:         SourceConfig{Version: "v5.5.5"},
			Sdk:         SourceConfig{},
		},
		Extensions: map[string]Extension{
			"date":   {Enabled: true, Settings: map[string]bool{"minimal_tz": true}},
			"sqlite": {Enabled: false},
		},
		Php: PhpConfig{Src: "project-src", Entry: "index.php"},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "demo" || got.Esp32.StorageType != "microsd" || got.Type != "init-loop" {
		t.Errorf("scalar fields wrong: %+v", got)
	}
	if got.Esp32.Preset != "waveshare-esp32-p4-pico" {
		t.Errorf("board preset = %q", got.Esp32.Preset)
	}
	if !got.Extensions["date"].Enabled || !got.Extensions["date"].Settings["minimal_tz"] {
		t.Errorf("date extension/settings lost: %+v", got.Extensions["date"])
	}
	if got.Extensions["sqlite"].Enabled {
		t.Errorf("sqlite should be disabled")
	}
	if got.Php.Src != "project-src" || got.Php.Entry != "index.php" {
		t.Errorf("php = %+v", got.Php)
	}
}

func TestLoadNoLocalFile(t *testing.T) {
	// With no *.local.toml present, Load returns just the base config.
	dir := t.TempDir()
	base := "name = \"x\"\ntype = \"init-loop\"\n[esp32]\npreset = \"waveshare-esp32-p4-pico\"\nport = \"/dev/ttyACM0\"\nstorage_type = \"microsd\"\n"
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Esp32.Port != "/dev/ttyACM0" || c.Esp32.Preset != "waveshare-esp32-p4-pico" {
		t.Errorf("base not loaded: %+v", c.Esp32)
	}
}

func TestLoadNetworkAndTLSOptions(t *testing.T) {
	// The [esp32.network] dns array and the openssl string options (config_path/certs_path/
	// certs_source) plus the bool settings must all parse from real TOML.
	dir := t.TempDir()
	base := `name = "x"
type = "init-loop"
[esp32]
preset = "waveshare-esp32-p4-eth"
storage_type = "embedded"
[esp32.network]
dns = ["1.1.1.1", "8.8.8.8"]
[extensions.openssl]
enabled = true
full = true
tls = true
certs_path = "certs/roots.pem"
certs_source = "/etc/ssl/certs/ca-certificates.crt"
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Esp32.Network.Dns) != 2 || c.Esp32.Network.Dns[0] != "1.1.1.1" || c.Esp32.Network.Dns[1] != "8.8.8.8" {
		t.Errorf("dns = %v", c.Esp32.Network.Dns)
	}
	ext := c.Extensions["openssl"]
	if !ext.Enabled || !ext.Settings["full"] || !ext.Settings["tls"] {
		t.Errorf("openssl bool settings = %+v", ext.Settings)
	}
	if ext.Options["certs_path"] != "certs/roots.pem" || ext.Options["certs_source"] != "/etc/ssl/certs/ca-certificates.crt" {
		t.Errorf("openssl string options = %+v", ext.Options)
	}
}

func TestLoadLocalOverride(t *testing.T) {
	dir := t.TempDir()
	base := "name = \"x\"\ntype = \"init-loop\"\n" +
		"[esp32]\npreset = \"waveshare-esp32-p4-pico\"\nport = \"\"\nstorage_type = \"microsd\"\n" +
		"[esp32.idf]\npath = \"/base/idf\"\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	// Local overrides only the serial port; everything else must stay from the base.
	local := "[esp32]\nport = \"/dev/ttyACM1\"\n"
	if err := os.WriteFile(filepath.Join(dir, LocalFileName), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if c.Esp32.Port != "/dev/ttyACM1" {
		t.Errorf("local override not applied: port = %q", c.Esp32.Port)
	}
	if c.Esp32.Preset != "waveshare-esp32-p4-pico" {
		t.Errorf("base value clobbered: preset = %q", c.Esp32.Preset)
	}
	if c.Esp32.Idf.Path != "/base/idf" {
		t.Errorf("unrelated base value lost: esp32.idf.path = %q", c.Esp32.Idf.Path)
	}
	if c.Name != "x" || c.Esp32.StorageType != "microsd" {
		t.Errorf("base scalars lost: %+v", c)
	}
}
