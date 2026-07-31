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
	cmd.Flags().StringVar(&o.Board, "board", "esp32-p4-pico", "board / target")
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

	boardKey := o.Board
	storage, projType := "microsd", "init-loop"
	exts := map[string]config.Extension{}

	// Read the installed php-esp32: choose a board (family -> board), then the modes
	// and extensions it offers. If it isn't installed, keep the flag/default board and
	// skip these steps (the commands stay independent).
	var m *manifest.Manifest
	if repo, err := manifest.LoadRepo(o.PhpEsp32Dir); err == nil {
		m, _ = manifest.LoadManifest(o.PhpEsp32Dir, repo.DefaultVersion)
	}
	if m != nil {
		if !o.Yes {
			if fams, _ := manifest.Families(o.PhpEsp32Dir); len(fams) > 0 {
				fam := pickFamily(p, fams)
				if boards, _ := manifest.BoardsIn(o.PhpEsp32Dir, fam.Key); len(boards) > 0 {
					boardKey = pickBoard(p, boards, o.Board)
				}
			}
		}
		if board, err := manifest.LoadBoard(o.PhpEsp32Dir, fullBoard(boardKey, o.PhpEsp32Dir)); err == nil {
			s, pj := m.OfferedModes(board)
			if !o.Yes {
				storage = pickMode(p, "Storage type", s, "microsd")
				projType = pickMode(p, "Project type", pj, "init-loop")
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
		Name:        name,
		StorageType: storage,
		Type:        projType,
		Board:       config.BoardConfig{Target: boardKey, Port: port},
		Extensions:  exts,
		Php:         config.PhpConfig{Src: "project-src", Entry: "index.php"},
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

// fullBoard accepts "family/board" as-is; a bare board name is resolved against boards/*/.
func fullBoard(board, phpEsp32Dir string) string {
	if filepath.Dir(board) != "." {
		return board
	}
	matches, _ := filepath.Glob(filepath.Join(phpEsp32Dir, "boards", "*", board))
	if len(matches) == 1 {
		rel, _ := filepath.Rel(filepath.Join(phpEsp32Dir, "boards"), matches[0])
		return filepath.ToSlash(rel)
	}
	return board
}

func pickFamily(p prompt.Prompter, fams []manifest.Family) manifest.Family {
	opts := make([]prompt.Option, len(fams))
	for i, f := range fams {
		opts[i] = prompt.Option{Label: f.Name + " (" + f.Key + ")"}
	}
	i, _ := p.Select("Chip family", opts, 0)
	return fams[i]
}

func pickBoard(p prompt.Prompter, boards []manifest.BoardInfo, def string) string {
	opts := make([]prompt.Option, len(boards))
	d := 0
	for i, b := range boards {
		opts[i] = prompt.Option{Label: b.Name + " (" + b.Key + ")"}
		if b.Key == def {
			d = i
		}
	}
	i, _ := p.Select("Board", opts, d)
	return boards[i].Key
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
			on, _ := p.Confirm("  "+e.Key+": "+s.Description, s.Default)
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

func pickStarter(p prompt.Prompter) string {
	i, _ := p.Select("Starter PHP", []prompt.Option{{Label: "hello"}, {Label: "blink"}}, 0)
	if i == 1 {
		return "blink"
	}
	return "hello"
}
