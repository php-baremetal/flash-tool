package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"phpflash/internal/config"
)

// The chip -> mode -> preset board model. A project's [esp32] selects either a named preset
// (`preset = "waveshare-esp32-s3-eth"`) or a chip/mode family (`family = "esp32-s3/octal"`, which
// uses that mode's generic preset). The preset is a thin TOML under boards/<chip>/<mode>/presets/
// (or boards/<chip>/presets/ for a mode-less chip); the board.c and REQUIRES come from the chip.
// Resolving one yields the -DBOARD key, the IDF target, the flash size, the display name, and the
// peripheral -D args (a generated sdkconfig fragment + pins + network type) the build needs.

// chipFile mirrors boards/<chip>/chip.toml.
type chipFile struct {
	Name         string `toml:"name"`
	Target       string `toml:"target"`
	Capabilities struct {
		Microsd []string `toml:"microsd"`
		Network []string `toml:"network"`
	} `toml:"capabilities"`
}

// metaFile mirrors boards/<chip>/<mode>/meta.toml.
type metaFile struct {
	DefaultFlash string `toml:"default_flash"`
}

// presetMicrosd / presetNetwork are a preset's (or, later, a project's) peripheral blocks. Pins are
// pointers so an unset pin (nil) falls back to the chip board.c's factory default instead of 0.
type presetMicrosd struct {
	Mode  string `toml:"mode"`
	CS    *int   `toml:"cs"`    // SPI SD (S3)
	SCLK  *int   `toml:"sclk"`  // SPI SD (S3)
	MOSI  *int   `toml:"mosi"`  // SPI SD (S3)
	MISO  *int   `toml:"miso"`  // SPI SD (S3)
	PwrEn *int   `toml:"pwr_en"` // optional card-VDD switch GPIO (e.g. P4-ETH GPIO45)
}

type presetNetwork struct {
	Type string `toml:"type"`
	CS   *int   `toml:"cs"`
	INT  *int   `toml:"int"`
	RST  *int   `toml:"rst"`
	SCLK *int   `toml:"sclk"`
	MOSI *int   `toml:"mosi"`
	MISO *int   `toml:"miso"`
}

// presetFile mirrors a boards/.../presets/<name>.toml.
type presetFile struct {
	Name    string `toml:"name"`
	Flash   string `toml:"flash"`
	Console string `toml:"console"`
	Storage struct {
		Microsd presetMicrosd `toml:"microsd"`
	} `toml:"storage"`
	Network presetNetwork `toml:"network"`
}

// boardResolution is the resolved result of a [board] preset/family selection.
type boardResolution struct {
	BoardKey  string   // -DBOARD value, e.g. "esp32-s3/octal"
	IDFTarget string   // e.g. "esp32s3"
	FlashKB   int      // flash size in KiB (0 if unknown)
	BoardName string   // display name (-DPHP_BOARD_NAME)
	Network   string   // wired network type, e.g. "w5500" ("" if none)
	DArgs     []string // -D args: preset sdkconfig fragment, network type, peripheral pins, board name
}

