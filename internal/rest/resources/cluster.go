package resources

import (
	"net/http"

	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/state"
	"github.com/lxc/lxd/lxd/response"
)

var clustersCmd = rest.Endpoint{
	Path: "clusters",

	Post: rest.EndpointAction{Handler: clustersPost, AccessHandler: access.AllowAuthenticated},
	Get:  rest.EndpointAction{Handler: peersGet, AccessHandler: access.AllowAuthenticated},
}

func clustersPost(state *state.State, r *http.Request) response.Response {

	return response.EmptySyncResponse
}
