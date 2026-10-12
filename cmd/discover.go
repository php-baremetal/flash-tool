package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/discover"
	"phpflash/internal/manifest"
	"phpflash/internal/platform"
)

// printProbe renders one candidate preset's discovery-firmware result.
func printProbe(out io.Writer, df discover.DiscoverFW) {
	switch {
	case df.EthernetNA:
		fmt.Fprintln(out, "  Ethernet:     n/a (this preset wires no Ethernet)")
	case df.Ethernet:
		fmt.Fprintf(out, "  Ethernet:     up (%s)\n", df.IP)
	default:
		fmt.Fprintln(out, "  Ethernet:     no link (no PHY on these pins, or no cable/DHCP)")
	}
	switch {
	case df.MicroSDNA:
		fmt.Fprintln(out, "  microSD:      n/a (this preset wires no card slot)")
	case df.MicroSD:
		fmt.Fprintf(out, "  microSD:      card present (%s)\n", df.CardSize)
	default:
		fmt.Fprintln(out, "  microSD:      no card (empty slot or none)")
	}
	if df.PSRAM != "" {
		fmt.Fprintf(out, "  PSRAM:        %s\n", df.PSRAM)
	}
}

// discoverFWInputs resolves a preset into the inputs the discovery firmware build needs: its own temp
// build dir (where resolveBoard writes the preset sdkconfig fragment), the -D flags the main build
// would pass, the process env board.cmake reads in the early phase, and the wired network type ("" if
// none, i.e. wifi/none -- those have no link to probe).
func discoverFWInputs(phpDir, presetKey string) (buildDir string, dargs, env []string, netType string, err error) {
	cfg := &config.Config{}
	cfg.Esp32.Preset = presetKey
	buildDir = filepath.Join(os.TempDir(), "phpflash-discoverfw-"+strings.ReplaceAll(presetKey, "/", "_"))
	res, err := resolveBoard(cfg, phpDir, buildDir)
	if err != nil {
		return "", nil, nil, "", err
	}
	dargs = append([]string{"-DBOARD=" + res.BoardKey}, res.DArgs...)
	if res.IDFTarget != "" {
		dargs = append(dargs, "-DIDF_TARGET="+res.IDFTarget)
	}
	env = []string{"PHP_ESP32_MICROSD=ON", "PHP_ESP32_NETWORK=" + res.Network, "PHP_ESP32_BOARD=" + res.BoardKey}
	return buildDir, dargs, env, res.Network, nil
}

