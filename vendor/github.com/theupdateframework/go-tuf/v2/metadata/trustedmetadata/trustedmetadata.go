// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package trustedmetadata

import (
	"fmt"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// TrustedMetadata struct for storing trusted metadata.
//
// Thread Safety: TrustedMetadata is NOT safe for concurrent use. If multiple
// goroutines need to access a TrustedMetadata instance concurrently, external
// synchronization is required (e.g., a sync.Mutex).
type TrustedMetadata struct {
	Root      *metadata.Metadata[metadata.RootType]
	Snapshot  *metadata.Metadata[metadata.SnapshotType]
	Timestamp *metadata.Metadata[metadata.TimestampType]
	Targets   map[string]*metadata.Metadata[metadata.TargetsType]
	RefTime   time.Time
}

// New creates a new TrustedMetadata instance which ensures that the
// collection of metadata in it is valid and trusted through the whole
// client update workflow. It provides easy ways to update the metadata
// with the caller making decisions on what is updated
func New(rootData []byte) (*TrustedMetadata, error) {
	res := &TrustedMetadata{
		Targets: map[string]*metadata.Metadata[metadata.TargetsType]{},
		RefTime: time.Now().UTC(),
	}
	// load and validate the local root metadata
	// valid initial trusted root metadata is required
	err := res.loadTrustedRoot(rootData)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// UpdateRoot verifies and loads “rootData“ as new root metadata.
// Note that an expired intermediate root is considered valid: expiry is
// only checked for the final root in UpdateTimestamp()
func (trusted *TrustedMetadata) UpdateRoot(rootData []byte) (*metadata.Metadata[metadata.RootType], error) {
	log := metadata.GetLogger()

	if trusted.Timestamp != nil {
		return nil, &metadata.ErrRuntime{Msg: "cannot update root after timestamp"}
	}
	log.Info("Updating root")
	// generate root metadata
	newRoot, err := metadata.Root().FromBytes(rootData)
	if err != nil {
		return nil, err
	}
	// check metadata type matches root
	if newRoot.Signed.Type != metadata.ROOT {
		return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("expected %s, got %s", metadata.ROOT, newRoot.Signed.Type)}
	}
	// verify that new root is signed by trusted root
	err = trusted.Root.VerifyDelegate(metadata.ROOT, newRoot)
	if err != nil {
		return nil, err
	}
	// verify version
	if newRoot.Signed.Version != trusted.Root.Signed.Version+1 {
		return nil, &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("bad version number, expected %d, got %d", trusted.Root.Signed.Version+1, newRoot.Signed.Version)}
	}
	// verify that new root is signed by itself
	err = newRoot.VerifyDelegate(metadata.ROOT, newRoot)
	if err != nil {
		return nil, err
	}
	// save root if verified
	trusted.Root = newRoot
	log.Info("Updated root", "version", trusted.Root.Signed.Version)
	return trusted.Root, nil
}

// UpdateTimestamp verifies and loads “timestampData“ as new timestamp metadata.
// Note that an intermediate timestamp is allowed to be expired. "TrustedMetadata"
// will error in this case but the intermediate timestamp will be loaded.
// This way a newer timestamp can still be loaded (and the intermediate
// timestamp will be used for rollback protection). Expired timestamp will
// prevent loading snapshot metadata.
func (trusted *TrustedMetadata) UpdateTimestamp(timestampData []byte) (*metadata.Metadata[metadata.TimestampType], error) {
	log := metadata.GetLogger()

	if trusted.Snapshot != nil {
		return nil, &metadata.ErrRuntime{Msg: "cannot update timestamp after snapshot"}
	}
	// client workflow 5.3.10: Make sure final root is not expired.
	if trusted.Root.Signed.IsExpired(trusted.RefTime) {
		// no need to check for 5.3.11 (fast forward attack recovery):
		// timestamp/snapshot can not yet be loaded at this point
		return nil, &metadata.ErrExpiredMetadata{Msg: "final root.json is expired"}
	}
	log.Info("Updating timestamp")
	newTimestamp, err := metadata.Timestamp().FromBytes(timestampData)
	if err != nil {
		return nil, err
	}
	// check metadata type matches timestamp
	if newTimestamp.Signed.Type != metadata.TIMESTAMP {
		return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("expected %s, got %s", metadata.TIMESTAMP, newTimestamp.Signed.Type)}
	}
	// verify that new timestamp is signed by trusted root
	err = trusted.Root.VerifyDelegate(metadata.TIMESTAMP, newTimestamp)
	if err != nil {
		return nil, err
	}
	// if an existing trusted timestamp is updated,
	// check for a rollback attack
	if trusted.Timestamp != nil {
		// prevent rolling back timestamp version
		if newTimestamp.Signed.Version < trusted.Timestamp.Signed.Version {
			return nil, &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("new timestamp version %d must be >= %d", newTimestamp.Signed.Version, trusted.Timestamp.Signed.Version)}
		}
		// keep using old timestamp if versions are equal
		if newTimestamp.Signed.Version == trusted.Timestamp.Signed.Version {
			log.Info("New timestamp version equals the old one", "new", newTimestamp.Signed.Version, "old", trusted.Timestamp.Signed.Version)
			return nil, &metadata.ErrEqualVersionNumber{Msg: fmt.Sprintf("new timestamp version %d equals the old one %d", newTimestamp.Signed.Version, trusted.Timestamp.Signed.Version)}
		}
		// prevent rolling back snapshot version
		snapshotMeta := trusted.Timestamp.Signed.Meta[fmt.Sprintf("%s.json", metadata.SNAPSHOT)]
		newSnapshotMeta := newTimestamp.Signed.Meta[fmt.Sprintf("%s.json", metadata.SNAPSHOT)]
		if newSnapshotMeta.Version < snapshotMeta.Version {
			return nil, &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("new snapshot version %d must be >= %d", newSnapshotMeta.Version, snapshotMeta.Version)}
		}
	}
	// expiry not checked to allow old timestamp to be used for rollback
	// protection of new timestamp: expiry is checked in UpdateSnapshot()
	// save root if verified
	trusted.Timestamp = newTimestamp
	log.Info("Updated timestamp", "version", trusted.Timestamp.Signed.Version)

	// timestamp is loaded: error if it is not valid _final_ timestamp
	err = trusted.checkFinalTimestamp()
	if err != nil {
		return nil, err
	}
	// all okay
	return trusted.Timestamp, nil
}

