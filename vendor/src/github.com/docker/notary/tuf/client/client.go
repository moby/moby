package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	tuf "github.com/docker/notary/tuf"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/store"
	"github.com/docker/notary/tuf/utils"
)

// Client is a usability wrapper around a raw TUF repo
type Client struct {
	local  *tuf.Repo
	remote store.RemoteStore
	cache  store.MetadataStore
}

// NewClient initialized a Client with the given repo, remote source of content, and cache
func NewClient(local *tuf.Repo, remote store.RemoteStore, cache store.MetadataStore) *Client {
	return &Client{
		local:  local,
		remote: remote,
		cache:  cache,
	}
}

// Update performs an update to the TUF repo as defined by the TUF spec
func (c *Client) Update() error {
	// 1. Get timestamp
	//   a. If timestamp error (verification, expired, etc...) download new root and return to 1.
	// 2. Check if local snapshot is up to date
	//   a. If out of date, get updated snapshot
	//     i. If snapshot error, download new root and return to 1.
	// 3. Check if root correct against snapshot
	//   a. If incorrect, download new root and return to 1.
	// 4. Iteratively download and search targets and delegations to find target meta
	logrus.Debug("updating TUF client")
	err := c.update()
	if err != nil {
		logrus.Debug("Error occurred. Root will be downloaded and another update attempted")
		if err := c.downloadRoot(); err != nil {
			logrus.Debug("Client Update (Root):", err)
			return err
		}
		// If we error again, we now have the latest root and just want to fail
		// out as there's no expectation the problem can be resolved automatically
		logrus.Debug("retrying TUF client update")
		return c.update()
	}
	return nil
}

func (c *Client) update() error {
	err := c.downloadTimestamp()
	if err != nil {
		logrus.Debugf("Client Update (Timestamp): %s", err.Error())
		return err
	}
	err = c.downloadSnapshot()
	if err != nil {
		logrus.Debugf("Client Update (Snapshot): %s", err.Error())
		return err
	}
	err = c.checkRoot()
	if err != nil {
		// In this instance the root has not expired base on time, but is
		// expired based on the snapshot dictating a new root has been produced.
		logrus.Debug(err)
		return err
	}
	// will always need top level targets at a minimum
	err = c.downloadTargets("targets")
	if err != nil {
		logrus.Debugf("Client Update (Targets): %s", err.Error())
		return err
	}
	return nil
}

// checkRoot determines if the hash, and size are still those reported
// in the snapshot file. It will also check the expiry, however, if the
// hash and size in snapshot are unchanged but the root file has expired,
// there is little expectation that the situation can be remedied.
func (c Client) checkRoot() error {
	role := data.CanonicalRootRole
	size := c.local.Snapshot.Signed.Meta[role].Length
	hashSha256 := c.local.Snapshot.Signed.Meta[role].Hashes["sha256"]

	raw, err := c.cache.GetMeta("root", size)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(raw)
	if !bytes.Equal(hash[:], hashSha256) {
		return fmt.Errorf("Cached root sha256 did not match snapshot root sha256")
	}

	if int64(len(raw)) != size {
		return fmt.Errorf("Cached root size did not match snapshot size")
	}

	root := &data.SignedRoot{}
	err = json.Unmarshal(raw, root)
	if err != nil {
		return ErrCorruptedCache{file: "root.json"}
	}

	if signed.IsExpired(root.Signed.Expires) {
		return tuf.ErrLocalRootExpired{}
	}
	return nil
}

