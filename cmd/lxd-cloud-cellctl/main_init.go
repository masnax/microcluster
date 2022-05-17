package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdInit struct {
	common *control.CmdControl

	flagBootstrap bool
	flagJoin      string
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure the LXD Cloud cell",
		RunE:  c.Run,
		Example: `  microcluster-cellctl init --bootstrap
  microcluster-cellctl init --join <address>`,
	}

	cmd.Flags().BoolVar(&c.flagBootstrap, "bootstrap", false, "Configure a standalone cell")
	cmd.Flags().StringVar(&c.flagJoin, "join", "", "Join the cell at the given address")
	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunInit(c.flagBootstrap, c.flagJoin, cmd, args)
}
