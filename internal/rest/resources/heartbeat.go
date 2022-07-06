package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lxc/lxd/lxd/response"

	"github.com/canonical/microcluster/internal/db"
	"github.com/canonical/microcluster/internal/db/cluster"
	"github.com/canonical/microcluster/internal/logger"
	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
)

var heartbeatCmd = rest.Endpoint{
	Path: "heartbeat",

	Post: rest.EndpointAction{Handler: heartbeatPost, AllowUntrusted: true},
}

func heartbeatPost(state *state.State, r *http.Request) response.Response {
	var hbInfo types.HeartbeatInfo
	err := json.NewDecoder(r.Body).Decode(&hbInfo)
	if err != nil {
		return response.SmartError(err)
	}

	if hbInfo.BeginRound {
		return beginHeartbeat(state, r)
	}

	// If we are not beginning a heartbeat, we are receiving one sent by the leader,
	// so we should update our local store of cluster members with the data from the heartbeat.
	clusterMemberList := []types.ClusterMember{}
	for _, clusterMember := range hbInfo.ClusterMembers {
		clusterMemberList = append(clusterMemberList, clusterMember)
	}

	err = state.Remotes().Replace(state.OS.TrustDir, clusterMemberList...)
	if err != nil {
		return response.SmartError(err)
	}

	// TODO: If our schema version is behind, we should try to update here.

	return response.EmptySyncResponse
}

// beginHeartbeat initiates a heartbeat from the leader node to all other cluster members, if we haven't sent one out
// recently.
func beginHeartbeat(state *state.State, r *http.Request) response.Response {
	// Only a leader can begin a heartbeat round.
	leader, err := state.Database.Leader()
	if err != nil {
		return response.SmartError(err)
	}

	defer leader.Close()

	leaderInfo, err := leader.Leader(state.Context)
	if err != nil {
		return response.SmartError(err)
	}

	if state.Address.URL.Host != leaderInfo.Address {
		return response.SmartError(fmt.Errorf("Attempt to initiate heartbeat from non-leader"))
	}

	// Get the database record of cluster members.
	var clusterMembers []types.ClusterMember
	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		dbClusterMembers, err := cluster.GetClusterMembers(ctx, tx, cluster.ClusterMemberFilter{})
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

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Get dqlite record of cluster members.
	dqliteCluster, err := state.Database.Cluster()
	if err != nil {
		return response.SmartError(err)
	}

	dqliteMap := map[string]string{}
	for _, member := range dqliteCluster {
		dqliteMap[member.Address] = member.Role.String()
	}

	// Update database with dqlite member roles.
	clusterMap := map[string]types.ClusterMember{}
	for _, clusterMember := range clusterMembers {
		role, ok := dqliteMap[clusterMember.Address.String()]

		// If a cluster member is pending and dqlite does not have a record for it yet, then skip it this round.
		if !ok && clusterMember.Role == string(cluster.Pending) {
			continue
		}

		clusterMember.Role = role
		clusterMap[clusterMember.Address.String()] = clusterMember
	}

	// If we sent out a heartbeat within double the request timeout,
	// then wait the up to half the request timeout before exiting to prevent sending more unsuccessful attempts.
	leaderEntry := clusterMap[state.Address.URL.Host]
	heartbeatInterval := time.Duration(time.Second * client.HeartbeatTimeout * 2)
	timeSinceLast := time.Since(leaderEntry.LastHeartbeat)
	if timeSinceLast < heartbeatInterval {
		sleepInterval := time.Duration(time.Second * client.HeartbeatTimeout / 2)
		if timeSinceLast < sleepInterval {
			sleepInterval = sleepInterval - timeSinceLast
		}

		<-time.After(sleepInterval)
		return response.EmptySyncResponse
	}

	// Update local record of cluster members from the database, including any pending nodes for authentication.
	err = state.Remotes().Replace(state.OS.TrustDir, clusterMembers...)
	if err != nil {
		return response.SmartError(err)
	}

	// Set the time of the last heartbeat to now.
	leaderEntry.LastHeartbeat = time.Now()
	clusterMap[state.Address.URL.Host] = leaderEntry

	// Record the maximum schema version discovered.
	hbInfo := types.HeartbeatInfo{ClusterMembers: clusterMap}
	for _, node := range clusterMembers {
		if node.SchemaVersion > hbInfo.MaxSchema {
			hbInfo.MaxSchema = node.SchemaVersion
		}
	}

	clusterClients, err := state.Cluster(nil)
	if err != nil {
		return response.SmartError(err)
	}

	// Send heartbeat to non-leader members, updating their local member cache and updating the node.
	// If we sent a heartbeat to this node within double the request timeout, then we can skip the node this round.
	err = clusterClients.Query(state.Context, true, func(ctx context.Context, c *client.Client) error {
		addr := c.URL().URL.Host
		currentMember, ok := hbInfo.ClusterMembers[addr]
		if !ok {
			logger.Warnf("Skipping heartbeat cluster member record with address %v due to pending status", addr)
			return nil
		}

		timeSinceLast := time.Since(currentMember.LastHeartbeat)
		if timeSinceLast < time.Duration(time.Second*client.HeartbeatTimeout*2) {
			logger.Warnf("Skipping heartbeat, one was sent %q ago", timeSinceLast.String())
			return nil
		}

		err := c.SendHeartbeat(ctx, hbInfo)
		if err != nil {
			logger.Error("Received error sending heartbeat to cluster member", logger.Ctx{"target": addr, "error": err})
			return nil
		}

		currentMember.LastHeartbeat = time.Now()
		hbInfo.ClusterMembers[addr] = currentMember

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Having sent a heartbeat to each valid cluster member, update the database record of members.
	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		dbClusterMembers, err := cluster.GetClusterMembers(ctx, tx, cluster.ClusterMemberFilter{})
		if err != nil {
			return err
		}

		for _, clusterMember := range dbClusterMembers {
			heartbeatInfo, ok := hbInfo.ClusterMembers[clusterMember.Address]
			if !ok {
				continue
			}

			clusterMember.Heartbeat = heartbeatInfo.LastHeartbeat
			clusterMember.Role = cluster.Role(heartbeatInfo.Role)
			err = cluster.UpdateClusterMember(ctx, tx, clusterMember.Address, clusterMember)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
