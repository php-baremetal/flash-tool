package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
	"phpflash/internal/config"
	"phpflash/internal/platform"
)

func newMonitorCmd() *cobra.Command {
	var idfPath, phpPath, port, until string
	var duration int
	var reset bool
	c := &cobra.Command{
		Use:   "monitor",
		Short: "Open the serial monitor",
		Long: "Open the serial monitor. By default this is the interactive ESP-IDF monitor (needs a\n" +
			"terminal). Pass --duration or --until for a non-interactive read that streams the output\n" +
			"and exits -- for scripts, CI, or capturing a boot log without a TTY.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// monitor needs only the paths and a port -- no manifest/build args.
			cfg, _ := config.Load(config.FileName) // optional: a config supplies path/port defaults
			idf, phpDir := resolveDirs(idfPath, phpPath, cfg)
			var cfgPort string
			if cfg != nil {
				cfgPort = cfg.Esp32.Port
			}
			p := resolvePort(port, cfgPort)
			if err := platform.CheckPortAccess(p); err != nil {
				return err
			}
			// Non-interactive (no TTY) when a stop condition is given: stream for a bounded time or
			// until a marker appears, then exit.
			if duration > 0 || until != "" {
				d := duration
				if d == 0 {
					d = 30 // a default cap so --until can't hang forever
				}
				return build.SerialRead(idf, p, d, until, reset, os.Stdout)
			}
			inv := build.ExecInvoker{PhpEsp32Dir: phpDir, IdfPath: idf, Out: os.Stdout}
			return build.Monitor(inv, os.Stdout, projectBuildDir(), p)
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port (empty = /dev/ttyACM*, then autodetect)")
	c.Flags().IntVarP(&duration, "duration", "d", 0, "non-interactive: read for N seconds, then exit")
	c.Flags().StringVarP(&until, "until", "u", "", "non-interactive: read until this marker appears (capped by --duration, default 30s)")
	c.Flags().BoolVar(&reset, "reset", true, "with --duration/--until: reset the board first to catch boot output (--reset=false to tail a running app)")
	return c
}
