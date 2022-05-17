package client

import (
	"context"
	"time"

	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/rest/types"
)

// Peers returns the list of peers.
func (c *Client) Peers(ctx context.Context, endpoint EndpointType) ([]types.Peer, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	peers := []types.Peer{}
	err := c.QueryStruct(queryCtx, "GET", endpoint, api.NewURL().Path("peers"), nil, &peers)

	return peers, err
}
