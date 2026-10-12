package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"phpflash/internal/config"
)

// repoPhpDir returns the php-esp32 repo checkout this flash-tool lives beside
// (../../php-esp32 relative to this file). Skips when it (or its S3 chip dir) isn't present,
// so the test is a no-op wherever the sibling checkout is absent (CI with only the Go module).
func repoPhpDir(t *testing.T) string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "php-esp32")
	if _, err := os.Stat(filepath.Join(dir, "boards", "esp32-s3", "chip.toml")); err != nil {
		t.Skip("no sibling php-esp32 checkout with the S3 chip boards")
	}
	return dir
}

func hasArg(dargs []string, prefix string) bool {
	for _, a := range dargs {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func TestResolveBoardPresets(t *testing.T) {
	phpDir := repoPhpDir(t)
	build := t.TempDir()

	cases := []struct {
		name      string
		preset    string
		family    string
		wantBoard string
		wantTarget string
		wantFlash int
		wantNet   string
		wantArgs  []string // extra -D args the preset must emit (pins / power-enable)
	}{
		{name: "quad-zero", preset: "waveshare-esp32-s3-zero", wantBoard: "esp32-s3/quad", wantTarget: "esp32s3", wantFlash: 4096},
		{name: "octal-devkitc", preset: "espressif-esp32-s3-devkitc-1-n8r8", wantBoard: "esp32-s3/octal", wantTarget: "esp32s3", wantFlash: 8192},
		{name: "octal-n16", preset: "espressif-esp32-s3-devkitc-1-n16r8", wantBoard: "esp32-s3/octal", wantTarget: "esp32s3", wantFlash: 16384},
		{name: "octal-eth", preset: "waveshare-esp32-s3-eth", wantBoard: "esp32-s3/octal", wantTarget: "esp32s3", wantFlash: 16384, wantNet: "w5500", wantArgs: []string{"-DPHP_ETH_CS=14", "-DPHP_SD_CS=4"}},
		{name: "family-octal", family: "esp32-s3/octal", wantBoard: "esp32-s3/octal", wantTarget: "esp32s3", wantFlash: 8192},
		// P4: a mode-less chip (BoardKey is the bare chip), SDIO SD (no SPI pins forwarded), EMAC/wifi-c6.
		{name: "p4-pico", preset: "waveshare-esp32-p4-pico", wantBoard: "esp32-p4", wantTarget: "esp32p4", wantFlash: 32768},
		{name: "p4-eth", preset: "waveshare-esp32-p4-eth", wantBoard: "esp32-p4", wantTarget: "esp32p4", wantFlash: 32768, wantNet: "emac", wantArgs: []string{"-DPHP_SD_PWR_EN=45", "-DPHP_ETH_RST=51"}},
		{name: "p4-wifi6", preset: "waveshare-esp32-p4-wifi6", wantBoard: "esp32-p4", wantTarget: "esp32p4", wantFlash: 32768, wantNet: "wifi-c6"},
		{name: "p4-family", family: "esp32-p4", wantBoard: "esp32-p4", wantTarget: "esp32p4", wantFlash: 32768},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Esp32.Preset = tc.preset
			cfg.Esp32.Family = tc.family
			res, err := resolveBoard(cfg, phpDir, build)
			if err != nil {
				t.Fatalf("resolveBoard: %v", err)
			}
			if res.BoardKey != tc.wantBoard {
				t.Errorf("BoardKey = %q, want %q", res.BoardKey, tc.wantBoard)
			}
			if res.IDFTarget != tc.wantTarget {
				t.Errorf("IDFTarget = %q, want %q", res.IDFTarget, tc.wantTarget)
			}
			if res.FlashKB != tc.wantFlash {
				t.Errorf("FlashKB = %d, want %d", res.FlashKB, tc.wantFlash)
			}
			if res.Network != tc.wantNet {
				t.Errorf("Network = %q, want %q", res.Network, tc.wantNet)
			}
			if !hasArg(res.DArgs, "-DPHP_PRESET_SDKCONFIG=") {
				t.Error("missing -DPHP_PRESET_SDKCONFIG")
			}
			if tc.wantNet != "" && !hasArg(res.DArgs, "-DPHP_NETWORK="+tc.wantNet) {
				t.Errorf("missing -DPHP_NETWORK=%s in %v", tc.wantNet, res.DArgs)
			}
			for _, want := range tc.wantArgs {
				if !hasArg(res.DArgs, want) {
					t.Errorf("missing %s in %v", want, res.DArgs)
				}
			}
		})
	}
}

func TestResolveBoardErrors(t *testing.T) {
	phpDir := repoPhpDir(t)
	build := t.TempDir()

	// No board selected -> error.
	cfg := &config.Config{}
	if _, err := resolveBoard(cfg, phpDir, build); err == nil {
		t.Error("no board selected: want error")
	}

	// Unknown preset -> error.
	cfg = &config.Config{}
	cfg.Esp32.Preset = "does-not-exist"
	if _, err := resolveBoard(cfg, phpDir, build); err == nil {
		t.Error("unknown preset: want error")
	}
}
