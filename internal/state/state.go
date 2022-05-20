package state

import (
	"context"

	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/db"
	"github.com/canonical/microcluster/internal/endpoints"
	"github.com/canonical/microcluster/internal/sys"
	"github.com/canonical/microcluster/internal/trust"
)

// State is a gateway to the stateful components of the microcluster daemon.
type State struct {
	// Context.
	Context context.Context

	// Ready channel.
	ReadyCh chan struct{}

	// File structure.
	OS *sys.OS

	// Listen Address.
	Address api.URL

	// Server.
	Endpoints *endpoints.Endpoints

	// Server certificate is used for server-to-server connection.
	// - Expected certificate in `peers`, `region`, `cell`, and `admin` Remotes.
	ServerCert func() *shared.CertInfo

	// Cluster certificate is used for downstream connections within a cluster.
	// - Used by all HTTPS listeners.
	// - Expected certificate in `cluster` and `migration` Remotes.
	ClusterCert func() *shared.CertInfo

	// Database.
	Database *db.DB

	// Remotes.
	Remotes func() trust.Remotes

	// Initialize APIs and bootstrap/join database.
	StartAPI func(bootstrap bool, joinAddresses ...string) error

	// When set, the consumer API will only allow GET requests.
	ReadOnly bool
}
