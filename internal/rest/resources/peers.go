package resources

import (
	"fmt"
	"net/http"

	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/logger"
	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
)

var peersCmd = rest.Endpoint{
	Path: "peers",

	Get: rest.EndpointAction{Handler: peersGet, AllowUntrusted: true},
}

func peersGet(state *state.State, r *http.Request) response.Response {
	members, err := state.Database.Cluster()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get peers: %w", err))
	}

	apiPeers := make([]types.Peer, len(members))
	for i, member := range members {
		// Create an entry for any nodes without truststore yaml files.
		addrPort, err := types.ParseAddrPort(member.Address)
		if err != nil {
			return response.SmartError(err)
		}

		currentPeer := state.Remotes().RemoteByAddress(addrPort)
		if currentPeer == nil {
			addrPort, err := types.ParseAddrPort(member.Address)
			if err != nil {
				return response.SmartError(err)
			}

			apiPeers[i] = types.Peer{
				Name:        "",
				Addresses:   types.AddrPorts{addrPort},
				Role:        member.Role.String(),
				Certificate: types.X509Certificate{},
				Status:      types.PeerNotTrusted,
			}

			continue
		}

		peer := types.Peer{
			Name:        currentPeer.Name,
			Addresses:   currentPeer.Addresses,
			Role:        member.Role.String(),
			Certificate: currentPeer.Certificate,
			Status:      types.PeerUnreachable,
		}

		if member.Address == state.Address.URL.Host {
			peer.Status = types.PeerOnline
			apiPeers[i] = peer

			continue
		}

		for _, addr := range currentPeer.URLs() {
			peerCert, err := state.ClusterCert().PublicKeyX509()
			if err != nil {
				return response.SmartError(err)
			}

			d, err := client.New(addr, state.ServerCert(), peerCert, false)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to create HTTPS client for peer with address %q: %w", addr.String(), err))
			}

			err = d.QueryStruct(state.Context, "GET", client.InternalEndpoint, api.NewURL().Path("ready"), nil, nil)
			if err == nil {
				peer.Status = types.PeerOnline
				break
			} else {
				logger.Warnf("Failed to get status of peer with address %q: %v", addr.String(), err)
			}
		}

		apiPeers[i] = peer
	}

	nodeAddrs := make([]string, len(members))
	for i, member := range members {
		nodeAddrs[i] = member.Address
	}

	// Create entries for nodes with truststore files that are not actually in dqlite.
	for _, peer := range state.Remotes() {
		found := false
		for _, addr := range peer.Addresses {
			if shared.StringInSlice(addr.String(), nodeAddrs) {
				found = true
				break
			}
		}

		if !found {
			missingPeer := types.Peer{
				Name:        peer.Name,
				Addresses:   peer.Addresses,
				Certificate: peer.Certificate,
				Status:      types.PeerNotFound,
			}

			apiPeers = append(apiPeers, missingPeer)
		}
	}

	return response.SyncResponse(true, apiPeers)
}
