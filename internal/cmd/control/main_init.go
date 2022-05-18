package control

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
)

// RunInit initialises the cell/region daemon by either bootstrapping or joining an existing cluster.
func (c *CmdControl) RunInit(flagBootstrap bool, flagJoin string, flagToken string, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Help()
	}

	os, err := c.GetStateDir()
	if err != nil {
		return err
	}

	d, err := client.New(os.ControlSocket(), nil, nil, false)
	if err != nil {
		return err
	}

	if flagBootstrap && flagJoin != "" || flagBootstrap && flagToken != "" || flagJoin != "" && flagToken != "" {
		return fmt.Errorf("Option must be one of bootstrap, join, or token")
	}

	data := types.Control{
		Bootstrap: flagBootstrap,
		JoinToken: flagToken,
	}

	err = d.ControlDaemon(context.Background(), data)
	if err != nil {
		return err
	}

	return nil
}
