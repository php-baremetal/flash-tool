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

// TargetForBoard returns the ESP-IDF target (from the containing family's family.toml) for a board
// key, e.g. "esp32-s3-eth" -> "esp32s3". ok is false when no family contains that board (or its
// family declares no target) -- the caller then leaves the target to be resolved by the build
// itself. Used to pin -DIDF_TARGET so idf.py never guesses the target from a stray in-source
// sdkconfig and silently builds the wrong architecture.
func TargetForBoard(phpEsp32Dir, boardKey string) (target string, ok bool) {
	fams, _ := Families(phpEsp32Dir)
	for _, f := range fams {
		boards, _ := BoardsIn(phpEsp32Dir, f.Key)
		for _, b := range boards {
			if b.Key == boardKey {
				return f.Target, f.Target != ""
			}
		}
	}
	return "", false
}

// BoardDir returns the on-disk directory of a board (boards/<family>/<boardKey>) in an installed
// php-esp32, e.g. "esp32-s3-eth" -> "<phpEsp32Dir>/boards/esp32-s3/esp32-s3-eth". ok is false when
// no family contains that board. Used to reach a board's committed files (partitions.csv, ...).
func BoardDir(phpEsp32Dir, boardKey string) (dir string, ok bool) {
	fams, _ := Families(phpEsp32Dir)
	for _, f := range fams {
		boards, _ := BoardsIn(phpEsp32Dir, f.Key)
		for _, b := range boards {
			if b.Key == boardKey {
				return filepath.Join(phpEsp32Dir, "boards", f.Key, b.Key), true
			}
		}
	}
	return "", false
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
