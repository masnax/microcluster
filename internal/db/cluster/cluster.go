package cluster

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/lxc/lxd/lxd/db/query"
)

//go:generate -command mapper lxd-generate db mapper -t cluster_members.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e cluster_member objects version=2
//go:generate mapper stmt -e cluster_member objects-by-Address version=2
//go:generate mapper stmt -e cluster_member id version=2
//go:generate mapper stmt -e cluster_member create version=2
//go:generate mapper stmt -e cluster_member delete-by-Address version=2
//go:generate mapper stmt -e cluster_member update version=2
//
//go:generate mapper method -i -e cluster_member GetMany version=2
//go:generate mapper method -i -e cluster_member GetOne version=2
//go:generate mapper method -i -e cluster_member ID version=2
//go:generate mapper method -i -e cluster_member Exists version=2
//go:generate mapper method -i -e cluster_member Create version=2
//go:generate mapper method -i -e cluster_member DeleteOne-by-Address version=2
//go:generate mapper method -i -e cluster_member Update version=2

type ClusterMember struct {
	ID          int
	Name        string
	Address     string `db:"primary=yes"`
	Certificate string
	Schema      int
	Heartbeat   time.Time
	Role        string
}

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

// UpdateClusterMemberSchemaVersion sets the schema version for the cluster member with the given address.
// This helper is non-generated so as to be accessible without prepared statements, as our schema might have changed.
func UpdateClusterMemberSchemaVersion(tx *sql.Tx, version int, address string) error {
	stmt := "UPDATE cluster_members SET schema=? WHERE address=?"
	result, err := tx.Exec(stmt, version, address)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("updated %d rows instead of 1", n)
	}

	return nil
}

func GetClusterMemberSchemaVersions(tx *sql.Tx) ([]int, error) {
	sql := "SELECT schema FROM cluster_members WHERE NOT role='pending'"
	versions, err := query.SelectIntegers(tx, sql)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
