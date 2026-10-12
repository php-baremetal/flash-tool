package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"phpflash/internal/config"
	"phpflash/internal/prompt"
)

func TestRunInitScaffoldsWithScriptedAnswers(t *testing.T) {
	dir := t.TempDir()
	p := &prompt.Scripted{
		Inputs:       []string{"demo", ""},   // name, serial port (empty)
		Selects:      []int{0, 0, 0, 0, 0},    // chip, preset, storage, project, starter
		MultiSelects: [][]int{{}},             // no optional extensions
	}
	opts := initOpts{
		Dir:         dir,
		Board:       "waveshare-esp32-p4-pico",
		PhpEsp32Dir: filepath.Join("..", "internal", "manifest", "testdata", "php-esp32"),
	}
	if err := runInit(os.Stdout, p, opts); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(filepath.Join(dir, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "demo" || c.Esp32.StorageType != "microsd" || c.Type != "init-loop" {
		t.Errorf("config = %+v", c)
	}
	if c.Esp32.Preset != "waveshare-esp32-p4-pico" {
		t.Errorf("board preset = %q, want waveshare-esp32-p4-pico", c.Esp32.Preset)
	}
}
