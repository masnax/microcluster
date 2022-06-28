package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/microcluster/internal/db"
	"github.com/canonical/microcluster/internal/db/cluster"
	"github.com/canonical/microcluster/internal/logger"
	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
	"github.com/canonical/microcluster/internal/trust"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared/api"
)

var clusterCmd = rest.Endpoint{
	Path: "cluster",

	Post: rest.EndpointAction{Handler: clusterPost, AllowUntrusted: true},
	Get:  rest.EndpointAction{Handler: clusterGet, AccessHandler: access.AllowAuthenticated},
}

func clusterPost(state *state.State, r *http.Request) response.Response {
	req := types.ClusterMember{}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	leaderClient, err := state.Database.Leader()
	if err != nil {
		return response.SmartError(err)
	}

	leaderInfo, err := leaderClient.Leader(state.Context)
	if err != nil {
		return response.SmartError(err)
	}

	// Check if any of the remote's addresses are currently in use.
	existingRemote := state.Remotes().RemoteByAddress(req.Address)
	if existingRemote != nil {
		return response.SmartError(fmt.Errorf("Remote with address %q exists", req.Address.String()))
	}

	// Forward request to leader.
	if leaderInfo.Address != state.Address.URL.Host {
		client, err := state.Leader()
		if err != nil {
			return response.SmartError(err)
		}

		err = client.AddClusterMember(state.Context, req)
		if err != nil {
			return response.SmartError(err)
		}

		// If we are not the leader, just add the cluster member to our local store for authentication.
		err = state.Remotes().Add(state.OS.TrustDir, trust.Remote{Name: req.Name, Address: req.Address, Certificate: req.Certificate})
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}

	var clusterMembers []types.ClusterMember
	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		dbClusterMembers, err := cluster.GetClusterMembers(ctx, tx, cluster.ClusterMemberFilter{})
		if err != nil {
			return err
		}

		for _, clusterMember := range dbClusterMembers {
			if req.Address.String() == clusterMember.Address {
				return fmt.Errorf("Remote with address %q exists", req.Address.String())
			}
		}

		dbClusterMember := cluster.ClusterMember{
			Name:        req.Name,
			Address:     req.Address.String(),
			Certificate: req.Certificate.String(),
			Schema:      req.SchemaVersion,
			Heartbeat:   time.Time{},
			Role:        "pending",
		}

		_, err = cluster.CreateClusterMember(ctx, tx, dbClusterMember)
		if err != nil {
			return err
		}

		dbClusterMembers, err = cluster.GetClusterMembers(ctx, tx, cluster.ClusterMemberFilter{})
		if err != nil {
			return err
		}

		clusterMembers = make([]types.ClusterMember, 0, len(dbClusterMembers))
		for _, clusterMember := range dbClusterMembers {
			apiClusterMember, err := clusterMember.ToAPI()
			if err != nil {
				return err
			}

			clusterMembers = append(clusterMembers, *apiClusterMember)
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	err = state.Remotes().Replace(state.OS.TrustDir, clusterMembers...)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func clusterGet(state *state.State, r *http.Request) response.Response {
	var apiClusterMembers []types.ClusterMember
	err := state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		clusterMembers, err := cluster.GetClusterMembers(ctx, tx, cluster.ClusterMemberFilter{})
		if err != nil {
			return err
		}

		apiClusterMembers = make([]types.ClusterMember, 0, len(clusterMembers))
		for _, clusterMember := range clusterMembers {
			apiClusterMember, err := clusterMember.ToAPI()
			if err != nil {
				return err
			}

			apiClusterMembers = append(apiClusterMembers, *apiClusterMember)
		}

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get cluster members: %w", err))
	}

	clusterCert, err := state.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	for i, clusterMember := range apiClusterMembers {
		addr := api.NewURL().Scheme("https").Host(clusterMember.Address.String())
		d, err := client.New(*addr, state.ServerCert(), clusterCert, false)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to create HTTPS client for cluster member with address %q: %w", addr.String(), err))
		}

		err = d.QueryStruct(state.Context, "GET", client.InternalEndpoint, api.NewURL().Path("ready"), nil, nil)
		if err == nil {
			apiClusterMembers[i].Status = types.MemberOnline
		} else {
			logger.Warnf("Failed to get status of cluster member with address %q: %v", addr.String(), err)
		}
	}

	return response.SyncResponse(true, apiClusterMembers)
}
