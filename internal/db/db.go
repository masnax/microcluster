package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lxc/lxd/lxd/db/query"
	"github.com/lxc/lxd/lxd/db/schema"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/internal/db/cluster"
	"github.com/canonical/microcluster/internal/db/update"
	"github.com/canonical/microcluster/internal/logger"
)

// Tx is a convenience so we don't have to import sql.Tx everywhere.
type Tx = sql.Tx

// Open opens the dqlite database and loads the schema.
func (db *DB) Open(bootstrap bool, address api.URL) (bool, error) {
	ctx, cancel := context.WithTimeout(db.ctx, 10*time.Second)
	defer cancel()

	err := db.dqlite.Ready(ctx)
	if err != nil {
		return false, err
	}

	db.db, err = db.dqlite.Open(db.ctx, db.dbName)
	if err != nil {
		return false, err
	}

	otherNodesBehind := false
	newSchema := update.Schema()
	if !bootstrap {
		checkVersions := func(current int, tx *sql.Tx) error {
			schemaVersion := newSchema.Version()
			err = cluster.UpdateClusterMemberSchemaVersion(tx, schemaVersion, address.URL.Host)
			if err != nil {
				return err
			}

			versions, err := cluster.GetClusterMemberSchemaVersions(tx)
			if err != nil {
				return err
			}

			for _, version := range versions {
				if schemaVersion == version {
					// Versions are equal, there's hope for the
					// update. Let's check the next node.
					continue
				}

				if schemaVersion > version {
					// Our version is bigger, we should stop here
					// and wait for other nodes to be upgraded and
					// restarted.
					otherNodesBehind = true
					return schema.ErrGracefulAbort
				}

				// Another node has a version greater than ours
				// and presumeably is waiting for other nodes
				// to upgrade. Let's error out and shutdown
				// since we need a greater version.
				return fmt.Errorf("this node's version is behind, please upgrade")
			}
			return nil
		}

		newSchema.Check(checkVersions)
	}

	db.retry(func() error {
		_, err = newSchema.Ensure(db.db)
		return err
	})
	if otherNodesBehind {
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("Failed to bootstrap schema: %w", err)
	}

	err = cluster.PrepareStmts(db.db, false)
	if err != nil {
		return false, err
	}

	db.openCanceller.Cancel()

	return false, nil
}

// Transaction handles performing a transaction on the dqlite database.
func (db *DB) Transaction(ctx context.Context, f func(context.Context, *Tx) error) error {
	return db.retry(func() error {
		err := query.Transaction(ctx, db.db, f)
		if errors.Is(err, context.DeadlineExceeded) {
			// If the query timed out it likely means that the leader has abruptly become unreachable.
			// Now that this query has been cancelled, a leader election should have taken place by now.
			// So let's retry the transaction once more in case the global database is now available again.
			logger.Warn("Transaction timed out. Retrying once", logger.Ctx{"err": err})
			return query.Transaction(ctx, db.db, f)
		}

		return err
	})
}

func (db *DB) retry(f func() error) error {
	if db.ctx.Err() != nil {
		return f()
	}

	return query.Retry(f)
}
