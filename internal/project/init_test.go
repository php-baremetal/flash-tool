package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phpflash/internal/config"
)

func TestScaffoldWritesFiles(t *testing.T) {
	dir := t.TempDir()
	a := Answers{
		Dir:     dir,
		Starter: "hello",
		Config: &config.Config{
			Name: "demo", StorageType: "microsd", Type: "init-loop",
			Board: config.BoardConfig{Target: "esp32-p4-pico"},
			Php:   config.PhpConfig{Src: "project-src", Entry: "index.php"},
		},
	}
	if err := Scaffold(a); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{config.FileName, ".gitignore", filepath.Join("project-src", "index.php")} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "project-src", "index.php"))
	if !strings.Contains(string(idx), "<?php") {
		t.Errorf("index.php not a PHP file")
	}
}

func TestScaffoldRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, config.FileName), []byte("x"), 0o644)
	a := Answers{Dir: dir, Starter: "hello", Config: &config.Config{}}
	if err := Scaffold(a); err == nil {
		t.Fatal("want ErrExists")
	}
}
