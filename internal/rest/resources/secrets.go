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
	Path: "secrets/{name}",

	Delete: rest.EndpointAction{Handler: secretDelete, AccessHandler: access.AllowAuthenticated},
}

func secretsPost(state *state.State, r *http.Request) response.Response {
	req := types.SecretsPost{}

	// Parse the request.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	// Generate join secret for new member. This will be stored inside the join token operation and will be
	// supplied by the joining member (encoded inside the join token) which will allow us to lookup the correct
	// operation in order to validate the requested joining server name is correct and authorised.
	token, err := shared.RandomCryptoString()
	if err != nil {
		return response.InternalError(err)
	}

	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		exists, err := cluster.SecretExists(ctx, tx, req.Name)
		if err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("A join token already exists for the name %q", req.Name)
		}

		_, err = cluster.CreateSecret(ctx, tx, cluster.Secret{Name: req.Name, Token: token, Certificate: state.ClusterCert().Fingerprint()})
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, token)
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

func secretDelete(state *state.State, r *http.Request) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	err = state.Database.Transaction(state.Context, func(ctx context.Context, tx *db.Tx) error {
		return cluster.DeleteSecret(ctx, tx, name)
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
