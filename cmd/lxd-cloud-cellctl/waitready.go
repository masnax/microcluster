package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdWaitready struct {
	common *control.CmdControl

	flagTimeout int
}

func (c *cmdWaitready) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "waitready",
		Short: "Wait for the LXD Cloud cell daemon to be ready to process requests",
		RunE:  c.Run,
	}

	cmd.Flags().IntVarP(&c.flagTimeout, "timeout", "t", 0, "Number of seconds to wait before giving up"+"``")

	return cmd
}

func (c *cmdWaitready) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunWaitready(c.flagTimeout, cmd, args)
}
