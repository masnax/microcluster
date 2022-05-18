package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdShutdown struct {
	common *control.CmdControl
}

func (c *cmdShutdown) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown the LXD Cloud cell daemon",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdShutdown) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunShutdown(cmd, args)
}
