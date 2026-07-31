package manifest

import (
	"path/filepath"
	"testing"
)

func TestFamiliesAndBoards(t *testing.T) {
	dir := filepath.Join("testdata", "php-esp32")
	fams, err := Families(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fams) != 1 || fams[0].Key != "esp32-p4" || fams[0].Target != "esp32p4" {
		t.Fatalf("families = %+v", fams)
	}
	boards, err := BoardsIn(dir, "esp32-p4")
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].Key != "esp32-p4-pico" || boards[0].Name != "ESP32-P4-Pico" {
		t.Fatalf("boards = %+v", boards)
	}
}
