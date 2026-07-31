package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
)

func newFlashCmd() *cobra.Command {
	var idfPath, phpPath, port string
	c := &cobra.Command{
		Use:   "flash",
		Short: "Build and flash the firmware to the board",
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, err := loadBuildContext(idfPath, phpPath)
			if err != nil {
				return err
			}
			p := port
			if p == "" {
				p = bc.port
			}
			inv := build.ExecInvoker{PhpEsp32Dir: bc.phpDir, IdfPath: bc.idfPath, Out: os.Stdout}
			return build.Flash(inv, os.Stdout, bc.buildDir, bc.dargs, p)
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path")
	c.Flags().StringVar(&phpPath, "php-esp32-path", "", "php-esp32 path")
	c.Flags().StringVarP(&port, "port", "p", "", "serial port (empty = autodetect)")
	return c
}
