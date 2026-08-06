// Package discover identifies the ESP chip on a connected board and maps it to the boards this
// php-esp32 install supports. It works from the board outwards -- no project or config needed:
// plug a board in, read what it is, and see which supported board(s) match (or learn it's not
// supported yet).
package discover

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"phpflash/internal/manifest"
)

// ChipInfo is what esptool reports about the silicon on the board. Empty fields mean "not seen in
// the output". Target is the ESP-IDF target derived from Chip (e.g. "ESP32-P4" -> "esp32p4"), which
// is what a board family declares, so it's the key we match boards on.
type ChipInfo struct {
	Chip     string // e.g. "ESP32-P4"
	Target   string // e.g. "esp32p4"
	Revision string // e.g. "v1.3"
	MAC      string // e.g. "e8:f6:0a:e0:ce:92"
	Flash    string // e.g. "32MB"
	Features string // e.g. "High-Performance MCU"
	Crystal  string // e.g. "40MHz"
}

var (
	reChip     = regexp.MustCompile(`(?m)^Chip is (.+?)(?: \(revision (.+?)\))?\s*$`)
	reMAC      = regexp.MustCompile(`(?m)^MAC: (.+)$`)
	reFlash    = regexp.MustCompile(`(?m)^Detected flash size: (.+)$`)
	reFeatures = regexp.MustCompile(`(?m)^Features: (.+)$`)
	reCrystal  = regexp.MustCompile(`(?m)^Crystal is (.+)$`)
	reNonAlnum = regexp.MustCompile(`[^a-z0-9]`)
	reParen    = regexp.MustCompile(`\([^)]*\)`)
)

// TargetFromChip turns esptool's chip name into the ESP-IDF target string a family.toml uses:
// lowercase and stripped of non-alphanumerics ("ESP32-P4" -> "esp32p4", "ESP32-S3" -> "esp32s3",
// "ESP32" -> "esp32"). Newer esptool appends the package in parentheses ("ESP32-S3 (QFN56)"); the
// package isn't part of the ESP-IDF target, so drop any parenthesized group first -- otherwise the
// target would come out "esp32s3qfn56" and match no family.
func TargetFromChip(chip string) string {
	chip = reParen.ReplaceAllString(chip, "")
	return reNonAlnum.ReplaceAllString(strings.ToLower(strings.TrimSpace(chip)), "")
}

var reBT = regexp.MustCompile(`\bbt\b`)

// RadioFromFeatures reads the built-in wireless out of esptool's "Features:" line -- this is a
// property of the silicon, so it's the one network interface a chip probe genuinely knows. Returns
// e.g. "WiFi, Bluetooth" / "WiFi, Bluetooth, 802.15.4", or "none" (the ESP32-P4 has no radio).
func RadioFromFeatures(features string) string {
	f := strings.ToLower(features)
	var r []string
	if strings.Contains(f, "wifi") || strings.Contains(f, "wi-fi") {
		r = append(r, "WiFi")
	}
	if strings.Contains(f, "ble") || strings.Contains(f, "bluetooth") || reBT.MatchString(f) {
		r = append(r, "Bluetooth")
	}
	if strings.Contains(f, "802.15.4") {
		r = append(r, "802.15.4")
	}
	if len(r) == 0 {
		return "none"
	}
	return strings.Join(r, ", ")
}

// ParseChipInfo extracts the fields from esptool's flash_id output.
func ParseChipInfo(out string) ChipInfo {
	var c ChipInfo
	if m := reChip.FindStringSubmatch(out); m != nil {
		c.Chip = strings.TrimSpace(m[1])
		c.Revision = m[2]
		c.Target = TargetFromChip(c.Chip)
	}
	if m := reMAC.FindStringSubmatch(out); m != nil {
		c.MAC = strings.TrimSpace(m[1])
	}
	if m := reFlash.FindStringSubmatch(out); m != nil {
		c.Flash = strings.TrimSpace(m[1])
	}
	if m := reFeatures.FindStringSubmatch(out); m != nil {
		c.Features = strings.TrimSpace(m[1])
	}
	if m := reCrystal.FindStringSubmatch(out); m != nil {
		c.Crystal = strings.TrimSpace(m[1])
	}
	return c
}