// resolveBoard resolves a project's [esp32] board selection in the chip->mode->preset model. It writes
// the preset's sdkconfig fragment under buildDir and validates the peripheral config against the chip's
// capabilities. A project must select a board by preset or family, else it errors.
func resolveBoard(cfg *config.Config, phpDir, buildDir string) (res boardResolution, err error) {
	var presetPath string
	switch {
	case cfg.Esp32.Preset != "":
		if presetPath, err = findPreset(phpDir, cfg.Esp32.Preset); err != nil {
			return res, err
		}
	case cfg.Esp32.Family != "":
		// family = "<chip>/<mode>" (or "<chip>" for a mode-less chip) -> that leaf's generic preset.
		presetPath = filepath.Join(phpDir, "boards", filepath.FromSlash(cfg.Esp32.Family), "presets", "generic.toml")
		if _, e := os.Stat(presetPath); e != nil {
			return res, fmt.Errorf("family %q: no generic preset at %s", cfg.Esp32.Family, presetPath)
		}
	default:
		return res, fmt.Errorf("no board selected: set [esp32] preset or family in %s", config.FileName)
	}

	var preset presetFile
	if _, err = toml.DecodeFile(presetPath, &preset); err != nil {
		return res, fmt.Errorf("preset %s: %w", presetPath, err)
	}

	// Walk up from the preset to the chip (the ancestor holding chip.toml) and the mode leaf between.
	presetsDir := filepath.Dir(presetPath) // .../presets
	modeDir := filepath.Dir(presetsDir)    // .../<mode>  or  .../<chip> for a mode-less chip
	chipDir, mode := modeDir, ""
	if _, e := os.Stat(filepath.Join(modeDir, "chip.toml")); e != nil {
		chipDir = filepath.Dir(modeDir)
		mode = filepath.Base(modeDir)
	}
	chipKey := filepath.Base(chipDir)
	res.BoardKey = chipKey
	if mode != "" {
		res.BoardKey = chipKey + "/" + mode
	}

	var chip chipFile
	if _, err = toml.DecodeFile(filepath.Join(chipDir, "chip.toml"), &chip); err != nil {
		return res, fmt.Errorf("chip %s: %w", chipKey, err)
	}
	res.IDFTarget = chip.Target
	res.BoardName = preset.Name

	// Validate the preset's peripherals against what the chip can actually drive.
	if preset.Storage.Microsd.Mode != "" && !contains(chip.Capabilities.Microsd, preset.Storage.Microsd.Mode) {
		return res, fmt.Errorf("preset %s: microSD mode %q not supported by %s (supports: %s)",
			filepath.Base(presetPath), preset.Storage.Microsd.Mode, chipKey, strings.Join(chip.Capabilities.Microsd, ", "))
	}
	if preset.Network.Type != "" && !contains(chip.Capabilities.Network, preset.Network.Type) {
		return res, fmt.Errorf("preset %s: network type %q not supported by %s (supports: %s)",
			filepath.Base(presetPath), preset.Network.Type, chipKey, strings.Join(chip.Capabilities.Network, ", "))
	}

	// Flash size: the preset's, else the mode's default (meta.toml).
	flashStr := preset.Flash
	if flashStr == "" && mode != "" {
		var meta metaFile
		if _, e := toml.DecodeFile(filepath.Join(modeDir, "meta.toml"), &meta); e == nil {
			flashStr = meta.DefaultFlash
		}
	}
	res.FlashKB = parseSizeK(flashStr)

	// The generated sdkconfig fragment (flash size + console + network driver) layered last by the
	// top CMakeLists.
	frag, err := writePresetFragment(buildDir, flashStr, preset.Console, preset.Network.Type)
	if err != nil {
		return res, err
	}
	res.DArgs = append(res.DArgs, "-DPHP_PRESET_SDKCONFIG="+frag)
	if res.BoardName != "" {
		res.DArgs = append(res.DArgs, "-DPHP_BOARD_NAME="+res.BoardName)
	}

	// Peripheral pins the board.c reads. microSD pins are always forwarded (whether the SD code is
	// compiled is a project storage_type decision, PHP_STORAGE_MICROSD); network pins + type go only
	// when the preset wires a network.
	appendPin(&res.DArgs, "PHP_SD_CS", preset.Storage.Microsd.CS)
	appendPin(&res.DArgs, "PHP_SD_CLK", preset.Storage.Microsd.SCLK)
	appendPin(&res.DArgs, "PHP_SD_MOSI", preset.Storage.Microsd.MOSI)
	appendPin(&res.DArgs, "PHP_SD_MISO", preset.Storage.Microsd.MISO)
	appendPin(&res.DArgs, "PHP_SD_PWR_EN", preset.Storage.Microsd.PwrEn)
	if preset.Network.Type != "" {
		res.Network = preset.Network.Type
		res.DArgs = append(res.DArgs, "-DPHP_NETWORK="+preset.Network.Type)
		appendPin(&res.DArgs, "PHP_ETH_CS", preset.Network.CS)
		appendPin(&res.DArgs, "PHP_ETH_INT", preset.Network.INT)
		appendPin(&res.DArgs, "PHP_ETH_RST", preset.Network.RST)
		appendPin(&res.DArgs, "PHP_ETH_SCLK", preset.Network.SCLK)
		appendPin(&res.DArgs, "PHP_ETH_MOSI", preset.Network.MOSI)
		appendPin(&res.DArgs, "PHP_ETH_MISO", preset.Network.MISO)
	}
	return res, nil
}

