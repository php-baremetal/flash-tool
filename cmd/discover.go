package cmd

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/discover"
	"phpflash/internal/manifest"
	"phpflash/internal/platform"
)

// hasNet reports whether a board declares a network interface (used to order/interpret probes).
func hasNet(b manifest.BoardInfo) bool { return b.Network != "" && b.Network != "none" }

// printProbe renders one candidate board's discovery-firmware result.
func printProbe(out io.Writer, df discover.DiscoverFW) {
	fmt.Fprintf(out, "  board wiring: %s\n", df.Board)
	switch {
	case df.EthernetNA:
		fmt.Fprintln(out, "  Ethernet:     n/a (this board has no network hardware)")
	case df.Ethernet:
		fmt.Fprintf(out, "  Ethernet:     up (%s)\n", df.IP)
	default:
		fmt.Fprintln(out, "  Ethernet:     no link (no PHY on these pins, or no cable/DHCP)")
	}
	switch {
	case df.MicroSDNA:
		fmt.Fprintln(out, "  microSD:      n/a (this board has no card slot)")
	case df.MicroSD:
		fmt.Fprintf(out, "  microSD:      card present (%s)\n", df.CardSize)
	default:
		fmt.Fprintln(out, "  microSD:      no card (empty slot or none)")
	}
	if df.PSRAM != "" {
		fmt.Fprintf(out, "  PSRAM:        %s\n", df.PSRAM)
	}
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
				printFamilies(out, phpDir)
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

			// Board-first mapping: which supported boards fit this chip?
			families, _ := manifest.Families(phpDir)
			fam := discover.FamilyForTarget(families, info.Target)
			if fam == nil {
				fmt.Fprintf(out, "\nNo board here supports %q yet -- this looks like an unsupported board.\n", info.Target)
				fmt.Fprintln(out, "The chip is fine; this php-esp32 just has no board definition for it.")
				printFamilies(out, phpDir)
				fmt.Fprintf(out, "To add it, create boards/<family>/%s/ (family.toml with target = %q, plus board.toml).\n", info.Target, info.Target)
				return nil
			}

			boards, _ := manifest.BoardsIn(phpDir, fam.Key)
			fmt.Fprintf(out, "\nSupported boards for %s (family %q):\n", info.Target, fam.Name)
			for _, b := range boards {
				fmt.Fprintf(out, "  - %-15s %-16s network: %-9s microSD: %-3s (-DBOARD=%s)\n",
					b.Key, b.Name, b.Network, yesno(b.MicroSD), b.Key)
			}
			fmt.Fprintln(out, "Note: `network`/`microSD` above are what each board *model* carries (from its")
			fmt.Fprintln(out, "board.toml) -- the chip alone can't tell the models apart (same silicon).")

			// A ready-to-paste snippet so wiring a project to this board is copyable, not retyped.
			if len(boards) > 0 {
				fmt.Fprintf(out, "\nTo target one from a project, set in %s:\n", config.FileName)
				fmt.Fprintln(out, "  [board]")
				if len(boards) == 1 {
					fmt.Fprintf(out, "  target = %q\n", boards[0].Key)
				} else {
					fmt.Fprintf(out, "  target = %q   # pick the model matching yours from the list above\n", boards[0].Key)
				}
			}

			if !all {
				fmt.Fprintln(out, "\nTo actually probe THIS board's peripherals (Ethernet, microSD) -- the way to")
				fmt.Fprintln(out, "identify a blank board -- run `phpflash discover --all`. It flashes a small probe")
				fmt.Fprintln(out, "firmware (erasing the app), so it asks first.")
				return nil
			}

			// --all: actively probe the peripherals with a discovery firmware, built PER CANDIDATE
			// BOARD (reusing its board.c) so the probe uses each board's real GPIO wiring, then match.
			fmt.Fprintln(out, "\nWARNING: --all flashes a small discovery firmware to actively probe this")
			fmt.Fprintln(out, "board (Ethernet, microSD), building it once per candidate board. This ERASES")
			fmt.Fprintln(out, "the app on the board; you'll need to re-flash your firmware afterwards")
			fmt.Fprintln(out, "(phpflash flash).")
			if !assumeYes && !confirm(cmd, "Continue?") {
				fmt.Fprintln(out, "Aborted -- nothing was flashed.")
				return nil
			}

			// Network-capable boards first: their link coming up is the clearest discriminator and
			// lets us stop early.
			ordered := append([]manifest.BoardInfo{}, boards...)
			sort.SliceStable(ordered, func(i, j int) bool { return hasNet(ordered[i]) && !hasNet(ordered[j]) })

			var matched *manifest.BoardInfo
			hadNetCandidate := false
			cardSeen := false // any probe mounted a card -> the board physically has an SD slot
			for i := range ordered {
				b := ordered[i]
				if hasNet(b) {
					hadNetCandidate = true
				}
				fmt.Fprintf(out, "\n== Probing with the %s wiring ==\n", b.Key)
				if err := discover.BuildFlashDiscoverFW(idf, phpDir, b.Key, target, out); err != nil {
					return fmt.Errorf("discovery firmware (%s): %w", b.Key, err)
				}
				df := discover.ParseDiscoverFW(discover.ReadSerialUntil(idf, target, "DISCOVER-FW-END", 30))
				if !df.Seen {
					return fmt.Errorf("no output from the discovery firmware (%s)", b.Key)
				}
				printProbe(out, df)
				if df.MicroSD {
					cardSeen = true
				}
				if hasNet(b) && df.Ethernet { // this board's network came up -> it's this board
					matched = &ordered[i]
					break
				}
			}
			// Split the non-network candidates by whether they carry a microSD slot.
			var nonNetWithSD, nonNetNoSD []manifest.BoardInfo
			for i := range boards {
				if hasNet(boards[i]) {
					continue
				}
				if boards[i].MicroSD {
					nonNetWithSD = append(nonNetWithSD, boards[i])
				} else {
					nonNetNoSD = append(nonNetNoSD, boards[i])
				}
			}
			if matched == nil {
				if cardSeen {
					// A card mounted -> the board HAS a slot, which rules out the slotless -zero
					// variant. Pick the single non-network board that has a slot.
					if len(nonNetWithSD) == 1 {
						matched = &nonNetWithSD[0]
					}
				} else {
					// Nothing detected (no link, no card). This can't distinguish a slotless -zero
					// board from a board with an empty slot, so only decide if there is a single
					// non-network candidate overall.
					allNonNet := append(append([]manifest.BoardInfo{}, nonNetWithSD...), nonNetNoSD...)
					if len(allNonNet) == 1 {
						matched = &allNonNet[0]
					}
				}
			}

			fmt.Fprintln(out)
			if matched != nil {
				rule := strings.Repeat("=", 52)
				fmt.Fprintln(out, rule)
				fmt.Fprintf(out, "  THIS BOARD IS:  %s\n", matched.Name)
				fmt.Fprintf(out, "  build with:     -DBOARD=%s\n", matched.Key)
				fmt.Fprintln(out, rule)
			} else {
				fmt.Fprintln(out, "Couldn't uniquely identify the board -- the probe results are above.")
				if hadNetCandidate {
					fmt.Fprintln(out, "If it has Ethernet, connect the cable (the probe needs the link up) and retry.")
				}
				// Ambiguous between a slotless -zero board and an SD board with an empty slot:
				// inserting a card settles it (a mounted card rules out -zero).
				if !cardSeen && len(nonNetWithSD) > 0 && len(nonNetNoSD) > 0 {
					fmt.Fprintln(out, "If it has a microSD slot, insert a card and retry; if it has neither")
					fmt.Fprintln(out, "network nor a card slot, it's the slotless variant (e.g. -zero).")
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

// printFamilies lists the chip families this install knows about, for when the chip is unsupported
// or the probe was skipped.
func printFamilies(out io.Writer, phpDir string) {
	families, _ := manifest.Families(phpDir)
	if len(families) == 0 {
		fmt.Fprintln(out, "No board families found (is php-esp32 installed? try --php-esp32-path).")
		return
	}
	fmt.Fprintln(out, "Chip families this php-esp32 supports:")
	for _, f := range families {
		fmt.Fprintf(out, "  - %-10s %s (target %s)\n", f.Key, f.Name, f.Target)
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
