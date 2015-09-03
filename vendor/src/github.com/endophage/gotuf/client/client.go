package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	tuf "github.com/endophage/gotuf"
	"github.com/endophage/gotuf/data"
	"github.com/endophage/gotuf/keys"
	"github.com/endophage/gotuf/signed"
	"github.com/endophage/gotuf/store"
	"github.com/endophage/gotuf/utils"
)

const maxSize int64 = 5 << 20

type Client struct {
	local  *tuf.TufRepo
	remote store.RemoteStore
	keysDB *keys.KeyDB
	cache  store.MetadataStore
}

func NewClient(local *tuf.TufRepo, remote store.RemoteStore, keysDB *keys.KeyDB, cache store.MetadataStore) *Client {
	return &Client{
		local:  local,
		remote: remote,
		keysDB: keysDB,
		cache:  cache,
	}
}

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
			logrus.Errorf("client Update (Root):", err)
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
		return tuf.ErrLocalRootExpired{}
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
	role := data.RoleName("root")
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
	role := data.RoleName("root")
	size := maxSize
	var expectedSha256 []byte = nil
	if c.local.Snapshot != nil {
		size = c.local.Snapshot.Signed.Meta[role].Length
		expectedSha256 = c.local.Snapshot.Signed.Meta[role].Hashes["sha256"]
	}

	// if we're bootstrapping we may not have a cached root, an
	// error will result in the "previous root version" being
	// interpreted as 0.
	var download bool
	var err error
	var cachedRoot []byte = nil
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
		logrus.Debug("downloading new root")
		raw, err = c.remote.GetMeta(role, size)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		if expectedSha256 != nil && !bytes.Equal(hash[:], expectedSha256) {
			// if we don't have an expected sha256, we're going to trust the root
			// based purely on signature and expiry time validation
			return fmt.Errorf("Remote root sha256 did not match snapshot root sha256: %#x vs. %#x", hash, []byte(expectedSha256))
		}
		s = &data.Signed{}
		err = json.Unmarshal(raw, s)
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
func (c *Client) downloadTimestamp() error {
	logrus.Debug("downloadTimestamp")
	role := data.RoleName("timestamp")

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
	logrus.Debug("Downloading timestamp")
	raw, err := c.remote.GetMeta(role, maxSize)
	var s *data.Signed
	if err != nil || len(raw) == 0 {
		if err, ok := err.(store.ErrMetaNotFound); ok {
			return err
		}
		if old == nil {
			if err == nil {
				// couldn't retrieve data from server and don't have valid
				// data in cache.
				return store.ErrMetaNotFound{}
			}
			return err
		}
		s = old
	} else {
		download = true
		s = &data.Signed{}
		err = json.Unmarshal(raw, s)
		if err != nil {
			return err
		}
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
	role := data.RoleName("snapshot")
	size := c.local.Timestamp.Signed.Meta[role].Length
	expectedSha256, ok := c.local.Timestamp.Signed.Meta[role].Hashes["sha256"]
	if !ok {
		return fmt.Errorf("Sha256 is currently the only hash supported by this client. No Sha256 found for snapshot")
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
		logrus.Debug("downloading new snapshot")
		raw, err = c.remote.GetMeta(role, size)
		if err != nil {
			return err
		}
		genHash := sha256.Sum256(raw)
		if !bytes.Equal(genHash[:], expectedSha256) {
			return fmt.Errorf("Retrieved snapshot did not verify against hash in timestamp.")
		}
		s = &data.Signed{}
		err = json.Unmarshal(raw, s)
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

// downloadTargets is responsible for downloading any targets file
// including delegates roles. It will download the whole tree of
// delegated roles below the given one
func (c *Client) downloadTargets(role string) error {
	role = data.RoleName(role) // this will really only do something for base targets role
	snap := c.local.Snapshot.Signed
	root := c.local.Root.Signed
	r := c.keysDB.GetRole(role)
	if r == nil {
		return fmt.Errorf("Invalid role: %s", role)
	}
	keyIDs := r.KeyIDs
	s, err := c.GetTargetsFile(role, keyIDs, snap.Meta, root.ConsistentSnapshot, r.Threshold)
	if err != nil {
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

	return nil
}

func (c Client) GetTargetsFile(role string, keyIDs []string, snapshotMeta data.Files, consistent bool, threshold int) (*data.Signed, error) {
	// require role exists in snapshots
	roleMeta, ok := snapshotMeta[role]
	if !ok {
		return nil, fmt.Errorf("Snapshot does not contain target role")
	}
	expectedSha256, ok := snapshotMeta[role].Hashes["sha256"]
	if !ok {
		return nil, fmt.Errorf("Sha256 is currently the only hash supported by this client. No Sha256 found for targets role %s", role)
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

	var s *data.Signed
	if download {
		rolePath, err := c.RoleTargetsPath(role, hex.EncodeToString(expectedSha256), consistent)
		if err != nil {
			return nil, err
		}
		raw, err = c.remote.GetMeta(rolePath, snapshotMeta[role].Length)
		if err != nil {
			return nil, err
		}
		s = &data.Signed{}
		err = json.Unmarshal(raw, s)
		if err != nil {
			logrus.Error("Error unmarshalling targets file:", err)
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
			logrus.Errorf("Failed to write snapshot to local cache: %s", err.Error())
		}
	}
	return s, nil
}

// RoleTargetsPath generates the appropriate filename for the targets file,
// based on whether the repo is marked as consistent.
func (c Client) RoleTargetsPath(role string, hashSha256 string, consistent bool) (string, error) {
	if consistent {
		dir := filepath.Dir(role)
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

// TargetMeta ensures the repo is up to date, downloading the minimum
// necessary metadata files
func (c Client) TargetMeta(path string) (*data.FileMeta, error) {
	c.Update()
	var meta *data.FileMeta

	pathDigest := sha256.Sum256([]byte(path))
	pathHex := hex.EncodeToString(pathDigest[:])

	// FIFO list of targets delegations to inspect for target
	roles := []string{data.ValidRoles["targets"]}
	var role string
	for len(roles) > 0 {
		// have to do these lines here because of order of execution in for statement
		role = roles[0]
		roles = roles[1:]

		// Download the target role file if necessary
		err := c.downloadTargets(role)
		if err != nil {
			// as long as we find a valid target somewhere we're happy.
			// continue and search other delegated roles if any
			continue
		}

		meta = c.local.TargetMeta(role, path)
		if meta != nil {
			// we found the target!
			return meta, nil
		}
		delegations := c.local.TargetDelegations(role, path, pathHex)
		for _, d := range delegations {
			roles = append(roles, d.Name)
		}
	}
	return meta, nil
}

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
