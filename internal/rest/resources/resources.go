package resources

import (
	"github.com/canonical/microcluster/internal/rest"
	"github.com/canonical/microcluster/internal/rest/client"
)

// Resources represents all the resources served over the same path.
type Resources struct {
	Path      client.EndpointType
	Endpoints []rest.Endpoint
}

// ControlEndpoints are the endpoints available over the unix socket.
var ControlEndpoints = &Resources{
	Path: client.ControlEndpoint,
	Endpoints: []rest.Endpoint{
		controlCmd,
		sqlCmd,
		readyCmd,
		peersCmd,
	},
}

// InternalEndpoints are the /internal API endpoints available at the listen address.
var InternalEndpoints = &Resources{
	Path: client.InternalEndpoint,
	Endpoints: []rest.Endpoint{
		readyCmd,
		databaseCmd,
		peersCmd,
		secretsCmd,
		secretCmd,
	},
}
