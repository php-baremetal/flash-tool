package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	c := &Config{
		Name:        "demo",
		StorageType: "microsd",
		Type:        "init-loop",
		Board:       BoardConfig{Target: "esp32-p4-pico", Port: ""},
		EspIdf:      SourceConfig{Version: "v5.5.5"},
		PhpEsp32:    SourceConfig{},
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
	if got.Name != "demo" || got.StorageType != "microsd" || got.Type != "init-loop" {
		t.Errorf("scalar fields wrong: %+v", got)
	}
	if got.Board.Target != "esp32-p4-pico" {
		t.Errorf("board target = %q", got.Board.Target)
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
	base := "name = \"x\"\nstorage_type = \"microsd\"\ntype = \"init-loop\"\n[board]\ntarget = \"esp32-p4-pico\"\nport = \"/dev/ttyACM0\"\n"
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Board.Port != "/dev/ttyACM0" || c.Board.Target != "esp32-p4-pico" {
		t.Errorf("base not loaded: %+v", c.Board)
	}
}

func TestLoadNetworkAndTLSOptions(t *testing.T) {
	// The [network] dns array and the openssl string options (config_path/certs_path/certs_source)
	// plus the bool settings must all parse from real TOML.
	dir := t.TempDir()
	base := `name = "x"
storage_type = "embedded"
type = "init-loop"
[board]
target = "esp32-p4-eth"
[network]
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
	if len(c.Network.Dns) != 2 || c.Network.Dns[0] != "1.1.1.1" || c.Network.Dns[1] != "8.8.8.8" {
		t.Errorf("dns = %v", c.Network.Dns)
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
	base := "name = \"x\"\nstorage_type = \"microsd\"\ntype = \"init-loop\"\n" +
		"[board]\ntarget = \"esp32-p4-pico\"\nport = \"\"\n" +
		"[esp-idf]\npath = \"/base/idf\"\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	// Local overrides only the serial port; everything else must stay from the base.
	local := "[board]\nport = \"/dev/ttyACM1\"\n"
	if err := os.WriteFile(filepath.Join(dir, LocalFileName), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if c.Board.Port != "/dev/ttyACM1" {
		t.Errorf("local override not applied: port = %q", c.Board.Port)
	}
	if c.Board.Target != "esp32-p4-pico" {
		t.Errorf("base value clobbered: target = %q", c.Board.Target)
	}
	if c.EspIdf.Path != "/base/idf" {
		t.Errorf("unrelated base value lost: esp-idf.path = %q", c.EspIdf.Path)
	}
	if c.Name != "x" || c.StorageType != "microsd" {
		t.Errorf("base scalars lost: %+v", c)
	}
}
