package main

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/canonical/microcluster/internal/daemon"
	"github.com/canonical/microcluster/internal/logger"
	"github.com/canonical/microcluster/internal/version"
)

// Debug indicates whether to log debug messages or not.
var Debug bool

// Verbose indicates verbosity.
var Verbose bool

type cmdGlobal struct {
	cmd *cobra.Command //nolint:structcheck,unused // FIXME: Remove the nolint flag when this is in use.

	flagHelp    bool
	flagVersion bool

	flagLogDebug   bool
	flagLogVerbose bool
}

func (c *cmdGlobal) Run(cmd *cobra.Command, args []string) error {
	Debug = c.flagLogDebug
	Verbose = c.flagLogVerbose

	return logger.InitLogger("", c.flagLogDebug)
}

type cmdDaemon struct {
	global *cmdGlobal

	flagStateDir     string
	flagAdminAddr    string
	flagConsumerAddr string
}

func (c *cmdDaemon) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "microcluster-celld",
		Short:   "LXD Cloud - The LXD based Private Cloud Manager (cell daemon)",
		Version: version.Version,
	}

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	defer logger.Info("Daemon stopped")
	d := daemon.NewDaemon()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGPWR)
	signal.Notify(sigCh, unix.SIGINT)
	signal.Notify(sigCh, unix.SIGQUIT)
	signal.Notify(sigCh, unix.SIGTERM)

	chIgnore := make(chan os.Signal, 1)
	signal.Notify(chIgnore, unix.SIGHUP)

	err := d.Init(c.flagAdminAddr, c.flagStateDir)
	if err != nil {
		return err
	}

	for {
		select {
		case sig := <-sigCh:
			logCtx := logger.WithCtx(logger.Ctx{"signal": sig})
			logCtx.Info("Received signal")
			if d.ShutdownCtx.Err() != nil {
				logCtx.Warn("Ignoring signal, shutdown already in progress")
			} else {
				go func() {
					d.ShutdownDoneCh <- d.Stop(context.Background(), sig)
				}()
			}

		case err = <-d.ShutdownDoneCh:
			return err
		}
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	daemonCmd := cmdDaemon{global: &cmdGlobal{}}
	app := daemonCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}
	app.PersistentPreRunE = daemonCmd.global.Run

	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&daemonCmd.global.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogVerbose, "verbose", "v", false, "Show all information messages")

	app.PersistentFlags().StringVar(&daemonCmd.flagStateDir, "state-dir", "", "Path to store LXD Cloud state information"+"``")
	app.PersistentFlags().StringVar(&daemonCmd.flagAdminAddr, "admin-address", "", "Address:Port to bind for the admin API"+"``")
	app.PersistentFlags().StringVar(&daemonCmd.flagConsumerAddr, "consumer-address", "", "Address:Port to bind for the consumer API"+"``")

	app.SetVersionTemplate("{{.Version}}\n")

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
