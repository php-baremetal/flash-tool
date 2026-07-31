package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
)

func newBuildCmd() *cobra.Command {
	var idfPath, phpPath string
	c := &cobra.Command{
		Use:   "build",
		Short: "Build the firmware for this project (drives ESP-IDF)",
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, err := loadBuildContext(idfPath, phpPath)
			if err != nil {
				return err
			}
			// Render the build as phases + a progress bar; keep the raw output so we
			// can replay it if the build fails.
			pw := build.NewProgress(os.Stdout)
			inv := build.ExecInvoker{PhpEsp32Dir: bc.phpDir, IdfPath: bc.idfPath, Out: pw}
			if err := build.Build(inv, os.Stdout, bc.buildDir, bc.sdkconfig, bc.dargs, bc.fetches); err != nil {
				pw.DumpRaw(os.Stderr)
				return err
			}
			fmt.Fprintln(os.Stdout, "\nBuild complete. Next: phpflash flash")
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	return c
}
