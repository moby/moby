/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metadata

import bolt "go.etcd.io/bbolt"

type migration struct {
	schema  string
	version int
	migrate func(*bolt.Tx) error
}

// migrations stores the list of database migrations
// for each update to the database schema. The migrations
// array MUST be ordered by version from least to greatest.
// The last entry in the array should correspond to the
// schemaVersion and dbVersion constants.
// A migration test MUST be added for each migration in
// the array.
// The migrate function can safely assume the version
// of the data it is migrating from is the previous version
// of the database.
var migrations = []migration{
	{
		schema:  "v1",
		version: 1,
		migrate: addChildLinks,
	},
	{
		schema:  "v1",
		version: 2,
		migrate: migrateIngests,
	},
	{
		schema:  "v1",
		version: 3,
		migrate: noOpMigration,
	},
}

// addChildLinks Adds children key to the snapshotters to enforce snapshot
// entries cannot be removed which have children
func addChildLinks(tx *bolt.Tx) error {
	v1bkt := tx.Bucket(bucketKeyVersion)
	if v1bkt == nil {
		return nil
	}

	// iterate through each namespace
	v1c := v1bkt.Cursor()

	for k, v := v1c.First(); k != nil; k, v = v1c.Next() {
		if v != nil {
			continue
		}
		nbkt := v1bkt.Bucket(k)

		sbkt := nbkt.Bucket(bucketKeyObjectSnapshots)
		if sbkt != nil {
			// Iterate through each snapshotter
			if err := sbkt.ForEach(func(sk, sv []byte) error {
				if sv != nil {
					return nil
				}
				snbkt := sbkt.Bucket(sk)

				// Iterate through each snapshot
				return snbkt.ForEach(func(k, v []byte) error {
					if v != nil {
						return nil
					}
					parent := snbkt.Bucket(k).Get(bucketKeyParent)
					if len(parent) > 0 {
						pbkt := snbkt.Bucket(parent)
						if pbkt == nil {
							// Not enforcing consistency during migration, skip
							return nil
						}
						cbkt, err := pbkt.CreateBucketIfNotExists(bucketKeyChildren)
						if err != nil {
							return err
						}
						if err := cbkt.Put(k, nil); err != nil {
							return err
						}
					}

					return nil
				})
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// migrateIngests moves ingests from the key/value ingest bucket
// to a structured ingest bucket for storing additional state about
// an ingest.
func migrateIngests(tx *bolt.Tx) error {
	v1bkt := tx.Bucket(bucketKeyVersion)
	if v1bkt == nil {
		return nil
	}

	// iterate through each namespace
	v1c := v1bkt.Cursor()

	for k, v := v1c.First(); k != nil; k, v = v1c.Next() {
		if v != nil {
			continue
		}
		bkt := v1bkt.Bucket(k).Bucket(bucketKeyObjectContent)
		if bkt == nil {
			continue
		}

		dbkt := bkt.Bucket(deprecatedBucketKeyObjectIngest)
		if dbkt == nil {
			continue
		}

		// Create new ingests bucket
		nbkt, err := bkt.CreateBucketIfNotExists(bucketKeyObjectIngests)
		if err != nil {
			return err
		}

		if err := dbkt.ForEach(func(ref, bref []byte) error {
			ibkt, err := nbkt.CreateBucketIfNotExists(ref)
			if err != nil {
				return err
			}
			return ibkt.Put(bucketKeyRef, bref)
		}); err != nil {
			return err
		}

		if err := bkt.DeleteBucket(deprecatedBucketKeyObjectIngest); err != nil {
			return err
		}
	}

	return nil
}

// noOpMigration was for a database change from boltdb/bolt which is no
// longer being supported, to go.etcd.io/bbolt which is the currently
// maintained repo for boltdb.
func noOpMigration(tx *bolt.Tx) error {
	return nil
}
