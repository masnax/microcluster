package resources

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	dqliteClient "github.com/canonical/go-dqlite/client"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/gorilla/mux"
	"golang.org/x/sys/unix"

	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/cluster"
	"github.com/canonical/microcluster/internal/db/update"
	internalClient "github.com/canonical/microcluster/internal/rest/client"
	internalTypes "github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
	"github.com/canonical/microcluster/internal/trust"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/rest/access"
	"github.com/canonical/microcluster/rest/types"
)

var clusterCmd = rest.Endpoint{
	Path: "cluster",

	Put:  rest.EndpointAction{Handler: clusterPut, AccessHandler: access.AllowAuthenticated},
	Post: rest.EndpointAction{Handler: clusterPost, AllowUntrusted: true, AccessHandler: access.RestrictNotification},
	Get:  rest.EndpointAction{Handler: clusterGet, AccessHandler: access.AllowAuthenticated},
}

var clusterMemberCmd = rest.Endpoint{
	Path: "cluster/{name}",

	Put:    rest.EndpointAction{Handler: clusterMemberPut, AccessHandler: access.AllowAuthenticated},
	Delete: rest.EndpointAction{Handler: clusterMemberDelete, AccessHandler: access.AllowClusterMembers},
}

var clusterMemberUpgradeCmd = rest.Endpoint{
	Path: "cluster/{name}/upgrade",

	Put: rest.EndpointAction{Handler: upgradeClusterMember, AccessHandler: access.AllowAuthenticated},
}

