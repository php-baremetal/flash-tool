package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
)

func newBuildCmd() *cobra.Command {
	var idfPath, phpPath string
	var clean bool
	c := &cobra.Command{
		Use:   "build",
		Short: "Build the firmware for this project (drives ESP-IDF)",
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, err := loadBuildContext(idfPath, phpPath)
			if err != nil {
				return err
			}
			// A failed configure leaves negative results cached (e.g. "compiler identification is
			// unknown"), which then repeat even after the environment is fixed. --clean wipes the
			// build directory first so the next build reconfigures from scratch.
			if clean {
				fmt.Fprintf(os.Stdout, "Removing %s (clean build)\n", bc.buildDir)
				if err := os.RemoveAll(bc.buildDir); err != nil {
					return err
				}
			}
			// Render the build as phases + a progress bar; keep the raw output so we
			// can replay it if the build fails.
			pw := build.NewProgress(os.Stdout)
			inv := build.ExecInvoker{PhpEsp32Dir: bc.phpDir, IdfPath: bc.idfPath, Out: pw}
			if err := build.Build(inv, os.Stdout, bc.buildDir, bc.sdkconfig, bc.dargs, bc.fetches); err != nil {
				pw.DumpRaw(os.Stderr)
				fmt.Fprintln(os.Stderr, "\nIf you just fixed the toolchain, submodules or IDF version, retry with: phpflash build --clean")
				return err
			}
			fmt.Fprintln(os.Stdout, "\nBuild complete. Next: phpflash flash")
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().BoolVar(&clean, "clean", false, "remove the build directory before building (clears a poisoned CMake cache)")
	return c
}
