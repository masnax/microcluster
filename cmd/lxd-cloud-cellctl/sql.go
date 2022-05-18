package main

import (
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/cmd/control"
)

type cmdSQL struct {
	common *control.CmdControl
}

func (c *cmdSQL) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Execute a SQL query against the LXD Cloud cell",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdSQL) Run(cmd *cobra.Command, args []string) error {
	return c.common.RunSQL(cmd, args)
}
