package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/manifest"
	"phpflash/internal/project"
	"phpflash/internal/prompt"
)

type initOpts struct {
	Dir         string
	Name        string
	Board       string
	PhpEsp32Dir string
	Yes         bool
	Force       bool
}

func newInitCmd() *cobra.Command {
	var o initOpts
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new PHP-on-ESP32 project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.Dir = args[0]
			} else {
				o.Dir = "."
			}
			if o.PhpEsp32Dir == "" {
				o.PhpEsp32Dir = config.Resolve(config.ResolveInputs{
					Env:     os.Getenv("PHP_ESP32_DIR"),
					Default: config.DefaultPhpEsp32Path(),
				}).Value
			}
			p := &prompt.Terminal{In: os.Stdin, Out: os.Stdout}
			return runInit(os.Stdout, p, o)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "project name (default: directory name)")
	cmd.Flags().StringVar(&o.Board, "board", "waveshare-esp32-p4-pico", "board preset")
	cmd.Flags().StringVar(&o.PhpEsp32Dir, "php-esp32-path", "", "path to the installed php-esp32")
	cmd.Flags().BoolVar(&o.Yes, "yes", false, "accept all defaults, no prompts")
	cmd.Flags().BoolVar(&o.Force, "force", false, "overwrite an existing config")
	return cmd
}

func runInit(out io.Writer, p prompt.Prompter, o initOpts) error {
	absDir, _ := filepath.Abs(o.Dir)
	defName := o.Name
	if defName == "" {
		defName = filepath.Base(absDir)
	}

	name := defName
	if !o.Yes {
		name, _ = p.Input("Project name", defName)
	}

	presetKey := o.Board
	storage, projType := "microsd", "init-loop"
	phpVersion := "" // empty = follow the repo default; pinned only if the user picks a non-default one
	exts := map[string]config.Extension{}

	// Read the installed php-esp32: choose a board preset (chip -> preset), then the modes
	// and extensions it offers. If it isn't installed, keep the flag/default preset and
	// skip these steps (the commands stay independent).
	var m *manifest.Manifest
	defaultVersion := ""
	if repo, err := manifest.LoadRepo(o.PhpEsp32Dir); err == nil {
		defaultVersion = repo.DefaultVersion
		m, _ = manifest.LoadManifest(o.PhpEsp32Dir, repo.DefaultVersion)
	}
	if m != nil {
		if !o.Yes {
			if fams, _ := manifest.Families(o.PhpEsp32Dir); len(fams) > 0 {
				chip := pickChip(p, fams)
				if presets, _ := manifest.BoardsIn(o.PhpEsp32Dir, chip.Key); len(presets) > 0 {
					presetKey = pickPreset(p, presets, o.Board).Key
				}
			}
		}
		// The chosen preset's peripherals decide which storage/project modes to offer.
		if info, ok := manifest.FindPreset(o.PhpEsp32Dir, presetKey); ok {
			board := presetBoard(info)
			s, pj := m.OfferedModes(&board)
			if !o.Yes {
				storage = pickMode(p, "Storage type", s, "microsd")
				projType = pickMode(p, "Project type", pj, "init-loop")
				// Offer the installed PHP versions when there's more than one. Choosing the default
				// leaves version empty (the project follows the repo default); any other pins it.
				if vers, _ := manifest.AvailableVersions(o.PhpEsp32Dir); len(vers) > 1 {
					if chosen := pickVersion(p, vers, defaultVersion); chosen != defaultVersion {
						phpVersion = chosen
					}
				}
			}
			exts = chooseExtensions(p, m, projType, o.Yes)
		}
	} else {
		fmt.Fprintln(out, "note: php-esp32 not installed; board and extensions default -- configure after system-setup")
	}

	port := ""
	starter := "hello"
	if !o.Yes {
		port, _ = p.Input("Serial port (empty = autodetect)", "")
		starter = pickStarter(p)
	}

	cfg := &config.Config{
		Name:       name,
		Type:       projType,
		Esp32:      config.Esp32Config{Preset: presetKey, Port: port, StorageType: storage},
		Extensions: exts,
		Php:        config.PhpConfig{Src: "project-src", Entry: "index.php", Version: phpVersion},
	}

	if o.Force {
		os.Remove(filepath.Join(absDir, config.FileName))
	}
	if err := project.Scaffold(project.Answers{Dir: absDir, Config: cfg, Starter: starter}); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s\n", filepath.Join(o.Dir, config.FileName))
	if o.Dir != "." {
		fmt.Fprintf(out, "\nNext: cd %s && phpflash build\n", o.Dir)
	} else {
		fmt.Fprintln(out, "\nNext: phpflash build")
	}
	return nil
}

