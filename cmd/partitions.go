package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/manifest"
)

func newPartitionsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "partitions",
		Short: "Manage the project's partition table",
	}
	c.AddCommand(newPartitionsPublishCmd())
	return c
}

func newPartitionsPublishCmd() *cobra.Command {
	var phpFlag string
	var force bool
	c := &cobra.Command{
		Use:   "publish",
		Short: "Copy the board's partition table into the project for customization",
		Long: "Write ./" + config.PartitionsFileName + " from the configured board's defaults, with guidance\n" +
			"comments, so you can customize the fixed partitions (e.g. resize the app `factory`).\n" +
			"`phpflash build` then uses this file instead of the board's committed table; the\n" +
			"`storage`/`phpstore` partitions stay auto-generated ([storage] reserve_kb / [store] size_kb).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.FileName)
			if err != nil {
				return fmt.Errorf("no %s in this directory (run `phpflash init` first): %w", config.FileName, err)
			}
			_, phpDir := resolveDirs("", phpFlag, cfg)
			boardDir, ok := manifest.BoardDir(phpDir, cfg.Board.Target)
			if !ok {
				return fmt.Errorf("board %q not found in %s (run `phpflash system-setup`?)", cfg.Board.Target, phpDir)
			}
			out := config.PartitionsFileName
			if _, err := os.Stat(out); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", out)
			}
			content, err := renderProjectPartitions(boardDir, cfg.Board.Target)
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
				return err
			}
			fmt.Printf("created ./%s (from board %q)\n", out, cfg.Board.Target)
			fmt.Printf("Edit the fixed partitions; `phpflash build` uses it automatically (delete to revert).\n")
			return nil
		},
	}
	c.Flags().StringVar(&phpFlag, "php-esp32-path", "", "path to the php-esp32 checkout (else config/env/default)")
	c.Flags().BoolVar(&force, "force", false, "overwrite an existing "+config.PartitionsFileName)
	return c
}

// renderProjectPartitions builds the project partitions.csv from a board's committed table: a
// guidance header, the column header, and the board's fixed partition rows (nvs, phy_init,
// factory, ...). The auto-generated `storage`/`phpstore` rows are dropped -- the build appends them.
func renderProjectPartitions(boardDir, boardKey string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(boardDir, config.PartitionsFileName))
	if err != nil {
		return "", fmt.Errorf("read board partition table: %w", err)
	}
	var rows []string
	defFactoryK := 0
	for _, line := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		fields := strings.Split(t, ",")
		name := strings.TrimSpace(fields[0])
		if name == "storage" || name == "phpstore" {
			continue // auto-generated per build; never authored by hand
		}
		if name == "factory" && len(fields) >= 5 {
			defFactoryK = parseSizeK(fields[4])
		}
		rows = append(rows, strings.TrimRight(line, " \t"))
	}
	flash := boardFlashSize(boardDir)
	var b strings.Builder
	b.WriteString(partitionsHeader(boardKey, flash, factoryTable(boardKey, parseSizeK(flash), defFactoryK)))
	b.WriteString("# Name,     Type, SubType,  Offset,   Size\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	return b.String(), nil
}

// parseSizeK parses a partition/flash size (e.g. "3456K", "3M", "4MB") into KiB. 0 when unparseable.
func parseSizeK(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "B") // "4MB" -> "4M"
	mult := 1
	switch {
	case strings.HasSuffix(s, "K"):
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		s, mult = strings.TrimSuffix(s, "M"), 1024
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n * mult
}

// factoryTable renders a comment block listing sensible `factory` sizes for a board, derived from its
// flash size and committed default. Empty when the flash size or default can't be read. The rest of
// the flash (bootloader/partition-table/nvs/phy) is roughly 96 KiB; every option leaves at least
// 128 KiB for the generated storage/phpstore partitions.
func factoryTable(boardKey string, flashK, defK int) string {
	if flashK == 0 || defK == 0 {
		return ""
	}
	const reserveK = 96 // bootloader + partition table + nvs + phy_init, approx
	usableK := flashK - reserveK
	align := func(k int) int { return (k / 64) * 64 }
	maxK := align(usableK - 128)
	if maxK <= 0 {
		return ""
	}
	type opt struct {
		k    int
		note string
	}
	cands := []opt{
		{align(defK - 384), "roomier embedded source / store"},
		{defK, "board default"},
		{align(defK + 256), "tighter -- small source only"},
		{maxK, "max embedded (little room for source/store)"},
	}
	seen := map[int]bool{}
	var opts []opt
	for _, c := range cands {
		if c.k < 1024 || c.k > maxK || seen[c.k] {
			continue
		}
		seen[c.k] = true
		opts = append(opts, c)
	}
	sort.Slice(opts, func(i, j int) bool { return opts[i].k < opts[j].k })
	if len(opts) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Sensible `factory` sizes for %s (%dK usable of %dK flash):\n", boardKey, usableK, flashK)
	b.WriteString("#   factory   free       note\n")
	for _, o := range opts {
		free := fmt.Sprintf("~%dK", usableK-o.k)
		fmt.Fprintf(&b, "#   %-8s  %-9s  %s\n", strconv.Itoa(o.k)+"K", free, o.note)
	}
	b.WriteString("#\n")
	return b.String()
}

// boardFlashSize reads the human flash size (e.g. "4MB") from a board's sdkconfig.board, for the
// guidance header. Returns "" when it can't be found.
func boardFlashSize(boardDir string) string {
	raw, err := os.ReadFile(filepath.Join(boardDir, "sdkconfig.board"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `CONFIG_ESPTOOLPY_FLASHSIZE=`) && strings.Contains(line, `"`) {
			if parts := strings.SplitN(line, `"`, 3); len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func partitionsHeader(boardKey, flash, factoryTable string) string {
	fl := "the board's flash"
	if flash != "" {
		fl = flash + " flash"
	}
	return "# Project partition table -- published by `phpflash partitions publish` from board \"" + boardKey + "\".\n" +
		"#\n" +
		"# It overrides the board's committed table for THIS project only; `phpflash build` picks it up\n" +
		"# automatically just by being here (delete it to fall back to the board default).\n" +
		"#\n" +
		"# You may change:\n" +
		"#   - factory (the app partition): resize it. It must hold the ~3 MB firmware AND leave room\n" +
		"#     for the auto-generated storage/phpstore partitions within " + fl + ". App partitions\n" +
		"#     start on a 64 KB boundary. See the size table below.\n" +
		"#   - nvs: 16K-64K is plenty (Wi-Fi calibration + IDF NVS).\n" +
		"#   - add your own `data` partitions if you need them.\n" +
		"#\n" +
		factoryTable +
		"# Do NOT add these here -- they are generated per build and appended automatically:\n" +
		"#   - storage  (embedded PHP source FAT)  -> size it with [storage] reserve_kb\n" +
		"#   - phpstore (persistent store_* NVS)   -> enable it with [store] size_kb\n" +
		"# Any storage/phpstore line here is ignored.\n" +
		"#\n" +
		"# Sizes take K/M suffixes; leave Offset blank to auto-place.\n" +
		"#\n"
}
