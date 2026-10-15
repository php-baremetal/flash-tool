package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
	"phpflash/internal/config"
	"phpflash/internal/discover"
	"phpflash/internal/platform"
)

// rePsram pulls the embedded PSRAM size out of esptool's Features line,
// e.g. "... Embedded PSRAM 2MB (AP_3v3)".
var rePsram = regexp.MustCompile(`(?i)PSRAM\s+(\d+)\s*MB`)

func newFlashCmd() *cobra.Command {
	var idfPath, phpPath, port string
	var force bool
	c := &cobra.Command{
		Use:   "flash",
		Short: "Build and flash the firmware to the board",
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, err := loadBuildContext(idfPath, phpPath)
			if err != nil {
				return err
			}
			p := resolvePort(port, bc.port)
			// Fail early with an actionable message when the port exists but isn't accessible
			// (the common first-run "not in the dialout group" case), instead of a cryptic esptool error.
			if err := platform.CheckPortAccess(p); err != nil {
				return err
			}
			// Pre-flight guards (skipped by --force): one chip probe shared by both. Lenient -- a probe
			// that can't run leaves ChipInfo empty and each guard waves the flash through (esptool is the
			// backstop).
			if !force {
				var chip discover.ChipInfo
				if p != "" {
					if info, _, perr := discover.ProbeChip(bc.idfPath, p); perr == nil {
						chip = info
					}
				}
				// Guard against flashing firmware built for one chip onto a different one (e.g. an
				// esp32s3 image onto a connected P4). esptool would fail cryptically mid-flash.
				if err := checkChipMatchesBoard(bc, chip, os.Stdout); err != nil {
					return err
				}
				// Warn on a possible PSRAM-mode mismatch (the project declares quad/octal; the board only
				// reports a size, which doesn't pin the mode down -- 8 MB can be quad or octal). A likely
				// mismatch asks to confirm rather than hard-blocks, since the wrong mode means no boot.
				if err := checkPsramMode(cmd, bc, chip, os.Stdout); err != nil {
					return err
				}
				// Guard against an image that doesn't fit the connected board's flash.
				if err := checkFlashFits(bc, chip, os.Stdout); err != nil {
					return err
				}
			}
			// A secure project writes through the chip's hardware encryption once the key is burned.
			// The first flash of a virgin chip is plaintext (it encrypts itself on first boot); only a
			// chip that already has Flash Encryption needs encrypted-flash.
			encrypted := false
			if bc.secure {
				if chipFlashEncrypted(bc.idfPath, p) {
					encrypted = true
					fmt.Fprintln(os.Stdout, "secure: the chip is already encrypted -> using encrypted-flash")
				} else {
					fmt.Fprintln(os.Stdout, "secure: first flash (plaintext; the chip encrypts itself on first boot)")
				}
			}
			inv := build.ExecInvoker{PhpEsp32Dir: bc.phpDir, IdfPath: bc.idfPath, Out: os.Stdout}
			if err := build.Flash(inv, os.Stdout, bc.buildDir, bc.dargs, p, encrypted); err != nil {
				return err
			}
			// A microsd project ships no embedded image; wipe the `storage` partition so a leftover
			// one from an earlier embedded build can't mount at /app and shadow the microSD. Skipped when
			// the flashed table has no `storage` partition: then there is nothing to mount or erase.
			if bc.storageType != "" && bc.storageType != "embedded" && build.FlashedTableHasStorage(bc.buildDir) {
				build.EraseStoragePartition(inv, os.Stdout, p)
			}
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port (empty = /dev/ttyACM*, then autodetect)")
	c.Flags().BoolVar(&force, "force", false, "flash even if the connected chip doesn't match the project's board / flash")
	return c
}

// chipFlashEncrypted reports whether the connected chip already has Flash Encryption burned, so the
// flash must go through encrypted-flash. Best-effort: any probe failure returns false and the flash
// proceeds as plaintext (the chip/idf.py stay the backstop).
func chipFlashEncrypted(idfPath, port string) bool {
	if port == "" {
		return false
	}
	sec, err := discover.ProbeSecurity(idfPath, port)
	return err == nil && sec.FlashEncryption
}

// checkChipMatchesBoard refuses the flash if the (pre-probed) chip's ESP-IDF target differs from the
// project board's family target. Lenient: an empty board target or an empty probe result (couldn't
// read the chip) lets the flash proceed -- only a *confirmed* mismatch aborts.
func checkChipMatchesBoard(bc *buildContext, info discover.ChipInfo, out io.Writer) error {
	if bc.idfTarget == "" || info.Target == "" {
		return nil
	}
	if info.Target != bc.idfTarget {
		return fmt.Errorf(
			"board mismatch: this project targets %q (%s), but the connected chip is %s (%s).\n"+
				"  Fix [esp32].preset in %s, plug in the right board, or re-run with --force.",
			bc.board, bc.idfTarget, info.Chip, info.Target, config.FileName)
	}
	fmt.Fprintf(out, "board check: %s (%s) matches the project's %s\n", info.Chip, info.Target, bc.board)
	return nil
}

// checkFlashFits refuses the flash if the built image needs more flash than the connected board has.
// Lenient: if the actual flash size (from the probe) or the image extent (flasher_args.json) can't be
// read, it lets the flash proceed.
func checkFlashFits(bc *buildContext, info discover.ChipInfo, out io.Writer) error {
	if info.Flash == "" {
		return nil
	}
	actualK := parseSizeK(info.Flash)
	if actualK == 0 {
		return nil
	}
	need := requiredFlashBytes(bc.buildDir)
	if need == 0 {
		return nil
	}
	actual := int64(actualK) * 1024
	if need > actual {
		return fmt.Errorf(
			"flash too small: the firmware needs ~%s but this board has only %s.\n"+
				"  Reduce the project (embedded source / store) or use a board with more flash,\n"+
				"  or re-run with --force to flash anyway.",
			humanBytes(need), info.Flash)
	}
	fmt.Fprintf(out, "flash check: image ~%s fits the board's %s\n", humanBytes(need), info.Flash)
	return nil
}

// checkPsramMode refuses the flash when the build's PSRAM mode (quad/octal, read from the compiled
// sdkconfig) doesn't match the connected module. The wrong mode leaves PSRAM uninitialised, and since
// the whole PHP heap lives there the board simply won't boot. On the S3 the module's PSRAM size maps to
// the mode (<=2 MB quad, >=8 MB octal). Lenient: no declared mode (e.g. a P4 build) or an unreadable
// PSRAM size waves the flash through.
func checkPsramMode(cmd *cobra.Command, bc *buildContext, info discover.ChipInfo, out io.Writer) error {
	want := expectedPsramMode(bc.sdkconfig)
	if want == "" {
		return nil
	}
	mb := chipPsramMB(info.Features)
	if mb == 0 {
		return nil
	}
	suspect, guess := psramSuspect(want, mb)
	if !suspect {
		fmt.Fprintf(out, "PSRAM check: %s build, board reports %d MB -- consistent\n", want, mb)
		return nil
	}
	fmt.Fprintf(out, "PSRAM warning: this build targets %s PSRAM, but the board reports %d MB PSRAM,\n"+
		"  which might be %s. If it is, PSRAM won't initialise and the board won't boot.\n", want, mb, guess)
	if !confirm(cmd, "Flash anyway?") {
		return fmt.Errorf("aborted: PSRAM mode not confirmed (use --force to skip this check)")
	}
	return nil
}

// psramSuspect reports whether the board's reported PSRAM size looks inconsistent with the build's
// declared mode, and the mode it more likely is. The size doesn't pin the mode down -- 2 MB modules are
// quad, but 8 MB exists in both quad (APS6404) and octal (APS6408) -- so this is a heads-up, not a
// verdict: a 2 MB board can't be octal (fairly sure), and an 8 MB board on a quad build might be octal.
func psramSuspect(want string, mb int) (bool, string) {
	switch {
	case want == "octal" && mb > 0 && mb < 4:
		return true, "quad"
	case want == "quad" && mb >= 8:
		return true, "octal"
	default:
		return false, ""
	}
}

// expectedPsramMode reads the compiled sdkconfig for the S3 PSRAM mode: "quad", "octal", or "" (no mode
// declared -- e.g. a P4 build, whose in-package PSRAM has no quad/octal choice).
func expectedPsramMode(sdkconfigPath string) string {
	raw, err := os.ReadFile(sdkconfigPath)
	if err != nil {
		return ""
	}
	s := string(raw)
	if strings.Contains(s, "CONFIG_SPIRAM_MODE_OCT=y") {
		return "octal"
	}
	if strings.Contains(s, "CONFIG_SPIRAM_MODE_QUAD=y") {
		return "quad"
	}
	return ""
}

// chipPsramMB parses the embedded PSRAM size (MB) from esptool's Features line. 0 when absent.
func chipPsramMB(features string) int {
	m := rePsram.FindStringSubmatch(features)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// requiredFlashBytes returns the highest flash address the image writes to (max offset + file size
// across flasher_args.json's flash_files), i.e. how much flash the build needs. 0 when it can't be read.
func requiredFlashBytes(buildDir string) int64 {
	compiled := filepath.Join(buildDir, "compiled")
	raw, err := os.ReadFile(filepath.Join(compiled, "flasher_args.json"))
	if err != nil {
		return 0
	}
	var fa struct {
		FlashFiles map[string]string `json:"flash_files"`
	}
	if err := json.Unmarshal(raw, &fa); err != nil {
		return 0
	}
	var max int64
	for off, rel := range fa.FlashFiles {
		o, err := strconv.ParseInt(strings.TrimPrefix(strings.ToLower(off), "0x"), 16, 64)
		if err != nil {
			continue
		}
		fi, err := os.Stat(filepath.Join(compiled, rel))
		if err != nil {
			continue
		}
		if end := o + fi.Size(); end > max {
			max = end
		}
	}
	return max
}

// humanBytes renders a byte count as a short MB/KB string for messages.
func humanBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0fKB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