// checkFinalTimestamp verifies if trusted timestamp is not expired
func (trusted *TrustedMetadata) checkFinalTimestamp() error {
	if trusted.Timestamp.Signed.IsExpired(trusted.RefTime) {
		return &metadata.ErrExpiredMetadata{Msg: "timestamp.json is expired"}
	}
	return nil
}

// UpdateSnapshot verifies and loads “snapshotData“ as new snapshot metadata.
// Note that an intermediate snapshot is allowed to be expired and version
// is allowed to not match timestamp meta version: TrustedMetadata
// will error for case of expired metadata or when using bad versions but the
// intermediate snapshot will be loaded. This way a newer snapshot can still
// be loaded (and the intermediate snapshot will be used for rollback protection).
// Expired snapshot or snapshot that does not match timestamp meta version will
// prevent loading targets.
func (trusted *TrustedMetadata) UpdateSnapshot(snapshotData []byte, isTrusted bool) (*metadata.Metadata[metadata.SnapshotType], error) {
	log := metadata.GetLogger()

	if trusted.Timestamp == nil {
		return nil, &metadata.ErrRuntime{Msg: "cannot update snapshot before timestamp"}
	}
	if trusted.Targets[metadata.TARGETS] != nil {
		return nil, &metadata.ErrRuntime{Msg: "cannot update snapshot after targets"}
	}
	log.Info("Updating snapshot")

	// snapshot cannot be loaded if final timestamp is expired
	err := trusted.checkFinalTimestamp()
	if err != nil {
		return nil, err
	}
	snapshotMeta := trusted.Timestamp.Signed.Meta[fmt.Sprintf("%s.json", metadata.SNAPSHOT)]
	// verify non-trusted data against the hashes in timestamp, if any.
	// trusted snapshot data has already been verified once.
	if !isTrusted {
		err = snapshotMeta.VerifyLengthHashes(snapshotData)
		if err != nil {
			return nil, err
		}
	}
	newSnapshot, err := metadata.Snapshot().FromBytes(snapshotData)
	if err != nil {
		return nil, err
	}
	// check metadata type matches snapshot
	if newSnapshot.Signed.Type != metadata.SNAPSHOT {
		return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("expected %s, got %s", metadata.SNAPSHOT, newSnapshot.Signed.Type)}
	}
	// verify that new snapshot is signed by trusted root
	err = trusted.Root.VerifyDelegate(metadata.SNAPSHOT, newSnapshot)
	if err != nil {
		return nil, err
	}

	// version not checked against meta version to allow old snapshot to be
	// used in rollback protection: it is checked when targets is updated

	// if an existing trusted snapshot is updated, check for rollback attack
	if trusted.Snapshot != nil {
		for name, info := range trusted.Snapshot.Signed.Meta {
			newFileInfo, ok := newSnapshot.Signed.Meta[name]
			// prevent removal of any metadata in meta
			if !ok {
				return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("new snapshot is missing info for %s", name)}
			}
			// prevent rollback of any metadata versions
			if newFileInfo.Version < info.Version {
				return nil, &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("expected %s version %d, got %d", name, newFileInfo.Version, info.Version)}
			}
		}
	}

	// expiry not checked to allow old snapshot to be used for rollback
	// protection of new snapshot: it is checked when targets is updated
	trusted.Snapshot = newSnapshot
	log.Info("Updated snapshot", "version", trusted.Snapshot.Signed.Version)

	// snapshot is loaded, but we error if it's not valid _final_ snapshot
	err = trusted.checkFinalSnapshot()
	if err != nil {
		return nil, err
	}
	// all okay
	return trusted.Snapshot, nil
}