// downloadRoot is responsible for downloading the root.json
func (c *Client) downloadRoot() error {
	logrus.Debug("Downloading Root...")
	role := data.CanonicalRootRole
	// We can't read an exact size for the root metadata without risking getting stuck in the TUF update cycle
	// since it's possible that downloading timestamp/snapshot metadata may fail due to a signature mismatch
	var size int64 = -1
	var expectedSha256 []byte
	if c.local.Snapshot != nil {
		if prevRootMeta, ok := c.local.Snapshot.Signed.Meta[role]; ok {
			size = prevRootMeta.Length
			expectedSha256 = prevRootMeta.Hashes["sha256"]
		}
	}

	// if we're bootstrapping we may not have a cached root, an
	// error will result in the "previous root version" being
	// interpreted as 0.
	var download bool
	var err error
	var cachedRoot []byte
	old := &data.Signed{}
	version := 0

	if expectedSha256 != nil {
		// can only trust cache if we have an expected sha256 to trust
		cachedRoot, err = c.cache.GetMeta(role, size)
	}

	if cachedRoot == nil || err != nil {
		logrus.Debug("didn't find a cached root, must download")
		download = true
	} else {
		hash := sha256.Sum256(cachedRoot)
		if !bytes.Equal(hash[:], expectedSha256) {
			logrus.Debug("cached root's hash didn't match expected, must download")
			download = true
		}
		err := json.Unmarshal(cachedRoot, old)
		if err == nil {
			root, err := data.RootFromSigned(old)
			if err == nil {
				version = root.Signed.Version
			} else {
				logrus.Debug("couldn't parse Signed part of cached root, must download")
				download = true
			}
		} else {
			logrus.Debug("couldn't parse cached root, must download")
			download = true
		}
	}
	var s *data.Signed
	var raw []byte
	if download {
		// use consistent download if we have the checksum.
		raw, s, err = c.downloadSigned(role, size, expectedSha256)
		if err != nil {
			return err
		}
	} else {
		logrus.Debug("using cached root")
		s = old
	}
	if err := c.verifyRoot(role, s, version); err != nil {
		return err
	}
	if download {
		logrus.Debug("caching downloaded root")
		// Now that we have accepted new root, write it to cache
		if err = c.cache.SetMeta(role, raw); err != nil {
			logrus.Errorf("Failed to write root to local cache: %s", err.Error())
		}
	}
	return nil
}

func (c Client) verifyRoot(role string, s *data.Signed, minVersion int) error {
	// this will confirm that the root has been signed by the old root role
	// with the root keys we bootstrapped with.
	// Still need to determine if there has been a root key update and
	// confirm signature with new root key
	logrus.Debug("verifying root with existing keys")
	rootRole, err := c.local.GetBaseRole(role)
	if err != nil {
		logrus.Debug("no previous root role loaded")
		return err
	}
	// Verify using the rootRole loaded from the known root.json
	if err = signed.Verify(s, rootRole, minVersion); err != nil {
		logrus.Debug("root did not verify with existing keys")
		return err
	}

	logrus.Debug("updating known root roles and keys")
	root, err := data.RootFromSigned(s)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	// replace the existing root.json with the new one (just in memory, we
	// have another validation step before we fully accept the new root)
	err = c.local.SetRoot(root)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	// Verify the new root again having loaded the rootRole out of this new
	// file (verifies self-referential integrity)
	// TODO(endophage): be more intelligent and only re-verify if we detect
	//                  there has been a change in root keys
	logrus.Debug("verifying root with updated keys")
	rootRole, err = c.local.GetBaseRole(role)
	if err != nil {
		logrus.Debug("root role with new keys not loaded")
		return err
	}
	err = signed.Verify(s, rootRole, minVersion)
	if err != nil {
		logrus.Debug("root did not verify with new keys")
		return err
	}
	logrus.Debug("successfully verified root")
	return nil
}

