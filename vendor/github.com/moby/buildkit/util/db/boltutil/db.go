package boltutil

import (
	"io/fs"

	"github.com/moby/buildkit/util/db"
	bolt "go.etcd.io/bbolt"
)

func Open(p string, mode fs.FileMode, options *bolt.Options) (db.DB, error) {
	bdb, err := bolt.Open(p, mode, options)
	if err != nil {
		return nil, err
	}
	return bdb, nil
}
