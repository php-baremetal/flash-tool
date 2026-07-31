package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/setup"
)

// execRunner is the real Runner: it shells out to git and to the repos' scripts.
type execRunner struct{}

func (execRunner) Git(args ...string) error {
	c := exec.Command("git", args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func (execRunner) Shell(dir, script string, args ...string) error {
	c := exec.Command(filepath.Join(dir, script), args...)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func (execRunner) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func newSystemSetupCmd() *cobra.Command {
	var idfPath, idfVersion, phpPath, phpVersion string
	cmd := &cobra.Command{
		Use:   "system-setup",
		Short: "Install the global prerequisites (ESP-IDF + php-esp32)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg *config.Config
			if c, err := config.Load(config.FileName); err == nil {
				cfg = c
			}
			plan := setup.Plan{
				IdfPath:         resolvePath(idfPath, cfgIdfPath(cfg), os.Getenv("IDF_PATH"), config.DefaultIdfPath()),
				IdfVersion:      firstNonEmpty(idfVersion, cfgIdfVersion(cfg), "v5.5.5"),
				PhpEsp32Path:    resolvePath(phpPath, cfgPhpPath(cfg), os.Getenv("PHP_ESP32_DIR"), config.DefaultPhpEsp32Path()),
				PhpEsp32Version: firstNonEmpty(phpVersion, cfgPhpVersion(cfg), "master"), // versioning not active yet
			}
			return setup.Run(execRunner{}, os.Stdout, plan)
		},
	}
	cmd.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	cmd.Flags().StringVar(&idfVersion, "idf-version", "", "ESP-IDF git ref")
	cmd.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	cmd.Flags().StringVar(&phpVersion, "php-esp32-version", "", "php-esp32 git ref")
	return cmd
}

func resolvePath(flag, cfg, env, def string) string {
	return config.Resolve(config.ResolveInputs{Flag: flag, Config: cfg, Env: env, Default: def}).Value
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func cfgIdfPath(c *config.Config) string {
	if c != nil {
		return c.EspIdf.Path
	}
	return ""
}

func cfgIdfVersion(c *config.Config) string {
	if c != nil {
		return c.EspIdf.Version
	}
	return ""
}

func cfgPhpPath(c *config.Config) string {
	if c != nil {
		return c.PhpEsp32.Path
	}
	return ""
}

func cfgPhpVersion(c *config.Config) string {
	if c != nil {
		return c.PhpEsp32.Version
	}
	return ""
}
