package manifest

import (
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Family is a chip family declared by boards/<family>/family.toml.
type Family struct {
	Key    string `toml:"-"` // the directory name, e.g. "esp32-p4"
	Name   string `toml:"name"`
	Target string `toml:"target"`
}

// BoardInfo is the short identity of a board, for listing/selection, plus the board-level
// peripherals a chip probe can't see (network interface, microSD slot).
type BoardInfo struct {
	Key     string // the directory name, e.g. "esp32-p4-pico"
	Name    string
	Network string // "ethernet" | "wifi" | "present" | "none"
	MicroSD bool
}

// Families lists the chip families available in an installed php-esp32
// (boards/*/family.toml), sorted by key. Empty (not an error) if none.
func Families(phpEsp32Dir string) ([]Family, error) {
	matches, _ := filepath.Glob(filepath.Join(phpEsp32Dir, "boards", "*", "family.toml"))
	var out []Family
	for _, ft := range matches {
		var f Family
		if _, err := toml.DecodeFile(ft, &f); err != nil {
			return nil, err
		}
		f.Key = filepath.Base(filepath.Dir(ft))
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// BoardsIn lists the boards under a family (boards/<family>/*/board.toml), sorted by
// key. Empty (not an error) if none.
func BoardsIn(phpEsp32Dir, family string) ([]BoardInfo, error) {
	matches, _ := filepath.Glob(filepath.Join(phpEsp32Dir, "boards", family, "*", "board.toml"))
	var out []BoardInfo
	for _, bt := range matches {
		var b Board
		if _, err := toml.DecodeFile(bt, &b); err != nil {
			return nil, err
		}
		out = append(out, BoardInfo{
			Key:     filepath.Base(filepath.Dir(bt)),
			Name:    b.Name,
			Network: b.NetworkKind(),
			MicroSD: b.HasMicroSD(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
