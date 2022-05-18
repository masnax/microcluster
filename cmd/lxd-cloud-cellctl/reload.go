package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdReload struct {
	common *control.CmdControl
}

func (c *cmdReload) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload the LXD Cloud cell daemon",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdReload) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunReload(cmd, args)
}