func upgradeClusterMember(s *state.State, r *http.Request) response.Response {
	ctx, cancel := context.WithTimeout(s.Context, time.Second*30)
	defer cancel()

	req := internalTypes.ClusterMemberUpgrade{}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	clusterCert, err := s.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	nonClusterRemotes := s.Remotes(trust.NonCluster)
	clusterRemotes := s.Remotes(trust.Cluster)

	remote := s.Remotes(trust.NonCluster).RemotesByName()[name]

	if remote.Name == "" {
		return response.SmartError(fmt.Errorf("Non-cluster member %q not found", name))
	}

	if clusterRemotes.RemotesByName()[name].Role == trust.Cluster {
		return response.SmartError(fmt.Errorf("%q is already a cluster member", name))
	}

	// Forward the request to the node to be upgraded so it can properly set the schema version and run its hooks.
	if s.Name() != name && !client.IsForwardedRequest(r) {
		c, err := internalClient.New(remote.URL(), s.ServerCert(), clusterCert, false)
		if err != nil {
			return response.SmartError(err)
		}

		err = c.UpgradeClusterMember(ctx, req)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}

	// If we received a cluster notification, instruct the leader to register the node as a cluster member.
	if s.Role() == trust.Cluster && client.IsForwardedRequest(r) {
		leaderClient, err := s.Database.Leader(ctx)
		if err != nil {
			return response.SmartError(err)
		}

		leaderInfo, err := leaderClient.Leader(ctx)
		if err != nil {
			return response.SmartError(err)
		}

		// Forward the request to the leader.
		if leaderInfo.Address != s.Address().URL.Host {
			leader, err := s.Leader()
			if err != nil {
				return response.SmartError(err)
			}

			leader.SetClusterNotification()
			err = leader.UpgradeClusterMember(ctx, req)
			if err != nil {
				return response.SmartError(err)
			}

			return response.EmptySyncResponse
		}

		clusterClients, err := s.Cluster(r, trust.Cluster)
		if err != nil {
			return response.SmartError(err)
		}

		// Fetch the clients now before we start changing the truststore.
		nonCluster, err := s.Cluster(r, trust.NonCluster)
		if err != nil {
			return response.SmartError(err)
		}

		// Update the database.
		err = s.Database.Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
			dbClusterMember := cluster.InternalClusterMember{
				Name:        remote.Name,
				Address:     remote.Address.String(),
				Certificate: remote.Certificate.String(),
				Schema:      req.SchemaVersion,
				Heartbeat:   time.Time{},
				Role:        cluster.Pending,
			}

			err = cluster.DeleteInternalNonClusterMember(ctx, tx, remote.Address.String())
			if err != nil {
				return err
			}

			_, err = cluster.CreateInternalClusterMember(ctx, tx, dbClusterMember)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		// Update the truststore.
		nonClusterRemotesMap := nonClusterRemotes.RemotesByName()

		newRemote := nonClusterRemotesMap[name]
		newRemote.Role = trust.Cluster
		delete(nonClusterRemotesMap, name)

		nonClusterList := make([]internalTypes.ClusterMember, 0, len(nonClusterRemotesMap))

		for _, remote := range nonClusterRemotesMap {
			nonClusterList = append(nonClusterList, internalTypes.ClusterMember{
				ClusterMemberLocal: internalTypes.ClusterMemberLocal{
					Name:        remote.Name,
					Address:     remote.Address,
					Certificate: remote.Certificate,
				}})
		}

		err = s.Remotes(trust.NonCluster).Replace(s.OS.TrustDir, nonClusterList...)
		if err != nil {
			return response.SmartError(err)
		}

		err = s.Remotes(trust.Cluster).Add(s.OS.TrustDir, newRemote)
		if err != nil {
			return response.SmartError(err)
		}

		// Instruct all the other non-cluster members to update their local stores.
		for _, c := range []client.Cluster{clusterClients, nonCluster} {
			err = c.Query(ctx, true, func(ctx context.Context, c *client.Client) error {
				// No need to send a request to ourselves, or to the node we are adding.
				if s.Address().URL.Host == c.URL().URL.Host || remote.URL().URL.Host == c.URL().URL.Host {
					return nil
				}

				info := internalTypes.ClusterMemberLocal{Name: remote.Name, Address: remote.Address, Certificate: remote.Certificate}
				err := c.RegisterClusterMember(ctx, internalTypes.ClusterMember{ClusterMemberLocal: info}, string(trust.Cluster), true)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				return response.SmartError(err)
			}
		}

		return response.EmptySyncResponse
	}

	err = state.PreUpgradeHook(s, req.InitConfig)
	if err != nil {
		return response.SmartError(err)
	}

	// Instruct a random cluster member to upgrade our status.
	randomRemote := s.Remotes(trust.Cluster).SelectRandom()
	c, err := internalClient.New(randomRemote.URL(), s.ServerCert(), clusterCert, true)
	if err != nil {
		return response.SmartError(err)
	}

	req.SchemaVersion = update.Schema().Version()
	err = c.UpgradeClusterMember(ctx, req)
	if err != nil {
		return response.SmartError(err)
	}

	err = s.UpgradeAPI(&remote.Location)

	cluster, err := s.Cluster(r, trust.Cluster)
	if err != nil {
		return response.SmartError(err)
	}

	// Instruct cluster nodes to run the OnUpgradedMember hook. hook.
	err = cluster.Query(ctx, true, func(ctx context.Context, c *client.Client) error {
		// No need to send a request to ourselves, or to the node we are adding.
		if s.Address().URL.Host == c.URL().URL.Host || remote.URL().URL.Host == c.URL().URL.Host {
			return nil
		}

		info := internalTypes.ClusterMemberLocal{Name: remote.Name, Address: remote.Address, Certificate: remote.Certificate}
		_, err := c.AddClusterMember(ctx, internalTypes.ClusterMember{ClusterMemberLocal: info}, true)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	err = state.PostUpgradeHook(s, req.InitConfig)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func clusterPost(s *state.State, r *http.Request) response.Response {
	req := internalTypes.ClusterMember{}

	upgrading := r.URL.Query().Get("upgrade") == "1"

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	// If we received a forwarded request, assume the new member was successfully added on the leader,
	// and execute the new member hook.
	if client.IsForwardedRequest(r) {
		if upgrading {
			err := state.OnUpgradedMemberHook(s)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to run post cluster member upgrade actions: %w", err))
			}
		} else {
			err := state.OnNewMemberHook(s)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to run post cluster member add actions: %w", err))
			}
		}

		return response.EmptySyncResponse
	}

	ctx, cancel := context.WithTimeout(s.Context, time.Second*30)
	defer cancel()

	leaderClient, err := s.Database.Leader(ctx)
	if err != nil {
		return response.SmartError(err)
	}

	leaderInfo, err := leaderClient.Leader(ctx)
	if err != nil {
		return response.SmartError(err)
	}

	// Check if any of the remote's addresses are currently in use.
	for _, role := range []trust.Role{trust.Cluster, trust.NonCluster} {
		existingRemote := s.Remotes(role).RemoteByAddress(req.Address)
		if existingRemote != nil {
			return response.SmartError(fmt.Errorf("Remote with address %q exists", req.Address.String()))
		}
	}

	// Forward request to leader.
	if leaderInfo.Address != s.Address().URL.Host {
		client, err := s.Leader()
		if err != nil {
			return response.SmartError(err)
		}

		tokenResponse, err := client.AddClusterMember(s.Context, req, false)
		if err != nil {
			return response.SmartError(err)
		}

		return response.SyncResponse(true, tokenResponse)
	}

	clusterRole := trust.Cluster
	err = s.Database.Transaction(s.Context, func(ctx context.Context, tx *sql.Tx) error {
		dbClusterMember := cluster.InternalClusterMember{
			Name:        req.Name,
			Address:     req.Address.String(),
			Certificate: req.Certificate.String(),
			Schema:      req.SchemaVersion,
			Heartbeat:   time.Time{},
			Role:        cluster.Pending,
		}

		record, err := cluster.GetInternalTokenRecord(ctx, tx, req.Secret)
		if err != nil {
			return err
		}

		clusterRole = trust.Role(record.Role)
		if clusterRole == trust.Cluster {
			_, err = cluster.CreateInternalClusterMember(ctx, tx, dbClusterMember)
			if err != nil {
				return err
			}
		} else {
			_, err = cluster.CreateInternalNonClusterMember(ctx, tx, cluster.InternalNonClusterMember{
				Name:        dbClusterMember.Name,
				Address:     dbClusterMember.Address,
				Certificate: dbClusterMember.Certificate,
			})
			if err != nil {
				return err
			}
		}

		return cluster.DeleteInternalTokenRecord(ctx, tx, record.Name)
	})
	if err != nil {
		return response.SmartError(err)
	}

	clusterMembers := map[trust.Role][]internalTypes.ClusterMemberLocal{}
	for _, role := range []trust.Role{trust.Cluster, trust.NonCluster} {
		remotes := s.Remotes(role)
		clusterMembers[role] = make([]internalTypes.ClusterMemberLocal, 0, remotes.Count())
		for _, clusterMember := range remotes.RemotesByName() {
			clusterMember := internalTypes.ClusterMemberLocal{
				Name:        clusterMember.Name,
				Address:     clusterMember.Address,
				Certificate: clusterMember.Certificate,
			}

			clusterMembers[role] = append(clusterMembers[role], clusterMember)
		}
	}

	clusterCert, err := s.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	tokenResponse := internalTypes.TokenResponse{
		ClusterCert: types.X509Certificate{Certificate: clusterCert},
		ClusterKey:  string(s.ClusterCert().PrivateKey()),

		ClusterMembers:    clusterMembers[trust.Cluster],
		NonClusterMembers: clusterMembers[trust.NonCluster],
	}

	newRemote := trust.Remote{
		Location:    trust.Location{Name: req.Name, Address: req.Address, Role: clusterRole},
		Certificate: req.Certificate,
	}

	// Add the cluster member to our local store for authentication.
	err = s.Remotes(clusterRole).Add(s.OS.TrustDir, newRemote)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, tokenResponse)
}

func clusterGet(s *state.State, r *http.Request) response.Response {
	// Default to the cluster role if no role is specified.
	role := trust.Role(r.URL.Query().Get("role"))
	if role == "" {
		role = trust.Cluster
	}

	var apiNonClusterMembers []internalTypes.ClusterMemberLocal
	var apiClusterMembers []internalTypes.ClusterMember
	err := s.Database.Transaction(s.Context, func(ctx context.Context, tx *sql.Tx) error {
		if role == trust.NonCluster {
			clusterMembers, err := cluster.GetInternalNonClusterMembers(ctx, tx)
			if err != nil {
				return err
			}

			apiNonClusterMembers = make([]internalTypes.ClusterMemberLocal, 0, len(clusterMembers))
			for _, clusterMember := range clusterMembers {
				apiClusterMember, err := clusterMember.ToAPI()
				if err != nil {
					return err
				}

				apiNonClusterMembers = append(apiNonClusterMembers, *apiClusterMember)
			}

			return nil
		}

		clusterMembers, err := cluster.GetInternalClusterMembers(ctx, tx)
		if err != nil {
			return err
		}

		apiClusterMembers = make([]internalTypes.ClusterMember, 0, len(clusterMembers))
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

	if role == trust.NonCluster {
		return response.SyncResponse(true, apiNonClusterMembers)
	}

	clusterCert, err := s.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	// Send a small request to each node to ensure they are reachable.
	for i, clusterMember := range apiClusterMembers {
		addr := api.NewURL().Scheme("https").Host(clusterMember.Address.String())
		d, err := internalClient.New(*addr, s.ServerCert(), clusterCert, false)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to create HTTPS client for cluster member with address %q: %w", addr.String(), err))
		}

		err = d.CheckReady(s.Context)
		if err == nil {
			apiClusterMembers[i].Status = internalTypes.MemberOnline
		} else {
			logger.Warnf("Failed to get status of cluster member with address %q: %v", addr.String(), err)
		}
	}

	return response.SyncResponse(true, apiClusterMembers)
}

func clusterPut(s *state.State, r *http.Request) response.Response {
	req := internalTypes.ClusterMember{}

	upgrading := r.URL.Query().Get("upgrade") == "1"

	// Default to the cluster role if no role is specified.
	role := trust.Role(r.URL.Query().Get("role"))
	if role == "" {
		role = trust.Cluster
	}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	newRemote := trust.Remote{
		Location:    trust.Location{Name: req.Name, Address: req.Address, Role: role},
		Certificate: req.Certificate,
	}

	ctx, cancel := context.WithTimeout(s.Context, 30*time.Second)
	defer cancel()

	if !client.IsForwardedRequest(r) {
		cluster, err := s.Cluster(r, trust.Cluster)
		if err != nil {
			return response.SmartError(err)
		}

		err = cluster.Query(ctx, true, func(ctx context.Context, c *client.Client) error {
			// No need to send a request to ourselves, or to the node we are adding.
			if s.Address().URL.Host == c.URL().URL.Host || req.Address.String() == c.URL().URL.Host {
				return nil
			}

			return c.RegisterClusterMember(ctx, req, string(role), false)
		})

		// Also add the remote to the other non-cluster members.
		cluster, err = s.Cluster(r, trust.NonCluster)
		if err != nil {
			return response.SmartError(err)
		}

		err = cluster.Query(ctx, true, func(ctx context.Context, c *client.Client) error {
			// No need to send a request to ourselves, or to the node we are adding.
			if s.Address().URL.Host == c.URL().URL.Host || req.Address.String() == c.URL().URL.Host {
				return nil
			}

			return c.RegisterClusterMember(ctx, req, string(role), false)
		})
	}

	if !upgrading {
		// At this point, the node has joined dqlite so we can add a local record for it if we haven't already from a heartbeat (or if we are the leader).
		remotes := s.Remotes(role)
		_, ok := remotes.RemotesByName()[newRemote.Name]
		if !ok {
			err = remotes.Add(s.OS.TrustDir, newRemote)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed adding local record of newly joined node %q: %w", req.Name, err))
			}
		}
	} else {
		nonClusterRemotes := s.Remotes(trust.NonCluster)
		nonClusterRemotesMap := nonClusterRemotes.RemotesByName()

		newRemote := nonClusterRemotesMap[req.Name]
		newRemote.Role = trust.Cluster
		delete(nonClusterRemotesMap, req.Name)

		nonClusterList := make([]internalTypes.ClusterMember, 0, len(nonClusterRemotesMap))

		for _, remote := range nonClusterRemotesMap {
			nonClusterList = append(nonClusterList, internalTypes.ClusterMember{
				ClusterMemberLocal: internalTypes.ClusterMemberLocal{
					Name:        remote.Name,
					Address:     remote.Address,
					Certificate: remote.Certificate,
				}})
		}

		err = s.Remotes(trust.NonCluster).Replace(s.OS.TrustDir, nonClusterList...)
		if err != nil {
			return response.SmartError(err)
		}

		err = s.Remotes(trust.Cluster).Add(s.OS.TrustDir, newRemote)
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.EmptySyncResponse
}

// clusterDisableMu is used to prevent the daemon process from being replaced/stopped during removal from the
// cluster until such time as the request that initiated the removal has finished. This allows for self removal
// from the cluster when not the leader.
var clusterDisableMu sync.Mutex

// Re-execs the daemon of the cluster member with a fresh s.
func clusterMemberPut(s *state.State, r *http.Request) response.Response {
	force := r.URL.Query().Get("force") == "1"

	// If we received a cluster notification, run the pre-removal hook and return.
	if client.IsForwardedRequest(r) {
		err := state.PreRemoveHook(s, force)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to run pre-removal hook: %w", err))
		}

		return response.EmptySyncResponse
	}

	err := s.Database.Stop()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed shutting down database: %w", err))
	}

	err = state.StopListeners()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed shutting down listeners: %w", err))
	}

	err = os.RemoveAll(s.OS.StateDir)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to remove the s directory: %w", err))
	}

	go func() {
		<-r.Context().Done() // Wait until request has finished.

		// Wait until we can acquire the lock. This way if another request is holding the lock we won't
		// replace/stop the LXD daemon until that request has finished.
		clusterDisableMu.Lock()
		defer clusterDisableMu.Unlock()
		execPath, err := os.Readlink("/proc/self/exe")
		if err != nil {
			execPath = "bad-exec-path"
		}

		// The execPath from /proc/self/exe can end with " (deleted)" if the lxd binary has been removed/changed
		// since the lxd process was started, strip this so that we only return a valid path.
		logger.Info("Restarting daemon following removal from cluster")
		execPath = strings.TrimSuffix(execPath, " (deleted)")
		err = unix.Exec(execPath, os.Args, os.Environ())
		if err != nil {
			logger.Error("Failed restarting daemon", logger.Ctx{"err": err})
		}
	}()

	return response.ManualResponse(func(w http.ResponseWriter) error {
		err := response.EmptySyncResponse.Render(w)
		if err != nil {
			return err
		}

		// Send the response before replacing the LXD daemon process.
		f, ok := w.(http.Flusher)
		if ok {
			f.Flush()
		} else {
			return fmt.Errorf("http.ResponseWriter is not type http.Flusher")
		}

		return nil
	})
}

