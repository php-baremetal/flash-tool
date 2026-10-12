package manifest

import (
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Family is a chip this install supports, declared by boards/<chip>/chip.toml. (The name is kept for
// the callers that map a probed ESP-IDF target to "the chip that supports it".)
type Family struct {
	Key    string `toml:"-"` // the chip directory name, e.g. "esp32-p4"
	Name   string `toml:"name"`
	Target string `toml:"target"`
}

// BoardInfo is the short identity of a selectable board for listing/selection. In the chip->preset
// model a "board" is a vendor-model preset: Key is the preset name (what goes in [esp32] preset),
// Chip is the chip it sits under, and Network/MicroSD are the peripherals the preset wires (a chip
// probe can't tell models apart).
type BoardInfo struct {
	Key     string // the preset name, e.g. "waveshare-esp32-p4-eth"
	Name    string
	Chip    string // the chip key, e.g. "esp32-p4"
	Network string // "ethernet+wifi" | "ethernet" | "wifi" | "none"
	MicroSD bool
}

// chipToml / presetToml mirror the bits of boards/<chip>/chip.toml and a preset we read for listing.
type chipToml struct {
	Name         string `toml:"name"`
	Target       string `toml:"target"`
	Capabilities struct {
		Network []string `toml:"network"`
	} `toml:"capabilities"`
}

type presetToml struct {
	Name    string `toml:"name"`
	Storage struct {
		Microsd struct {
			Mode string `toml:"mode"`
		} `toml:"microsd"`
	} `toml:"storage"`
	Network struct {
		Type string `toml:"type"`
	} `toml:"network"`
}

// Families lists the chips available in an installed php-esp32 (boards/*/chip.toml), sorted by key.
// Empty (not an error) if none.
func Families(phpEsp32Dir string) ([]Family, error) {
	matches, _ := filepath.Glob(filepath.Join(phpEsp32Dir, "boards", "*", "chip.toml"))
	var out []Family
	for _, ct := range matches {
		var f Family
		if _, err := toml.DecodeFile(ct, &f); err != nil {
			return nil, err
		}
		f.Key = filepath.Base(filepath.Dir(ct))
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// presetPaths globs the preset files for a chip: boards/<chip>/presets/*.toml (mode-less) and
// boards/<chip>/*/presets/*.toml (chip/mode).
func presetPaths(phpEsp32Dir, chip string) []string {
	var paths []string
	for _, pat := range []string{
		filepath.Join(phpEsp32Dir, "boards", chip, "presets", "*.toml"),
		filepath.Join(phpEsp32Dir, "boards", chip, "*", "presets", "*.toml"),
	} {
		m, _ := filepath.Glob(pat)
		paths = append(paths, m...)
	}
	return paths
}

// netKind maps a preset's wired network + the chip's on-die radio to a display/offer category. The
// chip's radio is always available (the `wifi` extension), independent of the preset, so a board whose
// chip has WiFi always reports it -- plus "ethernet" when the preset also wires a PHY. A companion-WiFi
// preset (P4 wifi-c6) reports "wifi" even though the chip itself has no radio.
func netKind(presetNet string, chipNetwork []string) string {
	hasWifi := false
	for _, n := range chipNetwork {
		if n == "wifi" {
			hasWifi = true
		}
	}
	wired := presetNet != "" && presetNet != "wifi-c6" // w5500 / emac
	wifi := hasWifi || presetNet == "wifi-c6"
	switch {
	case wired && wifi:
		return "ethernet+wifi"
	case wired:
		return "ethernet"
	case wifi:
		return "wifi"
	default:
		return "none"
	}
}

// BoardsIn lists the presets under a chip (boards/<chip>/presets/ and boards/<chip>/*/presets/),
// sorted by key. Empty (not an error) if none.
func BoardsIn(phpEsp32Dir, chip string) ([]BoardInfo, error) {
	var chipT chipToml
	_, _ = toml.DecodeFile(filepath.Join(phpEsp32Dir, "boards", chip, "chip.toml"), &chipT)
	var out []BoardInfo
	for _, pp := range presetPaths(phpEsp32Dir, chip) {
		var p presetToml
		if _, err := toml.DecodeFile(pp, &p); err != nil {
			return nil, err
		}
		key := filepath.Base(pp)
		key = key[:len(key)-len(".toml")]
		out = append(out, BoardInfo{
			Key:     key,
			Name:    p.Name,
			Chip:    chip,
			Network: netKind(p.Network.Type, chipT.Capabilities.Network),
			MicroSD: p.Storage.Microsd.Mode != "",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// AllPresets lists every preset across all chips, sorted by key. Used to resolve a preset by name
// (e.g. init's default) and to list selectable boards.
func AllPresets(phpEsp32Dir string) ([]BoardInfo, error) {
	fams, err := Families(phpEsp32Dir)
	if err != nil {
		return nil, err
	}
	var out []BoardInfo
	for _, f := range fams {
		boards, err := BoardsIn(phpEsp32Dir, f.Key)
		if err != nil {
			return nil, err
		}
		out = append(out, boards...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// FindPreset returns the preset with the given name (across all chips), and whether it was found.
func FindPreset(phpEsp32Dir, name string) (BoardInfo, bool) {
	all, _ := AllPresets(phpEsp32Dir)
	for _, b := range all {
		if b.Key == name {
			return b, true
		}
	}
	return BoardInfo{}, false
}

