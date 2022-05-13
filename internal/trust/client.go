package trust

import (
	"context"
	"fmt"

	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/rest/client"
)

// Peers returns a list of clients for every peer of the remote.
func (r Remotes) Peers(ctx context.Context, clientCert *shared.CertInfo) (client.Cluster, error) {
	remote := r.SelectRandom()
	if remote == nil {
		return nil, fmt.Errorf("No remote found")
	}

	remoteURL := remote.RandomURL()
	d, err := client.New(remoteURL, clientCert, remote.Certificate.Certificate)
	if err != nil {
		return nil, err
	}

	peers, err := d.Peers(ctx, client.InternalEndpoint)
	if err != nil {
		return nil, fmt.Errorf("Failed to get all peers of %q: %w", remoteURL.String(), err)
	}

	clients := make(client.Cluster, 0, len(peers))
	clients = append(clients, *d)
	for _, peer := range peers {
		if shared.StringInSlice(remoteURL.URL.Host, peer.Addresses.Strings()) {
			continue
		}

		peerURL := api.NewURL().Scheme("https").Host(peer.Addresses.SelectRandom().String())
		client, err := client.New(*peerURL, clientCert, remote.Certificate.Certificate)
		if err != nil {
			return nil, err
		}

		clients = append(clients, *client)
	}

	return clients, nil
}