// clusterMemberDelete Removes a cluster member from dqlite and re-execs its daemon.
func clusterMemberDelete(s *state.State, r *http.Request) response.Response {
	force := r.URL.Query().Get("force") == "1"
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	var allRemotes map[string]trust.Remote
	var role trust.Role
	for _, clusterRole := range []trust.Role{trust.Cluster, trust.NonCluster} {
		role = clusterRole
		allRemotes = s.Remotes(role).RemotesByName()
		_, ok := allRemotes[name]
		if ok {
			break
		}
	}

	// If we received a forwarded request, assume the new member was successfully removed on the leader,
	// and execute the post-remove hook.
	if client.IsForwardedRequest(r) {
		newRemotes := []internalTypes.ClusterMember{}
		for _, remote := range allRemotes {
			if remote.Name != name {
				clusterMember := internalTypes.ClusterMemberLocal{Name: remote.Name, Address: remote.Address, Certificate: remote.Certificate}
				newRemotes = append(newRemotes, internalTypes.ClusterMember{ClusterMemberLocal: clusterMember})
			}
		}

		// Remove the cluster member from the leader's trust store.
		err = s.Remotes(role).Replace(s.OS.TrustDir, newRemotes...)
		if err != nil {
			return response.SmartError(err)
		}

		if s.Role() == trust.Cluster {
			err := state.PostRemoveHook(s, force)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to run post cluster member remove actions: %w", err))
			}
		}

		return response.EmptySyncResponse
	}

	if allRemotes == nil {
		return response.SmartError(fmt.Errorf("No remote exists with the given name %q", name))
	}

	remote := allRemotes[name]

	ctx, cancel := context.WithTimeout(s.Context, time.Second*30)
	defer cancel()

	publicKey, err := s.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	// Forward the request to a cluster member if we aren't one.
	if s.Role() == trust.NonCluster {
		randomRemote := s.Remotes(trust.Cluster).SelectRandom()
		c, err := internalClient.New(randomRemote.URL(), s.ServerCert(), publicKey, false)
		if err != nil {
			return response.SmartError(err)
		}

		err = c.DeleteClusterMember(ctx, name, force)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}

	leader, err := s.Database.Leader(ctx)
	if err != nil {
		return response.SmartError(err)
	}

	leaderInfo, err := leader.Leader(ctx)
	if err != nil {
		return response.SmartError(err)
	}

	// If we are not the leader, just update our trust store and forward the request.
	if leaderInfo.Address != s.Address().URL.Host {
		if allRemotes[name].Address.String() == s.Address().URL.Host {
			// If the member being removed is ourselves and we are not the leader, then lock the
			// clusterPutDisableMu before we forward the request to the leader, so that when the leader
			// goes on to request clusterPutDisable back to ourselves it won't be actioned until we
			// have returned this request back to the original client.
			clusterDisableMu.Lock()
			logger.Info("Acquired cluster self removal lock", logger.Ctx{"member": name})

			go func() {
				<-r.Context().Done() // Wait until request is finished.

				logger.Info("Releasing cluster self removal lock", logger.Ctx{"member": name})
				clusterDisableMu.Unlock()
			}()
		}

		client, err := s.Leader()
		if err != nil {
			return response.SmartError(err)
		}

		err = client.DeleteClusterMember(s.Context, name, force)
		if err != nil {
			return response.SmartError(err)
		}

		newRemotes := []internalTypes.ClusterMember{}
		for _, remote := range allRemotes {
			if remote.Name != name {
				clusterMember := internalTypes.ClusterMemberLocal{Name: remote.Name, Address: remote.Address, Certificate: remote.Certificate}
				newRemotes = append(newRemotes, internalTypes.ClusterMember{ClusterMemberLocal: clusterMember})
			}
		}

		err = s.Remotes(role).Replace(s.OS.TrustDir, newRemotes...)
		if err != nil {
			return response.SmartError(err)
		}

		return response.ManualResponse(func(w http.ResponseWriter) error {
			err := response.EmptySyncResponse.Render(w)
			if err != nil {
				return err
			}

			// Send the response before replacing the LXD daemon process.
			f, ok := w.(http.Flusher)
			if ok {
				f.Flush()
			} else {
				return fmt.Errorf("http.ResponseWriter is not type http.Flusher")
			}

			return nil
		})
	}

	// Handle role turnover for cluster members.
	index := -1
	if role == trust.Cluster {
		info, err := leader.Cluster(s.Context)
		if err != nil {
			return response.SmartError(err)
		}

		for i, node := range info {
			if node.Address == remote.Address.String() {
				index = i
				break
			}
		}

		if index < 0 {
			return response.SmartError(fmt.Errorf("No dqlite cluster member exists with the given name %q", name))
		}

		localClient, err := internalClient.New(s.OS.ControlSocket(), nil, nil, false)
		if err != nil {
			return response.SmartError(err)
		}

		clusterMembers, err := localClient.GetClusterMembers(s.Context)
		if err != nil {
			return response.SmartError(err)
		}

		numPending := 0
		for _, clusterMember := range clusterMembers {
			if clusterMember.Role == string(cluster.Pending) {
				numPending++
			}
		}

		if len(clusterMembers)-numPending < 2 {
			return response.SmartError(fmt.Errorf("Cannot remove cluster members, there are no remaining non-pending members"))
		}

		if len(info) < 2 {
			return response.SmartError(fmt.Errorf("Cannot leave a cluster with %d members", len(info)))
		}

		// If we are removing the leader of a 2-node cluster, ensure the remaining node is a voter.
		if len(info) == 2 && allRemotes[name].Address.String() == leaderInfo.Address {
			for _, node := range info {
				if node.Address != leaderInfo.Address && node.Role != dqliteClient.Voter {
					err = leader.Assign(ctx, node.ID, dqliteClient.Voter)
					if err != nil {
						return response.SmartError(err)
					}
				}
			}
		}

		// If we are the leader and removing ourselves, reassign the leader role and perform the removal from there.
		if allRemotes[name].Address.String() == leaderInfo.Address {
			otherNodes := []uint64{}
			for _, node := range info {
				if node.Address != allRemotes[name].Address.String() && node.Role == dqliteClient.Voter {
					otherNodes = append(otherNodes, node.ID)
				}
			}

			if len(otherNodes) == 0 {
				return response.SmartError(fmt.Errorf("Found no voters to transfer leadership to"))
			}

			randomID := otherNodes[rand.Intn(len(otherNodes))]
			err = leader.Transfer(ctx, randomID)
			if err != nil {
				return response.SmartError(err)
			}

			client, err := s.Leader()
			if err != nil {
				return response.SmartError(err)
			}

			clusterDisableMu.Lock()
			logger.Info("Acquired cluster self removal lock", logger.Ctx{"member": name})

			go func() {
				<-r.Context().Done() // Wait until request is finished.

				logger.Info("Releasing cluster self removal lock", logger.Ctx{"member": name})
				clusterDisableMu.Unlock()
			}()

			err = client.DeleteClusterMember(s.Context, name, force)
			if err != nil {
				return response.SmartError(err)
			}

			return response.ManualResponse(func(w http.ResponseWriter) error {
				err := response.EmptySyncResponse.Render(w)
				if err != nil {
					return err
				}

				// Send the response before replacing the LXD daemon process.
				f, ok := w.(http.Flusher)
				if ok {
					f.Flush()
				} else {
					return fmt.Errorf("http.ResponseWriter is not type http.Flusher")
				}

				return nil
			})
		}
	}

	// Remove the cluster member from the database.
	err = s.Database.Transaction(s.Context, func(ctx context.Context, tx *sql.Tx) error {
		if role == trust.NonCluster {
			return cluster.DeleteInternalNonClusterMember(ctx, tx, remote.Address.String())
		} else {
			return cluster.DeleteInternalClusterMember(ctx, tx, remote.Address.String())
		}
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Set the forwarded flag so that the the system to be removed knows the removal is in progress.
	c, err := internalClient.New(remote.URL(), s.ServerCert(), publicKey, true)
	if err != nil {
		return response.SmartError(err)
	}

	// Tell the cluster member to run its PreRemove hook and return.
	err = c.ResetClusterMember(s.Context, name, force)
	if err != nil {
		return response.SmartError(err)
	}

	if role == trust.Cluster {
		// Remove the node from dqlite.
		info, err := leader.Cluster(s.Context)
		if err != nil {
			return response.SmartError(err)
		}

		err = leader.Remove(s.Context, info[index].ID)
		if err != nil {
			return response.SmartError(err)
		}
	}

	newRemotes := []internalTypes.ClusterMember{}
	for _, remote := range allRemotes {
		if remote.Name != name {
			clusterMember := internalTypes.ClusterMemberLocal{Name: remote.Name, Address: remote.Address, Certificate: remote.Certificate}
			newRemotes = append(newRemotes, internalTypes.ClusterMember{ClusterMemberLocal: clusterMember})
		}
	}

	// Remove the cluster member from the leader's trust store.
	err = s.Remotes(role).Replace(s.OS.TrustDir, newRemotes...)
	if err != nil {
		return response.SmartError(err)
	}

	c, err = internalClient.New(remote.URL(), s.ServerCert(), publicKey, false)
	if err != nil {
		return response.SmartError(err)
	}

	// Tell the node to reset itself.
	err = c.ResetClusterMember(s.Context, name, force)
	if err != nil {
		return response.SmartError(err)
	}

	clusterClients, err := s.Cluster(r, trust.Cluster)
	if err != nil {
		return response.SmartError(err)
	}

	nonClusterclients, err := s.Cluster(r, trust.NonCluster)
	if err != nil {
		return response.SmartError(err)
	}

	err = state.PostRemoveHook(s, force)
	if err != nil {
		return response.SmartError(err)
	}

	for _, c := range []client.Cluster{clusterClients, nonClusterclients} {
		// Tell all the other cluster members to run their PostRemove hooks.
		err = c.Query(s.Context, true, func(ctx context.Context, c *client.Client) error {
			return c.DeleteClusterMember(ctx, name, force)
		})
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.EmptySyncResponse
}
