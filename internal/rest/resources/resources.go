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
		secretsCmd,
		secretCmd,
		clusterCmd,
		heartbeatCmd,
	},
}

// PublicEndpoints are the /public API endpoints available without authentication.
var PublicEndpoints = &Resources{
	Path: client.PublicEndpoint,
	Endpoints: []rest.Endpoint{
		clusterCmd,
		secretCmd,
	},
}

// InternalEndpoints are the /internal API endpoints available at the listen address.
var InternalEndpoints = &Resources{
	Path: client.InternalEndpoint,
	Endpoints: []rest.Endpoint{
		readyCmd,
		databaseCmd,
		secretsCmd,
		secretCmd,
		clusterCmd,
		heartbeatCmd,
	},
}
