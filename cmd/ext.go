package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/templates"
)

// An extension name becomes both the directory and the C symbol `<name>_module_entry`, so it must
// be a valid C identifier: lowercase keeps the generated code and the filesystem tidy.
var extNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func newExtCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ext",
		Short: "Manage the project's custom C extensions (./firmware/exts)",
	}
	c.AddCommand(newExtNewCmd())
	return c
}

func newExtNewCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new C extension under ./firmware/exts/<name>",
		Long: "Scaffold a new C extension. It creates ./firmware/exts/<name>/<name>.c with a working\n" +
			"skeleton (a module entry named <name>_module_entry plus two example functions). phpflash\n" +
			"build then compiles it into the firmware. See docs/custom-extensions.md.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if _, err := os.Stat(config.FileName); err != nil {
				fmt.Fprintf(os.Stderr, "note: no %s here -- run this from your project directory\n",
					config.FileName)
			}
			file, err := scaffoldExt(".", name, force)
			if err != nil {
				return err
			}
			fmt.Printf("created %s\n", file)
			fmt.Printf("\nIt exposes %s_hello() and %s_add($a, $b). Add your own functions, then\n", name, name)
			fmt.Printf("`phpflash build` compiles it in -- guard use with function_exists('%s_hello').\n", name)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite an existing extension file")
	return c
}

// scaffoldExt writes firmware/exts/<name>/<name>.c under baseDir from the extension template and
// returns the created path. It validates the name (a lowercase C identifier), and refuses an
// existing file unless force is set.
func scaffoldExt(baseDir, name string, force bool) (string, error) {
	if !extNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid extension name %q: use lowercase letters, digits and "+
			"underscores, starting with a letter (it becomes the C symbol %s_module_entry)",
			name, name)
	}
	dir := filepath.Join(baseDir, "firmware", "exts", name)
	file := filepath.Join(dir, name+".c")
	if _, err := os.Stat(file); err == nil && !force {
		return "", fmt.Errorf("%s already exists (use --force to overwrite)", file)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	t, err := template.New("ext").Parse(templates.ExtC)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Name string }{Name: name}); err != nil {
		return "", err
	}
	if err := os.WriteFile(file, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return file, nil
}
