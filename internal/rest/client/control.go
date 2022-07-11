package client

import (
	"context"
	"time"

	"github.com/canonical/microcluster/internal/rest/types"
)

// client.ControlDaemon posts control data to the MicroCluster daemon.
func (c *Client) ControlDaemon(ctx context.Context, args types.Control) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.QueryStruct(queryCtx, "POST", ControlEndpoint, nil, args, nil)
}
