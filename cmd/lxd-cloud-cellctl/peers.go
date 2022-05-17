package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdPeers struct {
	common *control.CmdControl
}

func (c *cmdPeers) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peers",
		Short: "List all peers for this LXD Cloud cell",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdPeers) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunPeers(cmd, args)
}
