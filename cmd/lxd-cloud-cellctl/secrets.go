package main

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/internal/cmd/control"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"
)

type cmdSecrets struct {
	common *control.CmdControl
}

func (c *cmdSecrets) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets <server.crt>",
		Short: "Configure the LXD Cloud cell",
		RunE:  c.Run,
		Example: `  microcluster-cellctl init --bootstrap
  microcluster-cellctl init --join <address> --token <token>`,
	}

	return cmd
}

func (c *cmdSecrets) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	os, err := c.common.GetStateDir()
	if err != nil {
		return err
	}

	d, err := client.New(os.ControlSocket(), nil, nil, false)
	if err != nil {
		return err
	}

	cert, err := shared.ReadCert(args[0])
	if err != nil {
		return fmt.Errorf("MAW: %w", err)
	}

	secret, err := d.RequestToken(context.Background(), shared.CertFingerprint(cert))
	if err != nil {
		return err
	}

	fmt.Println(secret)

	return nil
}