// findPreset locates a preset by name anywhere under boards/*/presets/ (mode-less chip) or
// boards/*/*/presets/ (chip/mode). Errors if absent or ambiguous.
func findPreset(phpDir, name string) (string, error) {
	var matches []string
	for _, pat := range []string{
		filepath.Join(phpDir, "boards", "*", "*", "presets", name+".toml"),
		filepath.Join(phpDir, "boards", "*", "presets", name+".toml"),
	} {
		m, _ := filepath.Glob(pat)
		matches = append(matches, m...)
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("preset %q not found under boards/*/presets/", name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("preset %q is ambiguous (%d matches): %s", name, len(matches), strings.Join(matches, ", "))
	}
}

// writePresetFragment writes the preset's sdkconfig fragment (flash size + console) into buildDir and
// returns its absolute path. Partitions are generated dynamically (gen-partitions.cmake), so no
// partition table is set here.
func writePresetFragment(buildDir, flashStr, console, network string) (string, error) {
	var b strings.Builder
	if flashStr != "" {
		fmt.Fprintf(&b, "CONFIG_ESPTOOLPY_FLASHSIZE_%s=y\nCONFIG_ESPTOOLPY_FLASHSIZE=\"%s\"\n", flashStr, flashStr)
	}
	if console == "usb-jtag" {
		// USB-C wired to the chip's native USB (GPIO19/20). "uart"/"" keeps the IDF default UART console.
		b.WriteString("CONFIG_ESP_CONSOLE_USB_SERIAL_JTAG=y\n")
	}
	if network == "w5500" {
		// The S3 has no internal Ethernet MAC: enable the SPI-Ethernet path and the W5500 driver (both
		// off by default), so board.c's esp_eth_mac_new_w5500() is declared and linkable.
		b.WriteString("CONFIG_ETH_USE_SPI_ETHERNET=y\nCONFIG_ETH_SPI_ETHERNET_W5500=y\n")
	}
	if network == "wifi-c6" {
		// The P4 has no on-die radio: esp_wifi_remote forwards esp_wifi_* to an on-board ESP32-C6 over
		// SDIO (esp_hosted transport). Select the C6 slave + the Function-EV-Board SDIO profile (the
		// reference pin mapping) and the CONFIG_WIFI_RMT_* buffers the forwarded API needs. Re-enable the
		// auto-init (sdkconfig.chip forces it off so a companion-less P4 doesn't spam SDIO probes): here
		// there IS a companion, so bring the transport up before app_main as ESP-HOSTED expects.
		b.WriteString("CONFIG_SLAVE_IDF_TARGET_ESP32C6=y\n" +
			"CONFIG_ESP_HOSTED_CP_TARGET_ESP32C6=y\n" +
			"CONFIG_ESP_HOSTED_P4_DEV_BOARD_FUNC_BOARD=y\n" +
			"CONFIG_ESP_HOSTED_AUTO_CALL_INIT_BEFORE_APP_MAIN=y\n" +
			"CONFIG_WIFI_RMT_STATIC_RX_BUFFER_NUM=16\n" +
			"CONFIG_WIFI_RMT_DYNAMIC_RX_BUFFER_NUM=64\n" +
			"CONFIG_WIFI_RMT_DYNAMIC_TX_BUFFER_NUM=64\n" +
			"CONFIG_WIFI_RMT_AMPDU_TX_ENABLED=y\n" +
			"CONFIG_WIFI_RMT_TX_BA_WIN=32\n" +
			"CONFIG_WIFI_RMT_AMPDU_RX_ENABLED=y\n" +
			"CONFIG_WIFI_RMT_RX_BA_WIN=32\n")
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(buildDir, "preset.sdkconfig.defaults")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// appendPin adds a -D<name>=<gpio> arg when the pin is set (nil = keep board.c's default).
func appendPin(dargs *[]string, name string, v *int) {
	if v != nil {
		*dargs = append(*dargs, fmt.Sprintf("-D%s=%d", name, *v))
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
