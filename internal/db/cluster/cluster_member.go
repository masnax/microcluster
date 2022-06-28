package cluster

import (
	"fmt"
	"time"

	"github.com/canonical/microcluster/internal/rest/types"
)

//go:generate -command mapper lxd-generate db mapper -t cluster_members.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e cluster_member objects table=cluster_members version=2
//go:generate mapper stmt -e cluster_member objects-by-Address table=cluster_members version=2
//go:generate mapper stmt -e cluster_member id table=cluster_members version=2
//go:generate mapper stmt -e cluster_member create table=cluster_members version=2
//go:generate mapper stmt -e cluster_member delete-by-Address table=cluster_members version=2
//go:generate mapper stmt -e cluster_member update table=cluster_members version=2
//
//go:generate mapper method -i -e cluster_member GetMany version=2
//go:generate mapper method -i -e cluster_member GetOne version=2
//go:generate mapper method -i -e cluster_member ID version=2
//go:generate mapper method -i -e cluster_member Exists version=2
//go:generate mapper method -i -e cluster_member Create version=2
//go:generate mapper method -i -e cluster_member DeleteOne-by-Address version=2
//go:generate mapper method -i -e cluster_member Update version=2

// ClusterMember represents the global database entry for a dqlite cluster member.
type ClusterMember struct {
	ID          int
	Name        string
	Address     string `db:"primary=yes"`
	Certificate string
	Schema      int
	Heartbeat   time.Time
	Role        Role
}

// ClusterMemberFilter is used for filtering queries using generated methods.
type ClusterMemberFilter struct {
	Address *string
}

// ToAPI returns the api struct for a ClusterMember database entity.
// The cluster member's status will be reported as unreachable by default.
func (c ClusterMember) ToAPI() (*types.ClusterMember, error) {
	address, err := types.ParseAddrPort(c.Address)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse address %q of database cluster member: %w", c.Address, err)
	}

	certificate, err := types.ParseX509Certificate(c.Certificate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse certificate of database cluster member with address %q: %w", c.Address, err)
	}

	return &types.ClusterMember{
		ClusterMemberLocal: types.ClusterMemberLocal{
			Name:        c.Name,
			Address:     address,
			Certificate: *certificate,
		},
		Role:          c.Role,
		SchemaVersion: c.Schema,
		LastHeartbeat: c.Heartbeat,
		Status:        types.MemberUnreachable,
	}, nil
}
