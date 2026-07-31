package config

import (
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
