package project

import (
	"errors"
	"os"
	"path/filepath"

	"phpflash/internal/config"
	"phpflash/internal/templates"
)

var ErrExists = errors.New("php-esp32.config.toml already exists")

type Answers struct {
	Dir     string
	Config  *config.Config
	Starter string // "hello" | "blink"
}

// Scaffold writes the config and a .gitignore into a.Dir, and the chosen starter
// index.php into the project's PHP source folder (a.Config.Php.Src, default
// "project-src") -- that folder is what gets copied to the microSD. It refuses to
// overwrite an existing config; the caller handles --force by removing it first.
func Scaffold(a Answers) error {
	if err := os.MkdirAll(a.Dir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(a.Dir, config.FileName)
	if _, err := os.Stat(cfgPath); err == nil {
		return ErrExists
	}
	if err := a.Config.Save(cfgPath); err != nil {
		return err
	}

	src := a.Config.Php.Src
	if src == "" {
		src = "project-src"
	}
	srcDir := filepath.Join(a.Dir, src)
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return err
	}
	starter := templates.IndexHello
	if a.Starter == "blink" {
		starter = templates.IndexBlink
	}
	entry := a.Config.Php.Entry
	if entry == "" {
		entry = "index.php"
	}
	if err := os.WriteFile(filepath.Join(srcDir, entry), starter, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.Dir, ".gitignore"), templates.Gitignore, 0o644)
}
