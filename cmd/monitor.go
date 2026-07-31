package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
	"phpflash/internal/config"
)

func newMonitorCmd() *cobra.Command {
	var idfPath, phpPath, port string
	c := &cobra.Command{
		Use:   "monitor",
		Short: "Open the serial monitor",
		RunE: func(cmd *cobra.Command, args []string) error {
			// monitor needs only the paths and a port -- no manifest/build args.
			cfg, _ := config.Load(config.FileName) // optional: a config supplies path/port defaults
			idf, phpDir := resolveDirs(idfPath, phpPath, cfg)
			p := port
			if p == "" && cfg != nil {
				p = cfg.Board.Port
			}
			inv := build.ExecInvoker{PhpEsp32Dir: phpDir, IdfPath: idf, Out: os.Stdout}
			return build.Monitor(inv, os.Stdout, projectBuildDir(), p)
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port (empty = autodetect)")
	return c
}
