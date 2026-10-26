//go:build !windows

package boltutil

import bolt "go.etcd.io/bbolt"

func (d *DB) transactionDB() (*bolt.DB, error) {
	return d.bdb, nil
}
