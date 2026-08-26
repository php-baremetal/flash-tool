package cmd

import "github.com/spf13/cobra"

// Version is the phpflash version, set at release time via
// -ldflags "-X phpflash/cmd.Version=<tag>". "dev" for local builds.
var Version = "dev"

// NewRootCmd builds the phpflash root command with its subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "phpflash",
		Short:   "Create, configure and set up PHP-on-ESP32 projects",
		Version: Version,
		// A command that fails at runtime isn't a usage error: main already prints the error, so
		// don't let cobra dump the flag help + a duplicate "Error:" line on top of it.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newSystemSetupCmd())
	root.AddCommand(newBuildCmd())
	root.AddCommand(newFlashCmd())
	root.AddCommand(newMonitorCmd())
	root.AddCommand(newUpdateCertsCmd())
	root.AddCommand(newDiscoverCmd())
	root.AddCommand(newExtCmd())
	root.AddCommand(newPartitionsCmd())
	return root
}

// Execute runs the CLI.
func Execute() error {
	return NewRootCmd().Execute()
}
