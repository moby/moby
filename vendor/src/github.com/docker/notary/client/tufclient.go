package client

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	store "github.com/docker/notary/storage"
	"github.com/docker/notary/tuf"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
)

// TUFClient is a usability wrapper around a raw TUF repo
type TUFClient struct {
	remote     store.RemoteStore
	cache      store.MetadataStore
	oldBuilder tuf.RepoBuilder
	newBuilder tuf.RepoBuilder
}

// NewTUFClient initialized a TUFClient with the given repo, remote source of content, and cache
func NewTUFClient(oldBuilder, newBuilder tuf.RepoBuilder, remote store.RemoteStore, cache store.MetadataStore) *TUFClient {
	return &TUFClient{
		oldBuilder: oldBuilder,
		newBuilder: newBuilder,
		remote:     remote,
		cache:      cache,
	}
}

// Update performs an update to the TUF repo as defined by the TUF spec
func (c *TUFClient) Update() (*tuf.Repo, *tuf.Repo, error) {
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
		logrus.Debug("Resetting the TUF builder...")

		c.newBuilder = c.newBuilder.BootstrapNewBuilder()

		if err := c.downloadRoot(); err != nil {
			logrus.Debug("Client Update (Root):", err)
			return nil, nil, err
		}
		// If we error again, we now have the latest root and just want to fail
		// out as there's no expectation the problem can be resolved automatically
		logrus.Debug("retrying TUF client update")
		if err := c.update(); err != nil {
			return nil, nil, err
		}
	}
	return c.newBuilder.Finish()
}

func (c *TUFClient) update() error {
	if err := c.downloadTimestamp(); err != nil {
		logrus.Debugf("Client Update (Timestamp): %s", err.Error())
		return err
	}
	if err := c.downloadSnapshot(); err != nil {
		logrus.Debugf("Client Update (Snapshot): %s", err.Error())
		return err
	}
	// will always need top level targets at a minimum
	if err := c.downloadTargets(); err != nil {
		logrus.Debugf("Client Update (Targets): %s", err.Error())
		return err
	}
	return nil
}

// downloadRoot is responsible for downloading the root.json
func (c *TUFClient) downloadRoot() error {
	role := data.CanonicalRootRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	// We can't read an exact size for the root metadata without risking getting stuck in the TUF update cycle
	// since it's possible that downloading timestamp/snapshot metadata may fail due to a signature mismatch
	if !consistentInfo.ChecksumKnown() {
		logrus.Debugf("Loading root with no expected checksum")

		// get the cached root, if it exists, just for version checking
		cachedRoot, _ := c.cache.GetSized(role, -1)
		// prefer to download a new root
		_, remoteErr := c.tryLoadRemote(consistentInfo, cachedRoot)
		return remoteErr
	}

	_, err := c.tryLoadCacheThenRemote(consistentInfo)
	return err
}

// downloadTimestamp is responsible for downloading the timestamp.json
// Timestamps are special in that we ALWAYS attempt to download and only
// use cache if the download fails (and the cache is still valid).
func (c *TUFClient) downloadTimestamp() error {
	logrus.Debug("Loading timestamp...")
	role := data.CanonicalTimestampRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	// always get the remote timestamp, since it supersedes the local one
	cachedTS, cachedErr := c.cache.GetSized(role, notary.MaxTimestampSize)
	_, remoteErr := c.tryLoadRemote(consistentInfo, cachedTS)

	// check that there was no remote error, or if there was a network problem
	// If there was a validation error, we should error out so we can download a new root or fail the update
	switch remoteErr.(type) {
	case nil:
		return nil
	case store.ErrMetaNotFound, store.ErrServerUnavailable, store.ErrOffline, store.NetworkError:
		break
	default:
		return remoteErr
	}

	// since it was a network error: get the cached timestamp, if it exists
	if cachedErr != nil {
		logrus.Debug("no cached or remote timestamp available")
		return remoteErr
	}

	logrus.Warn("Error while downloading remote metadata, using cached timestamp - this might not be the latest version available remotely")
	err := c.newBuilder.Load(role, cachedTS, 1, false)
	if err == nil {
		logrus.Debug("successfully verified cached timestamp")
	}
	return err

}

