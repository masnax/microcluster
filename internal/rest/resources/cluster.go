package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
	"github.com/canonical/microcluster/internal/trust"
	"github.com/lxc/lxd/lxd/response"
)

var clusterCmd = rest.Endpoint{
	Path: "cluster",

	Post: rest.EndpointAction{Handler: clusterPost, AllowUntrusted: true},
	Get:  rest.EndpointAction{Handler: peersGet, AccessHandler: access.AllowAuthenticated},
}

func clusterPost(state *state.State, r *http.Request) response.Response {
	req := types.Cluster{}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	if !client.IsForwardedRequest(r) {
		cluster, err := state.Cluster(r)
		if err != nil {
			return response.SmartError(err)
		}

		err = cluster.Query(state.Context, true, func(ctx context.Context, c *client.Client) error {
			return c.AddClusterMember(ctx, req)
		})
		if err != nil {
			return response.SmartError(err)
		}
	}

	// Check if any of the remote's addresses are currently in use.
	for _, addr := range req.Addresses {
		existingRemote := state.Remotes().RemoteByAddress(addr)
		if existingRemote != nil {
			return response.SmartError(fmt.Errorf("Remote with address %q exists", addr.String()))
		}
	}

	// Add a trust store entry for the new member.
	// This will trigger a database update, unless another node has beaten us to it.
	err = state.Remotes().Add(state.OS.TrustDir, trust.Remote{Name: req.Name, Addresses: req.Addresses, Certificate: req.Certificate})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
