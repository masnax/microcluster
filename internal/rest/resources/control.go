package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
	"github.com/canonical/microcluster/internal/trust"
)

var controlCmd = rest.Endpoint{
	Post: rest.EndpointAction{Handler: controlPost, AccessHandler: access.AllowAuthenticated},
}

func controlPost(state *state.State, r *http.Request) response.Response {
	req := &types.Control{}
	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	if req.Bootstrap && req.JoinToken != "" {
		return response.SmartError(fmt.Errorf("Invalid options - received join token and bootstrap flag"))
	}

	if req.JoinToken != "" {
		return joinWithToken(state, req)
	}

	err = state.StartAPI(req.Bootstrap)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func joinWithToken(state *state.State, req *types.Control) response.Response {
	token, err := types.DecodeToken(req.JoinToken)
	if err != nil {
		return response.SmartError(err)
	}

	// Get a client to the target address.
	url := api.NewURL().Scheme("https").Host(token.JoinAddress.String())
	d, err := client.New(*url, state.ServerCert(), token.ClusterCert.Certificate, false)
	if err != nil {
		return response.SmartError(err)
	}

	// Submit the token string to obtain cluster credentials.
	secret, err := d.SubmitToken(context.Background(), state.ServerCert().Fingerprint(), token.Token)
	if err != nil {
		return response.SmartError(err)
	}

	err = util.WriteCert(state.OS.StateDir, "cluster", []byte(secret.ClusterCert.String()), []byte(secret.ClusterKey), nil)
	if err != nil {
		return response.SmartError(err)
	}

	joinAddrs := types.AddrPorts{}
	clusterMembers := make([]trust.Remote, 0, len(secret.ClusterMembers))
	for _, clusterMember := range secret.ClusterMembers {
		remote := trust.Remote{
			Name:        clusterMember.Name,
			Certificate: clusterMember.Certificate,
			Address:     clusterMember.Address,
		}

		joinAddrs = append(joinAddrs, clusterMember.Address)
		clusterMembers = append(clusterMembers, remote)
	}

	addr, err := types.ParseAddrPort(state.Address.URL.Host)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to parse listen address when bootstrapping API: %w", err))
	}

	serverCert, err := state.ServerCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to parse server certificate when bootstrapping API: %w", err))
	}

	// Add the local node to the list of clusterMembers.
	localClusterMember := trust.Remote{
		Name:        filepath.Base(state.OS.StateDir),
		Address:     addr,
		Certificate: types.X509Certificate{Certificate: serverCert},
	}

	clusterMembers = append(clusterMembers, localClusterMember)
	err = state.Remotes().Add(state.OS.TrustDir, clusterMembers...)
	if err != nil {
		return response.SmartError(err)
	}

	// Prepare the cluster for the incoming dqlite request by creating a database entry.
	newClusterMember := types.ClusterMember{
		ClusterMemberLocal: types.ClusterMemberLocal{
			Name:        localClusterMember.Name,
			Address:     localClusterMember.Address,
			Certificate: localClusterMember.Certificate,
		},
	}

	err = d.AddClusterMember(context.Background(), newClusterMember)
	if err != nil {
		return response.SmartError(err)
	}

	// Start the HTTPS listeners and join Dqlite.
	err = state.StartAPI(false, joinAddrs.Strings()...)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