// newDiscoverCmd inspects the board that's plugged in -- from the board outwards, no project or
// config needed. It lists the serial ports, reads the USB bridge, probes the chip with esptool,
// and maps the chip to the boards this php-esp32 install supports (or reports that it's a board
// nothing here covers yet).
func newDiscoverCmd() *cobra.Command {
	var idfPath, phpPath, port string
	var noProbe, all, assumeYes bool
	c := &cobra.Command{
		Use:   "discover",
		Short: "Identify the connected board (chip + matching supported boards)",
		Long: "Identify whatever board is plugged in, starting from the board itself (no project\n" +
			"needed). Lists serial ports and their USB bridge, probes the chip with esptool, and maps\n" +
			"it to the boards this php-esp32 supports -- or tells you it's not a supported board yet.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			// Board-first: resolve tool paths from flags/env/defaults only -- never a project config.
			idf, phpDir := resolveDirs(idfPath, phpPath, nil)

			ports := platform.ListPorts()
			if len(ports) == 0 {
				return fmt.Errorf("no serial device found (looked for /dev/ttyACM*, /dev/ttyUSB*). " +
					"Plug the board in and check the USB cable")
			}

			target := port
			if target == "" {
				target = ports[0]
			}
			fmt.Fprintln(out, "Serial ports:")
			for _, p := range ports {
				mark := "  "
				if p == target {
					mark = "> "
				}
				if usb, ok := platform.PortUSB(p); ok {
					fmt.Fprintf(out, "%s%s  (%s)\n", mark, p, usbDesc(usb))
				} else {
					fmt.Fprintf(out, "%s%s\n", mark, p)
				}
			}

			if noProbe {
				fmt.Fprintln(out, "\n(--no-probe: skipping the chip probe)")
				printChips(out, phpDir)
				return nil
			}

			fmt.Fprintf(out, "\nProbing %s (resets the board momentarily)...\n", target)
			info, raw, err := discover.ProbeChip(idf, target)
			if err != nil {
				fmt.Fprintln(out, indent(raw))
				return err
			}
			fmt.Fprintf(out, "Chip:     %s", info.Chip)
			if info.Revision != "" {
				fmt.Fprintf(out, " (revision %s)", info.Revision)
			}
			fmt.Fprintln(out)
			fmt.Fprintf(out, "Target:   %s\n", info.Target)
			if info.Flash != "" {
				fmt.Fprintf(out, "Flash:    %s\n", info.Flash)
			}
			if info.MAC != "" {
				fmt.Fprintf(out, "MAC:      %s\n", info.MAC)
			}
			if info.Features != "" {
				fmt.Fprintf(out, "Features: %s\n", info.Features)
			}
			// The chip's built-in wireless -- the one network interface the silicon itself reveals.
			if radio := discover.RadioFromFeatures(info.Features); radio == "none" {
				fmt.Fprintf(out, "Radio:    none (no built-in WiFi/BT; this chip needs a companion for wireless)\n")
			} else {
				fmt.Fprintf(out, "Radio:    %s (built into the chip)\n", radio)
			}

			// Hardware protection state (the eFuses the `secure` / `secure_boot` flags burn).
			if sec, err := discover.ProbeSecurity(idf, target); err == nil {
				fmt.Fprintf(out, "Flash encryption: %s\n", enabledStr(sec.FlashEncryption))
				fmt.Fprintf(out, "Secure boot:      %s\n", enabledStr(sec.SecureBoot))
				if sec.FlashEncryption || sec.SecureBoot {
					fmt.Fprintln(out, "  (protected board: flash with `phpflash flash` -- it writes encrypted; with secure boot, updates need the signing key)")
				}
			}

			// Board-first mapping: which chip supports this target, and its presets.
			chips, _ := manifest.Families(phpDir)
			chip := discover.FamilyForTarget(chips, info.Target)
			if chip == nil {
				fmt.Fprintf(out, "\nNo chip here supports %q yet -- this looks like an unsupported board.\n", info.Target)
				fmt.Fprintln(out, "The chip is fine; this php-esp32 just has no board definition for it.")
				printChips(out, phpDir)
				fmt.Fprintf(out, "To add it, create boards/<chip>/chip.toml with target = %q (plus board.c and presets/).\n", info.Target)
				return nil
			}

			boards, _ := manifest.BoardsIn(phpDir, chip.Key)
			fmt.Fprintf(out, "\nBoard presets for %s (chip %q):\n", info.Target, chip.Name)
			for _, b := range boards {
				fmt.Fprintf(out, "  - %-32s %-24s network: %-13s microSD: %s\n",
					b.Key, b.Name, b.Network, yesno(b.MicroSD))
			}
			fmt.Fprintln(out, "Note: `network`/`microSD` are what each preset wires -- the chip alone")
			fmt.Fprintln(out, "can't tell the models apart (same silicon).")

			// A ready-to-paste snippet so wiring a project to this board is copyable, not retyped.
			if len(boards) > 0 {
				fmt.Fprintf(out, "\nTo target one from a project, set in %s:\n", config.FileName)
				fmt.Fprintln(out, "  [esp32]")
				if len(boards) == 1 {
					fmt.Fprintf(out, "  preset = %q\n", boards[0].Key)
				} else {
					fmt.Fprintf(out, "  preset = %q   # pick the model matching yours from the list above\n", boards[0].Key)
				}
			}

			if !all {
				fmt.Fprintln(out, "\nTo actually probe THIS board's peripherals (Ethernet, microSD) -- the way to")
				fmt.Fprintln(out, "identify a blank board -- run `phpflash discover --all`. It flashes a small probe")
				fmt.Fprintln(out, "firmware (erasing the app), so it asks first.")
				return nil
			}

			// --all: actively probe the peripherals with a discovery firmware, built PER CANDIDATE
			// PRESET (reusing the chip's board.c with that preset's wiring), then match.
			fmt.Fprintln(out, "\nWARNING: --all flashes a small discovery firmware to actively probe this")
			fmt.Fprintln(out, "board (Ethernet, microSD), building it once per candidate preset. This ERASES")
			fmt.Fprintln(out, "the app on the board; you'll need to re-flash your firmware afterwards")
			fmt.Fprintln(out, "(phpflash flash).")
			if !assumeYes && !confirm(cmd, "Continue?") {
				fmt.Fprintln(out, "Aborted -- nothing was flashed.")
				return nil
			}

			// Resolve each preset into its discovery-firmware build inputs. `wired` presets (EMAC/W5500)
			// have a link to detect, which is the clearest discriminator, so probe them first.
			type cand struct {
				info     manifest.BoardInfo
				buildDir string
				dargs    []string
				env      []string
				wired    bool
			}
			var cands []cand
			for _, b := range boards {
				bd, dargs, env, net, err := discoverFWInputs(phpDir, b.Key)
				if err != nil {
					return fmt.Errorf("resolve preset %q: %w", b.Key, err)
				}
				cands = append(cands, cand{b, bd, dargs, env, net == "emac" || net == "w5500"})
			}
			sort.SliceStable(cands, func(i, j int) bool { return cands[i].wired && !cands[j].wired })

			var matched *manifest.BoardInfo
			hadWired, cardSeen := false, false
			for i := range cands {
				c := cands[i]
				if c.wired {
					hadWired = true
				}
				fmt.Fprintf(out, "\n== Probing with the %s wiring ==\n", c.info.Key)
				if err := discover.BuildFlashDiscoverFW(idf, phpDir, c.buildDir, c.dargs, c.env, target, out); err != nil {
					return fmt.Errorf("discovery firmware (%s): %w", c.info.Key, err)
				}
				df := discover.ParseDiscoverFW(discover.ReadSerialUntil(idf, target, "DISCOVER-FW-END", 30))
				if !df.Seen {
					return fmt.Errorf("no output from the discovery firmware (%s)", c.info.Key)
				}
				printProbe(out, df)
				if df.MicroSD {
					cardSeen = true
				}
				if c.wired && df.Ethernet { // this preset's link came up -> it's this board
					matched = &cands[i].info
					break
				}
			}
			// Split the non-wired candidates by whether they carry a microSD slot.
			var nonWiredWithSD, nonWiredNoSD []manifest.BoardInfo
			for i := range cands {
				if cands[i].wired {
					continue
				}
				if cands[i].info.MicroSD {
					nonWiredWithSD = append(nonWiredWithSD, cands[i].info)
				} else {
					nonWiredNoSD = append(nonWiredNoSD, cands[i].info)
				}
			}
			if matched == nil {
				if cardSeen {
					if len(nonWiredWithSD) == 1 {
						matched = &nonWiredWithSD[0]
					}
				} else {
					allNonWired := append(append([]manifest.BoardInfo{}, nonWiredWithSD...), nonWiredNoSD...)
					if len(allNonWired) == 1 {
						matched = &allNonWired[0]
					}
				}
			}

			fmt.Fprintln(out)
			if matched != nil {
				rule := strings.Repeat("=", 52)
				fmt.Fprintln(out, rule)
				fmt.Fprintf(out, "  THIS BOARD IS:  %s\n", matched.Name)
				fmt.Fprintf(out, "  set in %s:\n", config.FileName)
				fmt.Fprintf(out, "    [esp32]\n    preset = %q\n", matched.Key)
				fmt.Fprintln(out, rule)
			} else {
				fmt.Fprintln(out, "Couldn't uniquely identify the board -- the probe results are above.")
				if hadWired {
					fmt.Fprintln(out, "If it has Ethernet, connect the cable (the probe needs the link up) and retry.")
				}
				if !cardSeen && len(nonWiredWithSD) > 0 && len(nonWiredNoSD) > 0 {
					fmt.Fprintln(out, "If it has a microSD slot, insert a card and retry; if it has neither")
					fmt.Fprintln(out, "network nor a card slot, it's the slotless variant.")
				}
			}

			fmt.Fprintln(out, "\nThe discovery firmware is on the board now -- re-flash your project:")
			fmt.Fprintln(out, "  phpflash flash        (or: phpflash build && phpflash flash)")
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path (for esptool / building the probe fw)")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path (boards list + discovery firmware)")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port to probe (default: first detected)")
	c.Flags().BoolVar(&noProbe, "no-probe", false, "don't probe the chip (just list ports); doesn't reset the board")
	c.Flags().BoolVar(&all, "all", false, "actively probe peripherals: flash a discovery firmware (ERASES the app, asks first)")
	c.Flags().BoolVarP(&assumeYes, "yes", "y", false, "with --all, skip the confirmation prompt")
	return c
}

