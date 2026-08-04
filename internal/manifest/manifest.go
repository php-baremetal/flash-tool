package manifest

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var ErrNotInstalled = errors.New("php-esp32 not installed")

type Repo struct {
	DefaultVersion string `toml:"default_version"`
	DefaultBoard   string `toml:"default_board"`
}

type Mode struct {
	Key         string `toml:"key"`
	Available   bool   `toml:"available"`
	Flag        string `toml:"flag"` // build define for this mode, e.g. "PHP_PROJECT_WEB_SERVER=ON" (project types only)
	Description string `toml:"description"`
}

type Setting struct {
	Key         string `toml:"key"`
	Description string `toml:"description"`
	Flag        string `toml:"flag"`
	Default     bool   `toml:"default"`
	Fetch       string `toml:"fetch"`
}

type Extension struct {
	Key         string    `toml:"key"`
	Description string    `toml:"description"`
	Flag        string    `toml:"flag"`
	Fetch       string    `toml:"fetch"`
	RequiredFor []string  `toml:"required_for"`
	Requires    []string  `toml:"requires"`
	Settings    []Setting `toml:"setting"`
}

type Manifest struct {
	StorageTypes []Mode      `toml:"storage_type"`
	ProjectTypes []Mode      `toml:"project_type"`
	Extensions   []Extension `toml:"extension"`
}

type Board struct {
	Name         string   `toml:"name"`
	Family       string   `toml:"family"`
	StorageTypes []string `toml:"storage_types"`
	ProjectTypes []string `toml:"project_types"`
	Network      string   `toml:"network"` // "ethernet" | "wifi" | "" (none); a board-level peripheral
}

// HasMicroSD reports whether the board carries a microSD slot (declared via storage_types).
func (b Board) HasMicroSD() bool {
	for _, s := range b.StorageTypes {
		if s == "microsd" {
			return true
		}
	}
	return false
}

// NetworkKind returns the board's network interface: the explicit `network` field if set, else
// inferred ("present" if it offers the web-server project type, "none" otherwise). It's a board
// property (external PHY / radio / SD slot), not something the chip can be probed for.
func (b Board) NetworkKind() string {
	if b.Network != "" {
		return b.Network
	}
	for _, p := range b.ProjectTypes {
		if p == "web-server" {
			return "present"
		}
	}
	return "none"
}

func decode(path string, v interface{}) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return ErrNotInstalled
	}
	_, err := toml.DecodeFile(path, v)
	return err
}

func LoadRepo(phpEsp32Dir string) (*Repo, error) {
	var r Repo
	if err := decode(filepath.Join(phpEsp32Dir, "php-esp32.toml"), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func LoadManifest(phpEsp32Dir, version string) (*Manifest, error) {
	var m Manifest
	p := filepath.Join(phpEsp32Dir, "components", "php", "versions", version, "manifest.toml")
	if err := decode(p, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func LoadBoard(phpEsp32Dir, board string) (*Board, error) {
	var b Board
	p := filepath.Join(phpEsp32Dir, "boards", filepath.FromSlash(board), "board.toml")
	if err := decode(p, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
