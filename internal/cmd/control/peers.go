package control

import (
	"context"
	"sort"

	"github.com/lxc/lxd/lxc/utils"
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/rest/client"
)

// RunPeers lists the peers for the cell/region.
func (c *CmdControl) RunPeers(cmd *cobra.Command, args []string) error {
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

	peers, err := d.GetClusterMembers(context.Background(), client.ControlEndpoint)
	if err != nil {
		return err
	}

	data := make([][]string, len(peers))
	for i, peer := range peers {
		data[i] = []string{peer.Name, peer.Address.String(), peer.Role, peer.Certificate.String(), string(peer.Status)}
	}

	header := []string{"NAME", "ADDRESS", "ROLE", "CERTIFICATE", "STATUS"}
	sort.Sort(utils.ByName(data))

	return utils.RenderTable(utils.TableFormatTable, header, data, peers)
}
