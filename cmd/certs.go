package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"phpflash/internal/build"
	"phpflash/internal/config"
)

// newUpdateCertsCmd refreshes the project's TLS CA bundle: it re-copies the host's root-CA store
// into the project at certs_path (the folder from [extensions.openssl] certs_path), OVERWRITING the
// current bundle. Use it to pick up new or renewed root CAs from the host. `phpflash build` ships
// the bundle the first time but never overwrites it, so this is the way to refresh it.
func newUpdateCertsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update-certs",
		Short: "Refresh the TLS CA bundle in the project (re-copy the host root CAs into certs_path)",
		Long: "Re-copy the host's root-CA bundle into the project at certs_path, overwriting the\n" +
			"existing one. The full openssl build with the `tls` setting ships this bundle so the\n" +
			"device can verify HTTPS peers; `phpflash build` writes it once but won't overwrite it,\n" +
			"so run this to update it (renewed roots, a different certs_source, etc.).",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.FileName)
			if err != nil {
				return fmt.Errorf("no %s in this directory (run `phpflash init` first): %w", config.FileName, err)
			}
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			source, dest, err := build.RefreshTLSCerts(cfg, wd)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "updated CA bundle: %s -> %s\n", source, dest)
			return nil
		},
	}
	return c
}
