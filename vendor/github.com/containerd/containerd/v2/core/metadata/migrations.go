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

import (
	"bytes"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

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
	{
		schema:  "v1",
		version: 4,
		migrate: migrateSandboxes,
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

// migrateSandboxes moves sandboxes from root bucket into v1 bucket.
func migrateSandboxes(tx *bolt.Tx) error {
	v1bkt, err := tx.CreateBucketIfNotExists(bucketKeyVersion)
	if err != nil {
		return err
	}

	deletingBuckets := [][]byte{}

	if merr := tx.ForEach(func(ns []byte, nsbkt *bolt.Bucket) error {
		// Skip v1 bucket, even if users created sandboxes in v1 namespace.
		if bytes.Equal(bucketKeyVersion, ns) {
			return nil
		}

		deletingBuckets = append(deletingBuckets, ns)

		allsbbkt := nsbkt.Bucket(bucketKeyObjectSandboxes)
		if allsbbkt == nil {
			return nil
		}

		tnsbkt, err := v1bkt.CreateBucketIfNotExists(ns)
		if err != nil {
			return fmt.Errorf("failed to create namespace %s in bucket %s: %w",
				ns, bucketKeyVersion, err)
		}

		tallsbbkt, err := tnsbkt.CreateBucketIfNotExists(bucketKeyObjectSandboxes)
		if err != nil {
			return fmt.Errorf("failed to create bucket sandboxes in namespace %s: %w", ns, err)
		}

		return allsbbkt.ForEachBucket(func(sb []byte) error {
			sbbkt := allsbbkt.Bucket(sb) // single sandbox bucket

			tsbbkt, err := tallsbbkt.CreateBucketIfNotExists(sb)
			if err != nil {
				return fmt.Errorf("failed to create sandbox object %s in namespace %s: %w",
					sb, ns, err)
			}

			// copy single
			if cerr := sbbkt.ForEach(func(key, value []byte) error {
				if value == nil {
					return nil
				}

				return tsbbkt.Put(key, value)
			}); cerr != nil {
				return cerr
			}

			return sbbkt.ForEachBucket(func(subbkt []byte) error {
				tsubbkt, err := tsbbkt.CreateBucketIfNotExists(subbkt)
				if err != nil {
					return fmt.Errorf("failed to create subbucket %s in sandbox %s (namespace %s): %w",
						subbkt, sb, ns, err)
				}

				return sbbkt.Bucket(subbkt).ForEach(func(key, value []byte) error {
					if value == nil {
						return fmt.Errorf("unexpected bucket %s", key)
					}
					return tsubbkt.Put(key, value)
				})
			})
		})
	}); merr != nil {
		return fmt.Errorf("failed to copy sandboxes into v1 bucket: %w", err)
	}

	for _, ns := range deletingBuckets {
		derr := tx.DeleteBucket(ns)
		if derr != nil {
			return fmt.Errorf("failed to cleanup bucket %s in root: %w", ns, err)
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
