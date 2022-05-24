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

	if req.Bootstrap && req.JoinAddress != "" {
		return response.SmartError(fmt.Errorf("Invalid options - received join address and bootstrap flag"))
	}

	if req.Bootstrap && req.JoinToken != "" {
		return response.SmartError(fmt.Errorf("Invalid options - received join token and bootstrap flag"))
	}

	if req.JoinToken != "" && req.JoinAddress != "" {
		return response.SmartError(fmt.Errorf("Invalid options - received join token without join address"))
	}

	if req.JoinToken != "" {
		return joinWithToken(state, req)
	}

	if !req.Bootstrap && req.JoinAddress == "" {
		return response.SmartError(fmt.Errorf("Invalid options - expected to bootstrap or be given a join address"))
	}

	err = state.StartAPI(req.Bootstrap, req.JoinAddress)
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

	url := api.NewURL().Scheme("https").Host(token.JoinAddress.String())
	d, err := client.New(*url, state.ServerCert(), token.ClusterCert.Certificate)
	if err != nil {
		return response.SmartError(err)
	}

	secret, err := d.SubmitToken(context.Background(), state.ServerCert().Fingerprint(), token.Token)
	if err != nil {
		return response.SmartError(err)
	}

	err = util.WriteCert(state.OS.StateDir, "cluster", []byte(secret.ClusterCert.String()), []byte(secret.ClusterKey), nil)
	if err != nil {
		return response.SmartError(err)
	}

	peers := make([]trust.Remote, 0, len(secret.Peers)+1)
	for _, peer := range secret.Peers {
		remote := trust.Remote{
			Name:        peer.Name,
			Certificate: peer.Certificate,
			Addresses:   peer.Addresses,
		}

		peers = append(peers, remote)
	}

	addr, err := types.ParseAddrPort(state.Address.URL.Host)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to parse listen address when bootstrapping API: %w", err))
	}

	serverCert, err := state.ServerCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to parse server certificate when bootstrapping API: %w", err))
	}

	localPeer := trust.Remote{
		Name:        filepath.Base(state.OS.StateDir),
		Addresses:   types.AddrPorts{addr},
		Certificate: types.X509Certificate{Certificate: serverCert},
	}

	peers = append(peers, localPeer)
	err = state.Remotes().Add(state.OS.TrustDir, peers...)
	if err != nil {
		return response.SmartError(err)
	}

	err = state.StartAPI(false, state.Remotes().SelectRandom().Addresses.Strings()...)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