// ProbeChip runs esptool against the port (sourcing ESP-IDF's export.sh from idfPath so esptool is
// on PATH) and parses what it reports. It talks to the ROM/stub loader, so it briefly resets the
// board into download mode and hard-resets it after. Returns the parsed info and the raw output
// (handy for diagnostics on error).
func ProbeChip(idfPath, port string) (ChipInfo, string, error) {
	script := ". " + shquote(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && esptool.py --chip auto -p " + shquote(port) + " flash_id"
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	info := ParseChipInfo(string(out))
	if err != nil {
		return info, string(out), fmt.Errorf("esptool probe failed: %w", err)
	}
	if info.Chip == "" {
		return info, string(out), fmt.Errorf("could not read a chip from %s (is it the board? in use by a monitor?)", port)
	}
	return info, string(out), nil
}

// DiscoverFW is what the discovery firmware reports over serial (the DISCOVER-FW-BEGIN/END block)
// after ACTIVELY probing the board's peripherals -- Ethernet, microSD -- which the ROM/esptool
// can't see. This is how a fresh, blank board can be identified: flash it, read this. The firmware
// is built per-board (reusing that board's board.c), so Board is the wiring it used.
type DiscoverFW struct {
	Seen       bool
	Board      string // BOARD_NAME the build was compiled for
	Chip       string
	Cores      string
	Revision   string
	PSRAM      string // "32MB" | "none"
	MAC        string
	Ethernet   bool   // board_network_up() brought the link up (needs the cable)
	EthernetNA bool   // this board has no network hardware ("n/a")
	IP         string
	MicroSD    bool   // a card mounted
	CardSize   string // e.g. "14910MB", when a card mounted
}

var (
	reDfBegin = regexp.MustCompile(`(?m)^DISCOVER-FW-BEGIN\s*$`)
	reDfKV    = regexp.MustCompile(`(?m)^([a-z]+)=(.+?)\s*$`)
)

// ParseDiscoverFW pulls the key=value lines out of the discovery firmware's output block.
func ParseDiscoverFW(raw string) DiscoverFW {
	var d DiscoverFW
	d.Seen = reDfBegin.MatchString(raw)
	for _, m := range reDfKV.FindAllStringSubmatch(raw, -1) {
		k, v := m[1], strings.TrimSpace(m[2])
		switch k {
		case "board":
			d.Board = v
		case "chip":
			d.Chip = v
		case "cores":
			d.Cores = v
		case "revision":
			d.Revision = v
		case "psram":
			d.PSRAM = v
		case "mac":
			d.MAC = v
		case "ip":
			d.IP = v
		case "ethernet":
			d.Ethernet = v == "yes"
			d.EthernetNA = v == "n/a"
		case "microsd":
			if strings.HasPrefix(v, "card:") {
				d.MicroSD = true
				d.CardSize = strings.TrimPrefix(v, "card:")
			} else if v == "yes" {
				d.MicroSD = true
			}
		}
	}
	return d
}

// BuildFlashDiscoverFW builds the discovery firmware (php-esp32/tools/discover-fw) FOR A SPECIFIC
// BOARD -- reusing that board's board.c, so the probe uses its real GPIO wiring -- and flashes it,
// streaming idf.py's output to out. The build tree lives in a per-board temp dir, so the install
// stays clean and each board's build caches.
func BuildFlashDiscoverFW(idfPath, phpDir, boardKey, port string, out io.Writer) error {
	proj := filepath.Join(phpDir, "tools", "discover-fw")
	if _, err := os.Stat(filepath.Join(proj, "CMakeLists.txt")); err != nil {
		return fmt.Errorf("discovery firmware not found at %s (php-esp32 too old?)", proj)
	}
	buildDir := filepath.Join(os.TempDir(), "phpflash-discoverfw-"+boardKey)
	idfArgs := []string{"-C", proj, "-B", buildDir, "-DBOARD=" + boardKey}
	if port != "" {
		idfArgs = append(idfArgs, "-p", port)
	}
	idfArgs = append(idfArgs, "flash")
	quoted := make([]string, len(idfArgs))
	for i, a := range idfArgs {
		quoted[i] = shquote(a)
	}
	script := ". " + shquote(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && idf.py " + strings.Join(quoted, " ")
	c := exec.Command("bash", "-c", script)
	c.Stdout, c.Stderr = out, out
	return c.Run()
}

// ReadSerialUntil resets the board and reads its serial output until it sees marker (or timeoutSec
// elapses), returning the captured text. It uses the pyserial that ships with ESP-IDF (sourced from
// idfPath) -- the same reset-and-read the monitor does -- so no extra dependency. Best-effort.
func ReadSerialUntil(idfPath, port, marker string, timeoutSec int) string {
	// No single quotes in this script: the whole thing is single-quoted for bash below.
	const py = `
import serial,time,sys
try:
 p=serial.Serial(sys.argv[1],115200,timeout=0.2)
except Exception:
 sys.exit(0)
p.setDTR(False);p.setRTS(True);time.sleep(0.15);p.setRTS(False)
mark=sys.argv[2].encode();end=time.time()+int(sys.argv[3]);buf=b""
while time.time()<end:
 d=p.read(4096)
 if d:buf+=d
 if mark in buf:
  time.sleep(0.2);buf+=p.read(8192);break
p.close()
sys.stdout.write(buf.decode("utf-8","replace"))
`
	script := ". " + shquote(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && python -c " + shquote(py) + " " +
		shquote(port) + " " + shquote(marker) + " " + shquote(strconv.Itoa(timeoutSec))
	out, _ := exec.Command("bash", "-c", script).Output()
	return string(out)
}

// FamilyForTarget finds the board family whose ESP-IDF target matches (the chip is supported); nil
// if none does (an unsupported board -- the chip is fine, this install just has no board for it).
func FamilyForTarget(families []manifest.Family, target string) *manifest.Family {
	for i := range families {
		if families[i].Target == target {
			return &families[i]
		}
	}
	return nil
}

func shquote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