// downloadTimestamp is responsible for downloading the timestamp.json
// Timestamps are special in that we ALWAYS attempt to download and only
// use cache if the download fails (and the cache is still valid).
func (c *Client) downloadTimestamp() error {
	logrus.Debug("Downloading Timestamp...")
	role := data.CanonicalTimestampRole

	// We may not have a cached timestamp if this is the first time
	// we're interacting with the repo. This will result in the
	// version being 0
	var (
		old     *data.Signed
		ts      *data.SignedTimestamp
		version = 0
	)
	cachedTS, err := c.cache.GetMeta(role, notary.MaxTimestampSize)
	if err == nil {
		cached := &data.Signed{}
		err := json.Unmarshal(cachedTS, cached)
		if err == nil {
			ts, err := data.TimestampFromSigned(cached)
			if err == nil {
				version = ts.Signed.Version
			}
			old = cached
		}
	}
	// unlike root, targets and snapshot, always try and download timestamps
	// from remote, only using the cache one if we couldn't reach remote.
	raw, s, err := c.downloadSigned(role, notary.MaxTimestampSize, nil)
	if err == nil {
		ts, err = c.verifyTimestamp(s, version)
		if err == nil {
			logrus.Debug("successfully verified downloaded timestamp")
			c.cache.SetMeta(role, raw)
			c.local.SetTimestamp(ts)
			return nil
		}
	}
	if old == nil {
		// couldn't retrieve valid data from server and don't have unmarshallable data in cache.
		logrus.Debug("no cached timestamp available")
		return err
	}
	logrus.Debug(err.Error())
	logrus.Warn("Error while downloading remote metadata, using cached timestamp - this might not be the latest version available remotely")
	ts, err = c.verifyTimestamp(old, version)
	if err != nil {
		return err
	}
	logrus.Debug("successfully verified cached timestamp")
	c.local.SetTimestamp(ts)
	return nil
}

// verifies that a timestamp is valid, and returned the SignedTimestamp object to add to the tuf repo
func (c *Client) verifyTimestamp(s *data.Signed, minVersion int) (*data.SignedTimestamp, error) {
	timestampRole, err := c.local.GetBaseRole(data.CanonicalTimestampRole)
	if err != nil {
		logrus.Debug("no timestamp role loaded")
		return nil, err
	}
	if err := signed.Verify(s, timestampRole, minVersion); err != nil {
		return nil, err
	}
	return data.TimestampFromSigned(s)
}

// downloadSnapshot is responsible for downloading the snapshot.json
func (c *Client) downloadSnapshot() error {
	logrus.Debug("Downloading Snapshot...")
	role := data.CanonicalSnapshotRole
	if c.local.Timestamp == nil {
		return tuf.ErrNotLoaded{Role: data.CanonicalTimestampRole}
	}
	size := c.local.Timestamp.Signed.Meta[role].Length
	expectedSha256, ok := c.local.Timestamp.Signed.Meta[role].Hashes["sha256"]
	if !ok {
		return data.ErrMissingMeta{Role: "snapshot"}
	}

	var download bool
	old := &data.Signed{}
	version := 0
	raw, err := c.cache.GetMeta(role, size)
	if raw == nil || err != nil {
		logrus.Debug("no snapshot in cache, must download")
		download = true
	} else {
		// file may have been tampered with on disk. Always check the hash!
		genHash := sha256.Sum256(raw)
		if !bytes.Equal(genHash[:], expectedSha256) {
			logrus.Debug("hash of snapshot in cache did not match expected hash, must download")
			download = true
		}
		err := json.Unmarshal(raw, old)
		if err == nil {
			snap, err := data.SnapshotFromSigned(old)
			if err == nil {
				version = snap.Signed.Version
			} else {
				logrus.Debug("Could not parse Signed part of snapshot, must download")
				download = true
			}
		} else {
			logrus.Debug("Could not parse snapshot, must download")
			download = true
		}
	}
	var s *data.Signed
	if download {
		raw, s, err = c.downloadSigned(role, size, expectedSha256)
		if err != nil {
			return err
		}
	} else {
		logrus.Debug("using cached snapshot")
		s = old
	}

	snapshotRole, err := c.local.GetBaseRole(role)
	if err != nil {
		logrus.Debug("no snapshot role loaded")
		return err
	}
	err = signed.Verify(s, snapshotRole, version)
	if err != nil {
		return err
	}
	logrus.Debug("successfully verified snapshot")
	snap, err := data.SnapshotFromSigned(s)
	if err != nil {
		return err
	}
	c.local.SetSnapshot(snap)
	if download {
		err = c.cache.SetMeta(role, raw)
		if err != nil {
			logrus.Errorf("Failed to write snapshot to local cache: %s", err.Error())
		}
	}
	return nil
}