// downloadSnapshot is responsible for downloading the snapshot.json
func (c *TUFClient) downloadSnapshot() error {
	logrus.Debug("Loading snapshot...")
	role := data.CanonicalSnapshotRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	_, err := c.tryLoadCacheThenRemote(consistentInfo)
	return err
}

// downloadTargets downloads all targets and delegated targets for the repository.
// It uses a pre-order tree traversal as it's necessary to download parents first
// to obtain the keys to validate children.
func (c *TUFClient) downloadTargets() error {
	toDownload := []data.DelegationRole{{
		BaseRole: data.BaseRole{Name: data.CanonicalTargetsRole},
		Paths:    []string{""},
	}}

	for len(toDownload) > 0 {
		role := toDownload[0]
		toDownload = toDownload[1:]

		consistentInfo := c.newBuilder.GetConsistentInfo(role.Name)
		if !consistentInfo.ChecksumKnown() {
			logrus.Debugf("skipping %s because there is no checksum for it", role.Name)
			continue
		}

		children, err := c.getTargetsFile(role, consistentInfo)
		switch err.(type) {
		case signed.ErrExpired, signed.ErrRoleThreshold:
			if role.Name == data.CanonicalTargetsRole {
				return err
			}
			logrus.Warnf("Error getting %s: %s", role.Name, err)
			break
		case nil:
			toDownload = append(children, toDownload...)
		default:
			return err
		}
	}
	return nil
}

func (c TUFClient) getTargetsFile(role data.DelegationRole, ci tuf.ConsistentInfo) ([]data.DelegationRole, error) {
	logrus.Debugf("Loading %s...", role.Name)
	tgs := &data.SignedTargets{}

	raw, err := c.tryLoadCacheThenRemote(ci)
	if err != nil {
		return nil, err
	}

	// we know it unmarshals because if `tryLoadCacheThenRemote` didn't fail, then
	// the raw has already been loaded into the builder
	json.Unmarshal(raw, tgs)
	return tgs.GetValidDelegations(role), nil
}

func (c *TUFClient) tryLoadCacheThenRemote(consistentInfo tuf.ConsistentInfo) ([]byte, error) {
	cachedTS, err := c.cache.GetSized(consistentInfo.RoleName, consistentInfo.Length())
	if err != nil {
		logrus.Debugf("no %s in cache, must download", consistentInfo.RoleName)
		return c.tryLoadRemote(consistentInfo, nil)
	}

	if err = c.newBuilder.Load(consistentInfo.RoleName, cachedTS, 1, false); err == nil {
		logrus.Debugf("successfully verified cached %s", consistentInfo.RoleName)
		return cachedTS, nil
	}

	logrus.Debugf("cached %s is invalid (must download): %s", consistentInfo.RoleName, err)
	return c.tryLoadRemote(consistentInfo, cachedTS)
}

func (c *TUFClient) tryLoadRemote(consistentInfo tuf.ConsistentInfo, old []byte) ([]byte, error) {
	consistentName := consistentInfo.ConsistentName()
	raw, err := c.remote.GetSized(consistentName, consistentInfo.Length())
	if err != nil {
		logrus.Debugf("error downloading %s: %s", consistentName, err)
		return old, err
	}

	// try to load the old data into the old builder - only use it to validate
	// versions if it loads successfully.  If it errors, then the loaded version
	// will be 1
	c.oldBuilder.Load(consistentInfo.RoleName, old, 1, true)
	minVersion := c.oldBuilder.GetLoadedVersion(consistentInfo.RoleName)
	if err := c.newBuilder.Load(consistentInfo.RoleName, raw, minVersion, false); err != nil {
		logrus.Debugf("downloaded %s is invalid: %s", consistentName, err)
		return raw, err
	}
	logrus.Debugf("successfully verified downloaded %s", consistentName)
	if err := c.cache.Set(consistentInfo.RoleName, raw); err != nil {
		logrus.Debugf("Unable to write %s to cache: %s", consistentInfo.RoleName, err)
	}
	return raw, nil
}
