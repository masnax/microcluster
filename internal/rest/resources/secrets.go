package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared"

	"github.com/canonical/microcluster/internal/db"
	"github.com/canonical/microcluster/internal/db/cluster"
	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
)

var secretsCmd = rest.Endpoint{
	Path: "secrets",

	Post: rest.EndpointAction{Handler: secretsPost, AccessHandler: access.AllowAuthenticated},
	Get:  rest.EndpointAction{Handler: secretsGet, AccessHandler: access.AllowAuthenticated},
}

var secretCmd = rest.Endpoint{
	Path: "secrets/{joinerCert}",

	Post:   rest.EndpointAction{Handler: secretPost, AllowUntrusted: true},
	Delete: rest.EndpointAction{Handler: secretDelete, AccessHandler: access.AllowAuthenticated},
}

func secretsPost(state *state.State, r *http.Request) response.Response {
	req := types.Secret{}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	// Generate join secret for new member. This will be stored alongside the join
	// address and cluster certificate to simplify setup.
	tokenKey, err := shared.RandomCryptoString()
	if err != nil {
		return response.InternalError(err)
	}

	clusterCert, err := state.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.InternalError(err)
	}

	joinAddress, err := types.ParseAddrPort(state.Address.URL.Host)
	if err != nil {
		return response.InternalError(err)
	}

	token := types.Token{
		Token:       tokenKey,
		ClusterCert: types.X509Certificate{Certificate: clusterCert},
		JoinAddress: joinAddress,
	}

	tokenString, err := token.String()
	if err != nil {
		return response.InternalError(err)
	}

	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		exists, err := cluster.SecretExists(ctx, tx, req.JoinerCert)
		if err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("A join token already exists for the name %q", req.JoinerCert)
		}

		_, err = cluster.CreateSecret(ctx, tx, cluster.Secret{JoinerCert: req.JoinerCert, Token: tokenKey})
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, tokenString)
}

func secretsGet(state *state.State, r *http.Request) response.Response {
	var secrets []cluster.Secret
	err := state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		var err error
		secrets, err = cluster.GetSecrets(ctx, tx, cluster.SecretFilter{})

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, secrets)
}

func secretPost(state *state.State, r *http.Request) response.Response {
	joinerCert, err := url.PathUnescape(mux.Vars(r)["joinerCert"])
	if err != nil {
		return response.SmartError(err)
	}

	// Parse the request.
	req := types.Secret{}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	var secret *cluster.Secret
	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		var err error
		secret, err = cluster.GetSecret(ctx, tx, joinerCert)
		if err != nil {
			return err
		}

		if secret.Token != req.Token {
			return fmt.Errorf("Received invalid token for the given joiner certificate")
		}

		return cluster.DeleteSecret(ctx, tx, joinerCert)
	})
	if err != nil {
		return response.SmartError(err)
	}

	clusterCert, err := state.ClusterCert().PublicKeyX509()
	if err != nil {
		return response.SmartError(err)
	}

	remotes := state.Remotes()
	peers := make([]types.Peer, 0, len(remotes))
	for _, peer := range remotes {
		peer := types.Peer{
			Name:        peer.Name,
			Addresses:   peer.Addresses,
			Certificate: peer.Certificate,
		}

		peers = append(peers, peer)
	}

	secretResponse := types.SecretResponse{
		ClusterCert: types.X509Certificate{Certificate: clusterCert},
		ClusterKey:  string(state.ClusterCert().PrivateKey()),

		Peers: peers,
	}

	return response.SyncResponse(true, secretResponse)
}

func secretDelete(state *state.State, r *http.Request) response.Response {
	joinerCert, err := url.PathUnescape(mux.Vars(r)["joinerCert"])
	if err != nil {
		return response.SmartError(err)
	}

	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		return cluster.DeleteSecret(ctx, tx, joinerCert)
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
