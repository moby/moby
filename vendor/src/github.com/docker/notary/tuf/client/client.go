package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	tuf "github.com/docker/notary/tuf"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/keys"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/store"
	"github.com/docker/notary/tuf/utils"
)

const maxSize int64 = 5 << 20

// Client is a usability wrapper around a raw TUF repo
type Client struct {
	local  *tuf.Repo
	remote store.RemoteStore
	keysDB *keys.KeyDB
	cache  store.MetadataStore
}

// NewClient initialized a Client with the given repo, remote source of content, key database, and cache
func NewClient(local *tuf.Repo, remote store.RemoteStore, keysDB *keys.KeyDB, cache store.MetadataStore) *Client {
	return &Client{
		local:  local,
		remote: remote,
		keysDB: keysDB,
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
			logrus.Error("client Update (Root):", err)
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
		logrus.Errorf("Client Update (Timestamp): %s", err.Error())
		return err
	}
	err = c.downloadSnapshot()
	if err != nil {
		logrus.Errorf("Client Update (Snapshot): %s", err.Error())
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
		logrus.Errorf("Client Update (Targets): %s", err.Error())
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
	role := data.CanonicalRootRole
	size := maxSize
	var expectedSha256 []byte
	if c.local.Snapshot != nil {
		size = c.local.Snapshot.Signed.Meta[role].Length
		expectedSha256 = c.local.Snapshot.Signed.Meta[role].Hashes["sha256"]
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
	// as c.keysDB contains the root keys we bootstrapped with.
	// Still need to determine if there has been a root key update and
	// confirm signature with new root key
	logrus.Debug("verifying root with existing keys")
	err := signed.Verify(s, role, minVersion, c.keysDB)
	if err != nil {
		logrus.Debug("root did not verify with existing keys")
		return err
	}

	// This will cause keyDB to get updated, overwriting any keyIDs associated
	// with the roles in root.json
	logrus.Debug("updating known root roles and keys")
	root, err := data.RootFromSigned(s)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	err = c.local.SetRoot(root)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	// verify again now that the old keys have been replaced with the new keys.
	// TODO(endophage): be more intelligent and only re-verify if we detect
	//                  there has been a change in root keys
	logrus.Debug("verifying root with updated keys")
	err = signed.Verify(s, role, minVersion, c.keysDB)
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
	logrus.Debug("downloadTimestamp")
	role := data.CanonicalTimestampRole

	// We may not have a cached timestamp if this is the first time
	// we're interacting with the repo. This will result in the
	// version being 0
	var download bool
	old := &data.Signed{}
	version := 0
	cachedTS, err := c.cache.GetMeta(role, maxSize)
	if err == nil {
		err := json.Unmarshal(cachedTS, old)
		if err == nil {
			ts, err := data.TimestampFromSigned(old)
			if err == nil {
				version = ts.Signed.Version
			}
		} else {
			old = nil
		}
	}
	// unlike root, targets and snapshot, always try and download timestamps
	// from remote, only using the cache one if we couldn't reach remote.
	raw, s, err := c.downloadSigned(role, maxSize, nil)
	if err != nil || len(raw) == 0 {
		if err, ok := err.(store.ErrMetaNotFound); ok {
			return err
		}
		if old == nil {
			if err == nil {
				// couldn't retrieve data from server and don't have valid
				// data in cache.
				return store.ErrMetaNotFound{Resource: data.CanonicalTimestampRole}
			}
			return err
		}
		logrus.Debug("using cached timestamp")
		s = old
	} else {
		download = true
	}
	err = signed.Verify(s, role, version, c.keysDB)
	if err != nil {
		return err
	}
	logrus.Debug("successfully verified timestamp")
	if download {
		c.cache.SetMeta(role, raw)
	}
	ts, err := data.TimestampFromSigned(s)
	if err != nil {
		return err
	}
	c.local.SetTimestamp(ts)
	return nil
}

// downloadSnapshot is responsible for downloading the snapshot.json
func (c *Client) downloadSnapshot() error {
	logrus.Debug("downloadSnapshot")
	role := data.CanonicalSnapshotRole
	if c.local.Timestamp == nil {
		return ErrMissingMeta{role: "snapshot"}
	}
	size := c.local.Timestamp.Signed.Meta[role].Length
	expectedSha256, ok := c.local.Timestamp.Signed.Meta[role].Hashes["sha256"]
	if !ok {
		return ErrMissingMeta{role: "snapshot"}
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
			snap, err := data.TimestampFromSigned(old)
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

	err = signed.Verify(s, role, version, c.keysDB)
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
	stack := utils.NewStack()
	stack.Push(role)
	for !stack.Empty() {
		role, err := stack.PopString()
		if err != nil {
			return err
		}
		if c.local.Snapshot == nil {
			return ErrMissingMeta{role: role}
		}
		snap := c.local.Snapshot.Signed
		root := c.local.Root.Signed
		r := c.keysDB.GetRole(role)
		if r == nil {
			return fmt.Errorf("Invalid role: %s", role)
		}
		keyIDs := r.KeyIDs
		s, err := c.getTargetsFile(role, keyIDs, snap.Meta, root.ConsistentSnapshot, r.Threshold)
		if err != nil {
			if _, ok := err.(ErrMissingMeta); ok && role != data.CanonicalTargetsRole {
				// if the role meta hasn't been published,
				// that's ok, continue
				continue
			}
			logrus.Error("Error getting targets file:", err)
			return err
		}
		t, err := data.TargetsFromSigned(s)
		if err != nil {
			return err
		}
		err = c.local.SetTargets(role, t)
		if err != nil {
			return err
		}

		// push delegated roles contained in the targets file onto the stack
		for _, r := range t.Signed.Delegations.Roles {
			stack.Push(r.Name)
		}
	}
	return nil
}

func (c *Client) downloadSigned(role string, size int64, expectedSha256 []byte) ([]byte, *data.Signed, error) {
	raw, err := c.remote.GetMeta(role, size)
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

func (c Client) getTargetsFile(role string, keyIDs []string, snapshotMeta data.Files, consistent bool, threshold int) (*data.Signed, error) {
	// require role exists in snapshots
	roleMeta, ok := snapshotMeta[role]
	if !ok {
		return nil, ErrMissingMeta{role: role}
	}
	expectedSha256, ok := snapshotMeta[role].Hashes["sha256"]
	if !ok {
		return nil, ErrMissingMeta{role: role}
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
			targ, err := data.TargetsFromSigned(old)
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
		rolePath, err := c.RoleTargetsPath(role, hex.EncodeToString(expectedSha256), consistent)
		if err != nil {
			return nil, err
		}
		raw, s, err = c.downloadSigned(rolePath, size, expectedSha256)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Debug("using cached ", role)
		s = old
	}

	err = signed.Verify(s, role, version, c.keysDB)
	if err != nil {
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

// RoleTargetsPath generates the appropriate HTTP URL for the targets file,
// based on whether the repo is marked as consistent.
func (c Client) RoleTargetsPath(role string, hashSha256 string, consistent bool) (string, error) {
	if consistent {
		// Use path instead of filepath since we refer to the TUF role directly instead of its target files
		dir := path.Dir(role)
		if strings.Contains(role, "/") {
			lastSlashIdx := strings.LastIndex(role, "/")
			role = role[lastSlashIdx+1:]
		}
		role = path.Join(
			dir,
			fmt.Sprintf("%s.%s.json", hashSha256, role),
		)
	}
	return role, nil
}

// TargetMeta ensures the repo is up to date. It assumes downloadTargets
// has already downloaded all delegated roles
func (c Client) TargetMeta(role, path string, excludeRoles ...string) (*data.FileMeta, string) {
	excl := make(map[string]bool)
	for _, r := range excludeRoles {
		excl[r] = true
	}

	pathDigest := sha256.Sum256([]byte(path))
	pathHex := hex.EncodeToString(pathDigest[:])

	// FIFO list of targets delegations to inspect for target
	roles := []string{role}
	var (
		meta *data.FileMeta
		curr string
	)
	for len(roles) > 0 {
		// have to do these lines here because of order of execution in for statement
		curr = roles[0]
		roles = roles[1:]

		meta = c.local.TargetMeta(curr, path)
		if meta != nil {
			// we found the target!
			return meta, curr
		}
		delegations := c.local.TargetDelegations(curr, path, pathHex)
		for _, d := range delegations {
			if !excl[d.Name] {
				roles = append(roles, d.Name)
			}
		}
	}
	return meta, ""
}

// DownloadTarget downloads the target to dst from the remote
func (c Client) DownloadTarget(dst io.Writer, path string, meta *data.FileMeta) error {
	reader, err := c.remote.GetTarget(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	r := io.TeeReader(
		io.LimitReader(reader, meta.Length),
		dst,
	)
	err = utils.ValidateTarget(r, meta)
	return err
}