// checkFinalSnapshot verifies if it's not expired and snapshot version matches timestamp meta version
func (trusted *TrustedMetadata) checkFinalSnapshot() error {
	if trusted.Snapshot.Signed.IsExpired(trusted.RefTime) {
		return &metadata.ErrExpiredMetadata{Msg: "snapshot.json is expired"}
	}
	snapshotMeta := trusted.Timestamp.Signed.Meta[fmt.Sprintf("%s.json", metadata.SNAPSHOT)]
	if trusted.Snapshot.Signed.Version != snapshotMeta.Version {
		return &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("expected %d, got %d", snapshotMeta.Version, trusted.Snapshot.Signed.Version)}
	}
	return nil
}

// UpdateTargets verifies and loads “targetsData“ as new top-level targets metadata.
func (trusted *TrustedMetadata) UpdateTargets(targetsData []byte) (*metadata.Metadata[metadata.TargetsType], error) {
	return trusted.UpdateDelegatedTargets(targetsData, metadata.TARGETS, metadata.ROOT)
}

// UpdateDelegatedTargets verifies and loads “targetsData“ as new metadata for target “roleName“
func (trusted *TrustedMetadata) UpdateDelegatedTargets(targetsData []byte, roleName, delegatorName string) (*metadata.Metadata[metadata.TargetsType], error) {
	log := metadata.GetLogger()

	var ok bool
	if trusted.Snapshot == nil {
		return nil, &metadata.ErrRuntime{Msg: "cannot load targets before snapshot"}
	}
	// targets cannot be loaded if final snapshot is expired or its version
	// does not match meta version in timestamp
	err := trusted.checkFinalSnapshot()
	if err != nil {
		return nil, err
	}
	// check if delegator metadata is present
	if delegatorName == metadata.ROOT {
		if trusted.Root != nil {
			ok = true
		} else {
			ok = false
		}
	} else {
		_, ok = trusted.Targets[delegatorName]
	}
	if !ok {
		return nil, &metadata.ErrRuntime{Msg: "cannot load targets before delegator"}
	}
	log.Info("Updating delegated role", "role", roleName, "delegator", delegatorName)
	// Verify against the hashes in snapshot, if any
	meta, ok := trusted.Snapshot.Signed.Meta[fmt.Sprintf("%s.json", roleName)]
	if !ok {
		return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("snapshot does not contain information for %s", roleName)}
	}
	err = meta.VerifyLengthHashes(targetsData)
	if err != nil {
		return nil, err
	}
	newDelegate, err := metadata.Targets().FromBytes(targetsData)
	if err != nil {
		return nil, err
	}
	// check metadata type matches targets
	if newDelegate.Signed.Type != metadata.TARGETS {
		return nil, &metadata.ErrRepository{Msg: fmt.Sprintf("expected %s, got %s", metadata.TARGETS, newDelegate.Signed.Type)}
	}
	// get delegator metadata and verify the new delegatee
	if delegatorName == metadata.ROOT {
		err = trusted.Root.VerifyDelegate(roleName, newDelegate)
		if err != nil {
			return nil, err
		}
	} else {
		err = trusted.Targets[delegatorName].VerifyDelegate(roleName, newDelegate)
		if err != nil {
			return nil, err
		}
	}
	// check versions
	if newDelegate.Signed.Version != meta.Version {
		return nil, &metadata.ErrBadVersionNumber{Msg: fmt.Sprintf("expected %s version %d, got %d", roleName, meta.Version, newDelegate.Signed.Version)}
	}
	// check expiration
	if newDelegate.Signed.IsExpired(trusted.RefTime) {
		return nil, &metadata.ErrExpiredMetadata{Msg: fmt.Sprintf("new %s is expired", roleName)}
	}
	trusted.Targets[roleName] = newDelegate
	log.Info("Updated role", "role", roleName, "version", trusted.Targets[roleName].Signed.Version)
	return trusted.Targets[roleName], nil
}

// loadTrustedRoot verifies and loads "data" as trusted root metadata.
// Note that an expired initial root is considered valid: expiry is
// only checked for the final root in “UpdateTimestamp()“.
func (trusted *TrustedMetadata) loadTrustedRoot(rootData []byte) error {
	log := metadata.GetLogger()

	// generate root metadata
	newRoot, err := metadata.Root().FromBytes(rootData)
	if err != nil {
		return err
	}
	// check metadata type matches root
	if newRoot.Signed.Type != metadata.ROOT {
		return &metadata.ErrRepository{Msg: fmt.Sprintf("expected %s, got %s", metadata.ROOT, newRoot.Signed.Type)}
	}
	// verify root by itself
	err = newRoot.VerifyDelegate(metadata.ROOT, newRoot)
	if err != nil {
		return err
	}
	// save root if verified
	trusted.Root = newRoot
	log.Info("Loaded trusted root", "version", trusted.Root.Signed.Version)
	return nil
}
