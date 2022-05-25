package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
	"github.com/canonical/microcluster/internal/version"
)

func main() {
	// common flags.
	commonCmd := control.CmdControl{}

	app := &cobra.Command{
		Use:               "microcluster-cellctl",
		Short:             "LXD Cloud - The LXD based Private Cloud Manager (cell)",
		Version:           version.Version,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	app.PersistentFlags().StringVar(&commonCmd.FlagStateDir, "state-dir", "", "Path to store LXD Cloud state information"+"``")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&commonCmd.FlagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&commonCmd.FlagLogVerbose, "verbose", "v", false, "Show all information messages")

	app.SetVersionTemplate("{{.Version}}\n")

	var cmdInit = cmdInit{common: &commonCmd}
	app.AddCommand(cmdInit.Command())

	var cmdPeers = cmdPeers{common: &commonCmd}
	app.AddCommand(cmdPeers.Command())

	var cmdReload = cmdReload{common: &commonCmd}
	app.AddCommand(cmdReload.Command())

	var cmdShutdown = cmdShutdown{common: &commonCmd}
	app.AddCommand(cmdShutdown.Command())

	var cmdSQL = cmdSQL{common: &commonCmd}
	app.AddCommand(cmdSQL.Command())

	var cmdSecrets = cmdSecrets{common: &commonCmd}
	app.AddCommand(cmdSecrets.Command())

	var cmdWaitready = cmdWaitready{common: &commonCmd}
	app.AddCommand(cmdWaitready.Command())

	app.InitDefaultHelpCmd()

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