// confirm asks a yes/no question on the command's I/O and returns true only for y/yes.
func confirm(cmd *cobra.Command, q string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", q)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

// printChips lists the chips this install knows about, for when the chip is unsupported or the probe
// was skipped.
func printChips(out io.Writer, phpDir string) {
	chips, _ := manifest.Families(phpDir)
	if len(chips) == 0 {
		fmt.Fprintln(out, "No chips found (is php-esp32 installed? try --php-esp32-path).")
		return
	}
	fmt.Fprintln(out, "Chips this php-esp32 supports:")
	for _, c := range chips {
		fmt.Fprintf(out, "  - %-10s %s (target %s)\n", c.Key, c.Name, c.Target)
	}
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "  " + s
}

func yesno(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func enabledStr(b bool) string {
	if b {
		return "enabled"
	}
	return "not set"
}

// usbDesc renders a port's USB identity, skipping the strings the device didn't expose.
func usbDesc(usb platform.USBInfo) string {
	parts := []string{"USB " + usb.Vendor + ":" + usb.Product}
	if usb.Manufacturer != "" {
		parts = append(parts, usb.Manufacturer)
	}
	if usb.ProductName != "" {
		parts = append(parts, fmt.Sprintf("%q", usb.ProductName))
	}
	return strings.Join(parts, " ")
}
