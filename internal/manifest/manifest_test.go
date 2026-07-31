package manifest

import (
	"errors"
	"path/filepath"
	"testing"
)

func dir(t *testing.T) string { return filepath.Join("testdata", "php-esp32") }

func TestLoadRepo(t *testing.T) {
	r, err := LoadRepo(dir(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.DefaultVersion != "8.3.32" || r.DefaultBoard != "esp32-p4/esp32-p4-pico" {
		t.Errorf("repo = %+v", r)
	}
}

func TestLoadManifest(t *testing.T) {
	m, err := LoadManifest(dir(t), "8.3.32")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.StorageTypes) != 2 || len(m.ProjectTypes) != 2 {
		t.Fatalf("modes: %+v %+v", m.StorageTypes, m.ProjectTypes)
	}
	var date *Extension
	for i := range m.Extensions {
		if m.Extensions[i].Key == "date" {
			date = &m.Extensions[i]
		}
	}
	if date == nil || date.Flag != "PHP_EXT_DATE=ON" || len(date.Settings) != 1 ||
		date.Settings[0].Flag != "PHP_EXT_DATE_MINIMAL_TZ=ON" {
		t.Errorf("date extension parsed wrong: %+v", date)
	}
}

func TestLoadBoard(t *testing.T) {
	b, err := LoadBoard(dir(t), "esp32-p4/esp32-p4-pico")
	if err != nil {
		t.Fatal(err)
	}
	if b.Family != "esp32-p4" || len(b.StorageTypes) != 2 || len(b.ProjectTypes) != 2 {
		t.Errorf("board = %+v", b)
	}
}

func TestNotInstalled(t *testing.T) {
	_, err := LoadRepo(filepath.Join("testdata", "nope"))
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("err = %v, want ErrNotInstalled", err)
	}
}
