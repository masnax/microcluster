package client

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/lxc/lxd/shared/api"
)

// Cluster is a list of clients belonging to a cluster.
type Cluster []Client

// SelectRandom returns a randomly selected client.
func (c Cluster) SelectRandom() Client {
	return c[rand.Intn(len(c))]
}

// Query executes the given hook across all of the clients.
func (c Cluster) Query(ctx context.Context, concurrent bool, query func(context.Context, *Client) error) error {
	if !concurrent {
		for _, client := range c {
			err := query(ctx, &client)
			if err != nil {
				return err
			}
		}

		return nil
	}

	errors := make([]error, 0, len(c))
	mut := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, client := range c {
		wg.Add(1)
		go func(client Client) {
			defer wg.Done()
			err := query(ctx, &client)
			if err != nil {
				mut.Lock()
				errors = append(errors, err)
				mut.Unlock()
				return
			}
		}(client)
	}

	// Wait for all queries to complete and check for any errors.
	wg.Wait()
	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// ControlDaemon posts control data to the cell/region daemon.
func (c *Client) AddClusterMember(ctx context.Context, args types.Cluster) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.QueryStruct(queryCtx, "POST", InternalEndpoint, api.NewURL().Path("cluster"), args, nil)
}

// ControlDaemon posts control data to the cell/region daemon.
func (c *Client) InternalAddClusterMember(ctx context.Context, args types.Cluster) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.QueryStruct(queryCtx, "POST", InternalEndpoint, api.NewURL().Path("internal", "cluster"), args, nil)
}
