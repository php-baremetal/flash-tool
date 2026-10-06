package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
	"phpflash/internal/config"
	"phpflash/internal/discover"
	"phpflash/internal/platform"
)

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
			// Guard against flashing firmware built for one chip onto a different one (e.g. an
			// esp32s3 board's image onto a connected P4). esptool would fail cryptically mid-flash;
			// catch it up front with an actionable message. Skipped by --force, and never blocks on a
			// probe that simply couldn't run (no board yet, busy port -- esptool stays the backstop).
			if !force {
				if err := checkChipMatchesBoard(bc, p, os.Stdout); err != nil {
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
			// one from an earlier embedded build can't mount at /app and shadow the microSD.
			if bc.storageType != "" && bc.storageType != "embedded" {
				build.EraseStoragePartition(inv, os.Stdout, p)
			}
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port (empty = /dev/ttyACM*, then autodetect)")
	c.Flags().BoolVar(&force, "force", false, "flash even if the connected chip doesn't match the project's board")
	return c
}

// chipFlashEncrypted reports whether the connected chip already has Flash Encryption burned (eFuse
// SPI_BOOT_CRYPT_CNT enabled), so flashing must go through encrypted-flash. Best-effort: any probe
// failure returns false and the flash proceeds as plaintext (the chip/idf.py stay the backstop).
func chipFlashEncrypted(idfPath, port string) bool {
	if port == "" {
		return false
	}
	script := ". " + shq(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && espefuse.py --port " + shq(port) + " summary"
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "SPI_BOOT_CRYPT_CNT") && strings.Contains(line, "Enable") {
			return true
		}
	}
	return false
}

// checkChipMatchesBoard probes the connected chip and refuses the flash if its ESP-IDF target
// differs from the project board's family target. It is deliberately lenient: if the board's target
// can't be resolved, or the probe itself fails, it returns nil and lets the flash proceed (esptool
// verifies the chip again during the actual write). Only a *confirmed* mismatch aborts.
func checkChipMatchesBoard(bc *buildContext, port string, out io.Writer) error {
	if bc.idfTarget == "" {
		return nil // board not tied to a family target -- nothing to compare against
	}
	info, _, err := discover.ProbeChip(bc.idfPath, port)
	if err != nil || info.Target == "" {
		return nil // couldn't probe -- don't block; esptool will still verify the chip
	}
	if info.Target != bc.idfTarget {
		return fmt.Errorf(
			"board mismatch: this project targets %q (%s), but the connected chip is %s (%s).\n"+
				"  Fix [board].target in %s, plug in the right board, or re-run with --force.",
			bc.board, bc.idfTarget, info.Chip, info.Target, config.FileName)
	}
	fmt.Fprintf(out, "board check: %s (%s) matches the project's %s\n", info.Chip, info.Target, bc.board)
	return nil
}
