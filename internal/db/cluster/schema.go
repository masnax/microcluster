package cluster

import (
	"time"
)

//go:generate -command mapper lxd-generate db mapper -t schema.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e schema objects version=2
//go:generate mapper stmt -e schema id version=2
//go:generate mapper stmt -e schema create version=2
//
//go:generate mapper method -e schema GetMany version=2
//go:generate mapper method -e schema GetOne version=2
//go:generate mapper method -e schema ID version=2
//go:generate mapper method -e schema Exists version=2
//go:generate mapper method -e schema Create version=2

// Schema represents the database schema table.
type Schema struct {
	ID        int
	Version   int `db:"primary=yes"`
	UpdatedAt time.Time
}

// SchemaFilter represents the database schema table.
type SchemaFilter struct {
	ID        *int
	Version   *int
	UpdatedAt *time.Time
}