// presetBoard synthesizes a manifest.Board from a preset's peripherals, so OfferedModes can decide
// which storage/project types to offer: a card slot adds `microsd`; a network (wired or the chip's
// on-die radio) adds `web-server`.
func presetBoard(info manifest.BoardInfo) manifest.Board {
	storage := []string{"embedded"}
	if info.MicroSD {
		storage = []string{"microsd", "embedded"}
	}
	project := []string{"init-loop", "event-driven"}
	if info.Network != "none" {
		project = append(project, "web-server")
	}
	return manifest.Board{StorageTypes: storage, ProjectTypes: project, Network: info.Network}
}

func pickChip(p prompt.Prompter, fams []manifest.Family) manifest.Family {
	opts := make([]prompt.Option, len(fams))
	for i, f := range fams {
		opts[i] = prompt.Option{Label: f.Name + " (" + f.Key + ")"}
	}
	i, _ := p.Select("Chip", opts, 0)
	return fams[i]
}

func pickPreset(p prompt.Prompter, presets []manifest.BoardInfo, def string) manifest.BoardInfo {
	opts := make([]prompt.Option, len(presets))
	d := 0
	for i, b := range presets {
		opts[i] = prompt.Option{Label: b.Name + " (" + b.Key + ")"}
		if b.Key == def {
			d = i
		}
	}
	i, _ := p.Select("Board preset", opts, d)
	return presets[i]
}

func pickMode(p prompt.Prompter, label string, modes []manifest.Mode, def string) string {
	if len(modes) == 0 {
		return def
	}
	opts := make([]prompt.Option, len(modes))
	d := 0
	for i, m := range modes {
		opts[i] = prompt.Option{Label: m.Key}
		if m.Key == def {
			d = i
		}
	}
	i, _ := p.Select(label, opts, d)
	return modes[i].Key
}

func chooseExtensions(p prompt.Prompter, m *manifest.Manifest, projType string, yes bool) map[string]config.Extension {
	out := map[string]config.Extension{}
	if yes {
		return out
	}
	optional := m.OptionalFor(projType)
	if len(optional) == 0 {
		return out
	}
	opts := make([]prompt.Option, len(optional))
	for i, e := range optional {
		opts[i] = prompt.Option{Label: e.Key + " — " + e.Description}
	}
	chosen, _ := p.MultiSelect("Optional extensions", opts)
	for _, idx := range chosen {
		e := optional[idx]
		ext := config.Extension{Enabled: true}
		for _, s := range e.Settings {
			if s.Kind == "enum" {
				opts := make([]prompt.Option, len(s.Choices))
				def := 0
				for i, c := range s.Choices {
					label := c
					if c == s.EnumDefault() {
						label, def = c+" (default)", i
					}
					opts[i] = prompt.Option{Label: label}
				}
				idx, _ := p.Select("  "+e.Key+" "+s.Key+": "+s.Description, opts, def)
				if v := s.Choices[idx]; v != s.EnumDefault() {
					if ext.Options == nil {
						ext.Options = map[string]string{}
					}
					ext.Options[s.Key] = v
				}
				continue
			}
			def, _ := s.Default.(bool)
			on, _ := p.Confirm("  "+e.Key+": "+s.Description, def)
			if on {
				if ext.Settings == nil {
					ext.Settings = map[string]bool{}
				}
				ext.Settings[s.Key] = true
			}
		}
		out[e.Key] = ext
	}
	return out
}

func pickVersion(p prompt.Prompter, vers []string, def string) string {
	opts := make([]prompt.Option, len(vers))
	d := 0
	for i, v := range vers {
		label := v
		if v == def {
			label = v + " (default)"
			d = i
		}
		opts[i] = prompt.Option{Label: label}
	}
	i, _ := p.Select("PHP version", opts, d)
	return vers[i]
}

func pickStarter(p prompt.Prompter) string {
	i, _ := p.Select("Starter PHP", []prompt.Option{{Label: "hello"}, {Label: "blink"}}, 0)
	if i == 1 {
		return "blink"
	}
	return "hello"
}
