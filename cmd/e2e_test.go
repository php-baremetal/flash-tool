package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"phpflash/internal/config"
	"phpflash/internal/prompt"
)

func TestInitYesEndToEnd(t *testing.T) {
	dir := t.TempDir()
	// --yes: no prompts, all defaults, no php-esp32 installed (extensions skipped).
	opts := initOpts{Dir: dir, Board: "esp32-p4-pico", Yes: true, PhpEsp32Dir: filepath.Join(dir, "no-such")}
	if err := runInit(os.Stdout, &prompt.Scripted{}, opts); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(filepath.Join(dir, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if c.StorageType != "microsd" || c.Type != "init-loop" || c.Php.Entry != "index.php" {
		t.Errorf("defaults wrong: %+v", c)
	}
}