// downloadTargets downloads all targets and delegated targets for the repository.
// It uses a pre-order tree traversal as it's necessary to download parents first
// to obtain the keys to validate children.
func (c *Client) downloadTargets(role string) error {
	logrus.Debug("Downloading Targets...")
	stack := utils.NewStack()
	stack.Push(role)
	for !stack.Empty() {
		role, err := stack.PopString()
		if err != nil {
			return err
		}
		if c.local.Snapshot == nil {
			return tuf.ErrNotLoaded{Role: data.CanonicalSnapshotRole}
		}
		snap := c.local.Snapshot.Signed
		root := c.local.Root.Signed

		s, err := c.getTargetsFile(role, snap.Meta, root.ConsistentSnapshot)
		if err != nil {
			if _, ok := err.(data.ErrMissingMeta); ok && role != data.CanonicalTargetsRole {
				// if the role meta hasn't been published,
				// that's ok, continue
				continue
			}
			logrus.Error("Error getting targets file:", err)
			return err
		}
		t, err := data.TargetsFromSigned(s, role)
		if err != nil {
			return err
		}
		err = c.local.SetTargets(role, t)
		if err != nil {
			return err
		}

		// push delegated roles contained in the targets file onto the stack
		for _, r := range t.Signed.Delegations.Roles {
			if path.Dir(r.Name) == role {
				// only load children that are direct 1st generation descendants
				// of the role we've just downloaded
				stack.Push(r.Name)
			}
		}
	}
	return nil
}

func (c *Client) downloadSigned(role string, size int64, expectedSha256 []byte) ([]byte, *data.Signed, error) {
	rolePath := utils.ConsistentName(role, expectedSha256)
	raw, err := c.remote.GetMeta(rolePath, size)
	if err != nil {
		return nil, nil, err
	}
	if expectedSha256 != nil {
		genHash := sha256.Sum256(raw)
		if !bytes.Equal(genHash[:], expectedSha256) {
			return nil, nil, ErrChecksumMismatch{role: role}
		}
	}
	s := &data.Signed{}
	err = json.Unmarshal(raw, s)
	if err != nil {
		return nil, nil, err
	}
	return raw, s, nil
}

func (c Client) getTargetsFile(role string, snapshotMeta data.Files, consistent bool) (*data.Signed, error) {
	// require role exists in snapshots
	roleMeta, ok := snapshotMeta[role]
	if !ok {
		return nil, data.ErrMissingMeta{Role: role}
	}
	expectedSha256, ok := snapshotMeta[role].Hashes["sha256"]
	if !ok {
		return nil, data.ErrMissingMeta{Role: role}
	}

	// try to get meta file from content addressed cache
	var download bool
	old := &data.Signed{}
	version := 0
	raw, err := c.cache.GetMeta(role, roleMeta.Length)
	if err != nil || raw == nil {
		logrus.Debugf("Couldn't not find cached %s, must download", role)
		download = true
	} else {
		// file may have been tampered with on disk. Always check the hash!
		genHash := sha256.Sum256(raw)
		if !bytes.Equal(genHash[:], expectedSha256) {
			download = true
		}
		err := json.Unmarshal(raw, old)
		if err == nil {
			targ, err := data.TargetsFromSigned(old, role)
			if err == nil {
				version = targ.Signed.Version
			} else {
				download = true
			}
		} else {
			download = true
		}
	}

	size := snapshotMeta[role].Length
	var s *data.Signed
	if download {
		raw, s, err = c.downloadSigned(role, size, expectedSha256)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Debug("using cached ", role)
		s = old
	}
	var targetOrDelgRole data.BaseRole
	if data.IsDelegation(role) {
		delgRole, err := c.local.GetDelegationRole(role)
		if err != nil {
			logrus.Debugf("no %s delegation role loaded", role)
			return nil, err
		}
		targetOrDelgRole = delgRole.BaseRole
	} else {
		targetOrDelgRole, err = c.local.GetBaseRole(role)
		if err != nil {
			logrus.Debugf("no %s role loaded", role)
			return nil, err
		}
	}
	if err = signed.Verify(s, targetOrDelgRole, version); err != nil {
		return nil, err
	}
	logrus.Debugf("successfully verified %s", role)
	if download {
		// if we error when setting meta, we should continue.
		err = c.cache.SetMeta(role, raw)
		if err != nil {
			logrus.Errorf("Failed to write %s to local cache: %s", role, err.Error())
		}
	}
	return s, nil
}
