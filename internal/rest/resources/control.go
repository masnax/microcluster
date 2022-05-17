package resources

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lxc/lxd/lxd/response"

	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/access"
	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/canonical/microcluster/internal/state"
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
		return response.SmartError(fmt.Errorf("Invalid options"))
	}

	if !req.Bootstrap && req.JoinAddress == "" {
		return response.SmartError(fmt.Errorf("Invalid options - expected to bootstrap or be given a join address"))
	}

	if len(state.Remotes()) == 0 {
		return response.BadRequest(fmt.Errorf("Cannot initialise - truststore must contain peers"))
	}

	err = state.StartAPI(req.Bootstrap, req.JoinAddress)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
