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

package updater

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// Client update workflow implementation
//
// The "Updater" provides an implementation of the TUF client workflow (ref. https://theupdateframework.github.io/specification/latest/#detailed-client-workflow).
// "Updater" provides an API to query available targets and to download them in a
// secure manner: All downloaded files are verified by signed metadata.
// High-level description of "Updater" functionality:
//   - Initializing an "Updater" loads and validates the trusted local root
//     metadata: This root metadata is used as the source of trust for all other
//     metadata.
//   - Refresh() can optionally be called to update and load all top-level
//     metadata as described in the specification, using both locally cached
//     metadata and metadata downloaded from the remote repository. If refresh is
//     not done explicitly, it will happen automatically during the first target
//     info lookup.
//   - Updater can be used to download targets. For each target:
//   - GetTargetInfo() is first used to find information about a
//     specific target. This will load new targets metadata as needed (from
//     local cache or remote repository).
//   - FindCachedTarget() can optionally be used to check if a
//     target file is already locally cached.
//   - DownloadTarget() downloads a target file and ensures it is
//     verified correct by the metadata.
//
// Thread Safety: Updater is NOT safe for concurrent use. If multiple goroutines
// need to use an Updater concurrently, external synchronization is required
// (e.g., a sync.Mutex). Alternatively, create separate Updater instances for
// each goroutine.
type Updater struct {
	trusted *trustedmetadata.TrustedMetadata
	cfg     *config.UpdaterConfig
}

type roleParentTuple struct {
	Role   string
	Parent string
}

// New creates a new Updater instance and loads trusted root metadata
func New(config *config.UpdaterConfig) (*Updater, error) {
	// make sure the trusted root metadata and remote URL were provided
	if len(config.LocalTrustedRoot) == 0 || len(config.RemoteMetadataURL) == 0 {
		return nil, fmt.Errorf("no initial trusted root metadata or remote URL provided")
	}
	// create a new trusted metadata instance using the trusted root.json
	trustedMetadataSet, err := trustedmetadata.New(config.LocalTrustedRoot)
	if err != nil {
		return nil, err
	}
	// create an updater instance
	updater := &Updater{
		cfg:     config,
		trusted: trustedMetadataSet, // save trusted metadata set
	}
	// ensure paths exist, doesn't do anything if caching is disabled
	err = updater.cfg.EnsurePathsExist()
	if err != nil {
		return nil, err
	}
	// persist the initial root metadata to the local metadata folder
	err = updater.persistMetadata(metadata.ROOT, updater.cfg.LocalTrustedRoot)
	if err != nil {
		return nil, err
	}
	// all okay, return the updater instance
	return updater, nil
}

// Refresh loads and possibly refreshes top-level metadata.
// Downloads, verifies, and loads metadata for the top-level roles in the
// specified order (root -> timestamp -> snapshot -> targets) implementing
// all the checks required in the TUF client workflow.
// A Refresh() can be done only once during the lifetime of an Updater.
// If Refresh() has not been explicitly called before the first
// GetTargetInfo() call, it will be done implicitly at that time.
// The metadata for delegated roles is not updated by Refresh():
// that happens on demand during GetTargetInfo(). However, if the
// repository uses consistent snapshots (ref. https://theupdateframework.github.io/specification/latest/#consistent-snapshots),
// then all metadata downloaded by the Updater will use the same consistent repository state.
//
// If UnsafeLocalMode is set, no network interaction is performed, only
// the cached files on disk are used. If the cached data is not complete,
// this call will fail.
func (update *Updater) Refresh() error {
	if update.cfg.UnsafeLocalMode {
		return update.unsafeLocalRefresh()
	}
	return update.onlineRefresh()
}

// onlineRefresh implements the TUF client workflow as described for
// the Refresh function.
func (update *Updater) onlineRefresh() error {
	err := update.loadRoot()
	if err != nil {
		return err
	}
	err = update.loadTimestamp()
	if err != nil {
		return err
	}
	err = update.loadSnapshot()
	if err != nil {
		return err
	}
	_, err = update.loadTargets(metadata.TARGETS, metadata.ROOT)
	if err != nil {
		return err
	}
	return nil
}

// unsafeLocalRefresh tries to load the persisted metadata already cached
// on disk. Note that this is an usafe function, and does deviate from the
// TUF specification section 5.3 to 5.7 (update phases).
// The metadata on disk are verified against the provided root though,
// and expiration dates are verified.
func (update *Updater) unsafeLocalRefresh() error {
	// Root is already loaded
	// load timestamp
	var p = filepath.Join(update.cfg.LocalMetadataDir, metadata.TIMESTAMP)
	data, err := update.loadLocalMetadata(p)
	if err != nil {
		return err
	}
	_, err = update.trusted.UpdateTimestamp(data)
	if err != nil {
		return err
	}

	// load snapshot
	p = filepath.Join(update.cfg.LocalMetadataDir, metadata.SNAPSHOT)
	data, err = update.loadLocalMetadata(p)
	if err != nil {
		return err
	}
	_, err = update.trusted.UpdateSnapshot(data, false)
	if err != nil {
		return err
	}

	// targets
	p = filepath.Join(update.cfg.LocalMetadataDir, metadata.TARGETS)
	data, err = update.loadLocalMetadata(p)
	if err != nil {
		return err
	}
	// verify and load the new target metadata
	_, err = update.trusted.UpdateDelegatedTargets(data, metadata.TARGETS, metadata.ROOT)
	if err != nil {
		return err
	}

	return nil
}

// GetTargetInfo returns metadata.TargetFiles instance with information
// for targetPath. The return value can be used as an argument to
// DownloadTarget() and FindCachedTarget().
// If Refresh() has not been called before calling
// GetTargetInfo(), the refresh will be done implicitly.
// As a side-effect this method downloads all the additional (delegated
// targets) metadata it needs to return the target information.
func (update *Updater) GetTargetInfo(targetPath string) (*metadata.TargetFiles, error) {
	// do a Refresh() in case there's no trusted targets.json yet
	if update.trusted.Targets[metadata.TARGETS] == nil {
		err := update.Refresh()
		if err != nil {
			return nil, err
		}
	}
	return update.preOrderDepthFirstWalk(targetPath)
}

// DownloadTarget downloads the target file specified by targetFile
func (update *Updater) DownloadTarget(targetFile *metadata.TargetFiles, filePath, targetBaseURL string) (string, []byte, error) {
	log := metadata.GetLogger()

	var err error
	if filePath == "" {
		filePath, err = update.generateTargetFilePath(targetFile)
		if err != nil {
			return "", nil, err
		}
	}
	if targetBaseURL == "" {
		if update.cfg.RemoteTargetsURL == "" {
			return "", nil, &metadata.ErrValue{Msg: "targetBaseURL must be set in either DownloadTarget() or the Updater struct"}
		}
		targetBaseURL = ensureTrailingSlash(update.cfg.RemoteTargetsURL)
	} else {
		targetBaseURL = ensureTrailingSlash(targetBaseURL)
	}

	targetFilePath := targetFile.Path
	targetRemotePath := targetFilePath
	consistentSnapshot := update.trusted.Root.Signed.ConsistentSnapshot
	if consistentSnapshot && update.cfg.PrefixTargetsWithHash {
		hashes := ""
		// get first hex value of hashes
		for _, v := range targetFile.Hashes {
			hashes = hex.EncodeToString(v)
			break
		}
		baseName := filepath.Base(targetFilePath)
		dirName, ok := strings.CutSuffix(targetFilePath, "/"+baseName)
		if !ok {
			// <hash>.<target-name>
			targetRemotePath = fmt.Sprintf("%s.%s", hashes, baseName)
		} else {
			// <dir-prefix>/<hash>.<target-name>
			targetRemotePath = fmt.Sprintf("%s/%s.%s", dirName, hashes, baseName)
		}
	}
	fullURL := fmt.Sprintf("%s%s", targetBaseURL, targetRemotePath)
	data, err := update.cfg.Fetcher.DownloadFile(fullURL, targetFile.Length, 0)
	if err != nil {
		return "", nil, err
	}
	err = targetFile.VerifyLengthHashes(data)
	if err != nil {
		return "", nil, err
	}

	// do not persist the target file if cache is disabled
	if !update.cfg.DisableLocalCache {
		err = os.WriteFile(filePath, data, 0644)
		if err != nil {
			return "", nil, err
		}
	}
	log.Info("Downloaded target", "path", targetFile.Path)
	return filePath, data, nil
}

// FindCachedTarget checks whether a local file is an up to date target
func (update *Updater) FindCachedTarget(targetFile *metadata.TargetFiles, filePath string) (string, []byte, error) {
	var err error
	targetFilePath := ""
	// do not look for cached target file if cache is disabled
	if update.cfg.DisableLocalCache {
		return "", nil, nil
	}
	// get its path if not provided
	if filePath == "" {
		targetFilePath, err = update.generateTargetFilePath(targetFile)
		if err != nil {
			return "", nil, err
		}
	} else {
		targetFilePath = filePath
	}
	// get file content
	data, err := os.ReadFile(targetFilePath)
	if err != nil {
		// do not want to return err, instead we say that there's no cached target available
		return "", nil, nil
	}
	// verify if the length and hashes of this target file match the expected values
	err = targetFile.VerifyLengthHashes(data)
	if err != nil {
		// do not want to return err, instead we say that there's no cached target available
		return "", nil, nil
	}
	// if all okay, return its path
	return targetFilePath, data, nil
}

// loadTimestamp load local and remote timestamp metadata
func (update *Updater) loadTimestamp() error {
	log := metadata.GetLogger()
	// try to read local timestamp
	data, err := update.loadLocalMetadata(filepath.Join(update.cfg.LocalMetadataDir, metadata.TIMESTAMP))
	if err != nil {
		// this means there's no existing local timestamp so we should proceed downloading it without the need to UpdateTimestamp
		log.Info("Local timestamp does not exist")
	} else {
		// local timestamp exists, let's try to verify it and load it to the trusted metadata set
		_, err := update.trusted.UpdateTimestamp(data)
		if err != nil {
			if errors.Is(err, &metadata.ErrRepository{}) {
				// local timestamp is not valid, proceed downloading from remote; note that this error type includes several other subset errors
				log.Info("Local timestamp is not valid")
			} else {
				// another error
				return err
			}
		}
		log.Info("Local timestamp is valid")
		// all okay, local timestamp exists and it is valid, nevertheless proceed with downloading from remote
	}
	// load from remote (whether local load succeeded or not)
	data, err = update.downloadMetadata(metadata.TIMESTAMP, update.cfg.TimestampMaxLength, "")
	if err != nil {
		return err
	}
	// try to verify and load the newly downloaded timestamp
	_, err = update.trusted.UpdateTimestamp(data)
	if err != nil {
		if errors.Is(err, &metadata.ErrEqualVersionNumber{}) {
			// if the new timestamp version is the same as current, discard the
			// new timestamp; this is normal and it shouldn't raise any error
			return nil
		} else {
			// another error
			return err
		}
	}
	// proceed with persisting the new timestamp
	err = update.persistMetadata(metadata.TIMESTAMP, data)
	if err != nil {
		return err
	}
	return nil
}

// loadSnapshot load local (and if needed remote) snapshot metadata
func (update *Updater) loadSnapshot() error {
	log := metadata.GetLogger()
	// try to read local snapshot
	data, err := update.loadLocalMetadata(filepath.Join(update.cfg.LocalMetadataDir, metadata.SNAPSHOT))
	if err != nil {
		// this means there's no existing local snapshot so we should proceed downloading it without the need to UpdateSnapshot
		log.Info("Local snapshot does not exist")
	} else {
		// successfully read a local snapshot metadata, so let's try to verify and load it to the trusted metadata set
		_, err = update.trusted.UpdateSnapshot(data, true)
		if err != nil {
			// this means snapshot verification/loading failed
			if errors.Is(err, &metadata.ErrRepository{}) {
				// local snapshot is not valid, proceed downloading from remote; note that this error type includes several other subset errors
				log.Info("Local snapshot is not valid")
			} else {
				// another error
				return err
			}
		} else {
			// this means snapshot verification/loading succeeded
			log.Info("Local snapshot is valid: not downloading new one")
			return nil
		}
	}
	// local snapshot does not exist or is invalid, update from remote
	log.Info("Failed to load local snapshot")
	if update.trusted.Timestamp == nil {
		return fmt.Errorf("trusted timestamp not set")
	}
	// extract the snapshot meta from the trusted timestamp metadata
	snapshotMeta := update.trusted.Timestamp.Signed.Meta[fmt.Sprintf("%s.json", metadata.SNAPSHOT)]
	// extract the length of the snapshot metadata to be downloaded
	length := snapshotMeta.Length
	if length == 0 {
		length = update.cfg.SnapshotMaxLength
	}
	// extract which snapshot version should be downloaded in case of consistent snapshots
	version := ""
	if update.trusted.Root.Signed.ConsistentSnapshot {
		version = strconv.FormatInt(snapshotMeta.Version, 10)
	}
	// download snapshot metadata
	data, err = update.downloadMetadata(metadata.SNAPSHOT, length, version)
	if err != nil {
		return err
	}
	// verify and load the new snapshot
	_, err = update.trusted.UpdateSnapshot(data, false)
	if err != nil {
		return err
	}
	// persist the new snapshot
	err = update.persistMetadata(metadata.SNAPSHOT, data)
	if err != nil {
		return err
	}
	return nil
}

// loadTargets load local (and if needed remote) metadata for roleName
func (update *Updater) loadTargets(roleName, parentName string) (*metadata.Metadata[metadata.TargetsType], error) {
	log := metadata.GetLogger()
	// avoid loading "roleName" more than once during "GetTargetInfo"
	role, ok := update.trusted.Targets[roleName]
	if ok {
		return role, nil
	}
	// try to read local targets
	data, err := update.loadLocalMetadata(filepath.Join(update.cfg.LocalMetadataDir, roleName))
	if err != nil {
		// this means there's no existing local target file so we should proceed downloading it without the need to UpdateDelegatedTargets
		log.Info("Local role does not exist", "role", roleName)
	} else {
		// successfully read a local targets metadata, so let's try to verify and load it to the trusted metadata set
		delegatedTargets, err := update.trusted.UpdateDelegatedTargets(data, roleName, parentName)
		if err != nil {
			// this means targets verification/loading failed
			if errors.Is(err, &metadata.ErrRepository{}) {
				// local target file is not valid, proceed downloading from remote; note that this error type includes several other subset errors
				log.Info("Local role is not valid", "role", roleName)
			} else {
				// another error
				return nil, err
			}
		} else {
			// this means targets verification/loading succeeded
			log.Info("Local role is valid: not downloading new one", "role", roleName)
			return delegatedTargets, nil
		}
	}
	// local "roleName" does not exist or is invalid, update from remote
	log.Info("Failed to load local role", "role", roleName)
	if update.trusted.Snapshot == nil {
		return nil, fmt.Errorf("trusted snapshot not set")
	}
	// extract the targets' meta from the trusted snapshot metadata
	metaInfo, ok := update.trusted.Snapshot.Signed.Meta[fmt.Sprintf("%s.json", roleName)]
	if !ok {
		return nil, fmt.Errorf("role %s not found in snapshot", roleName)
	}
	// extract the length of the target metadata to be downloaded
	length := metaInfo.Length
	if length == 0 {
		length = update.cfg.TargetsMaxLength
	}
	// extract which target metadata version should be downloaded in case of consistent snapshots
	version := ""
	if update.trusted.Root.Signed.ConsistentSnapshot {
		version = strconv.FormatInt(metaInfo.Version, 10)
	}
	// download targets metadata
	data, err = update.downloadMetadata(roleName, length, version)
	if err != nil {
		return nil, err
	}
	// verify and load the new target metadata
	delegatedTargets, err := update.trusted.UpdateDelegatedTargets(data, roleName, parentName)
	if err != nil {
		return nil, err
	}
	// persist the new target metadata
	err = update.persistMetadata(roleName, data)
	if err != nil {
		return nil, err
	}
	return delegatedTargets, nil
}

// loadRoot load remote root metadata. Sequentially load and
// persist on local disk every newer root metadata version
// available on the remote
func (update *Updater) loadRoot() error {
	// calculate boundaries
	lowerBound := update.trusted.Root.Signed.Version + 1
	upperBound := lowerBound + update.cfg.MaxRootRotations

	// loop until we find the latest available version of root (download -> verify -> load -> persist)
	for nextVersion := lowerBound; nextVersion < upperBound; nextVersion++ {
		data, err := update.downloadMetadata(metadata.ROOT, update.cfg.RootMaxLength, strconv.FormatInt(nextVersion, 10))
		if err != nil {
			// downloading the root metadata failed for some reason
			var tmpErr *metadata.ErrDownloadHTTP
			if errors.As(err, &tmpErr) {
				if tmpErr.StatusCode != http.StatusNotFound {
					// unexpected HTTP status code
					return err
				}
				// 404 means current root is newest available, so we can stop the loop and move forward
				break
			}
			// some other error ocurred
			return err
		} else {
			// downloading root metadata succeeded, so let's try to verify and load it
			_, err = update.trusted.UpdateRoot(data)
			if err != nil {
				return err
			}
			// persist root metadata to disk
			err = update.persistMetadata(metadata.ROOT, data)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// preOrderDepthFirstWalk interrogates the tree of target delegations
// in order of appearance (which implicitly order trustworthiness),
// and returns the matching target found in the most trusted role.
func (update *Updater) preOrderDepthFirstWalk(targetFilePath string) (*metadata.TargetFiles, error) {
	log := metadata.GetLogger()
	// list of delegations to be interrogated. A (role, parent role) pair
	// is needed to load and verify the delegated targets metadata
	delegationsToVisit := []roleParentTuple{{
		Role:   metadata.TARGETS,
		Parent: metadata.ROOT,
	}}
	visitedRoleNames := map[string]bool{}
	// pre-order depth-first traversal of the graph of target delegations
	for len(visitedRoleNames) <= update.cfg.MaxDelegations && len(delegationsToVisit) > 0 {
		// pop the role name from the top of the stack
		delegation := delegationsToVisit[len(delegationsToVisit)-1]
		delegationsToVisit = delegationsToVisit[:len(delegationsToVisit)-1]
		// skip any visited current role to prevent cycles
		_, ok := visitedRoleNames[delegation.Role]
		if ok {
			log.Info("Skipping visited current role", "role", delegation.Role)
			continue
		}
		// the metadata for delegation.Role must be downloaded/updated before
		// its targets, delegations, and child roles can be inspected
		targets, err := update.loadTargets(delegation.Role, delegation.Parent)
		if err != nil {
			return nil, err
		}
		target, ok := targets.Signed.Targets[targetFilePath]
		if ok {
			log.Info("Found target in current role", "role", delegation.Role)
			return target, nil
		}
		// after pre-order check, add current role to set of visited roles
		visitedRoleNames[delegation.Role] = true
		if targets.Signed.Delegations != nil {
			var childRolesToVisit []roleParentTuple
			// note that this may be a slow operation if there are many
			// delegated roles
			roles := targets.Signed.Delegations.GetRolesForTarget(targetFilePath)
			for _, rolesForTarget := range roles {
				log.Info("Adding child role", "role", rolesForTarget.Name)
				childRolesToVisit = append(childRolesToVisit, roleParentTuple{Role: rolesForTarget.Name, Parent: delegation.Role})
				if rolesForTarget.Terminating {
					log.Info("Not backtracking to other roles")
					delegationsToVisit = []roleParentTuple{}
					break
				}
			}
			// push childRolesToVisit in reverse order of appearance
			// onto delegationsToVisit. Roles are popped from the end of
			// the list
			slices.Reverse(childRolesToVisit)
			delegationsToVisit = slices.Concat(delegationsToVisit, childRolesToVisit)
		}
	}
	if len(delegationsToVisit) > 0 {
		log.Info("Too many roles left to visit for max allowed delegations",
			"roles-left", len(delegationsToVisit),
			"allowed-delegations", update.cfg.MaxDelegations)
	}
	// if this point is reached then target is not found, return nil
	return nil, fmt.Errorf("target %s not found", targetFilePath)
}

// persistMetadata writes metadata to disk atomically to avoid data loss
func (update *Updater) persistMetadata(roleName string, data []byte) error {
	log := metadata.GetLogger()
	// do not persist the metadata if we have disabled local caching
	if update.cfg.DisableLocalCache {
		return nil
	}
	// caching enabled, proceed with persisting the metadata locally
	fileName := filepath.Join(update.cfg.LocalMetadataDir, fmt.Sprintf("%s.json", url.PathEscape(roleName)))
	// create a temporary file
	file, err := os.CreateTemp(update.cfg.LocalMetadataDir, "tuf_tmp")
	if err != nil {
		return err
	}
	// change the file permissions to our desired permissions
	err = file.Chmod(0644)
	if err != nil {
		// close and delete the temporary file if there was an error while writing
		file.Close()
		errRemove := os.Remove(file.Name())
		if errRemove != nil {
			log.Info("Failed to delete temporary file", "name", file.Name())
		}
		return errors.Join(err, errRemove)
	}
	// write the data content to the temporary file
	_, err = file.Write(data)
	if err != nil {
		// close and delete the temporary file if there was an error while writing
		file.Close()
		errRemove := os.Remove(file.Name())
		if errRemove != nil {
			log.Info("Failed to delete temporary file", "name", file.Name())
		}
		return errors.Join(err, errRemove)
	}

	// can't move/rename an open file on windows, so close it first
	err = file.Close()
	if err != nil {
		return err
	}
	// if all okay, rename the temporary file to the desired one
	err = os.Rename(file.Name(), fileName)
	if err != nil {
		return err
	}
	read, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	if string(read) != string(data) {
		return fmt.Errorf("failed to persist metadata")
	}
	return nil
}

// downloadMetadata download a metadata file and return it as bytes
func (update *Updater) downloadMetadata(roleName string, length int64, version string) ([]byte, error) {
	urlPath := ensureTrailingSlash(update.cfg.RemoteMetadataURL)
	// build urlPath
	if version == "" {
		urlPath = fmt.Sprintf("%s%s.json", urlPath, url.PathEscape(roleName))
	} else {
		urlPath = fmt.Sprintf("%s%s.%s.json", urlPath, version, url.PathEscape(roleName))
	}
	return update.cfg.Fetcher.DownloadFile(urlPath, length, 0)
}

// generateTargetFilePath generates path from TargetFiles
func (update *Updater) generateTargetFilePath(tf *metadata.TargetFiles) (string, error) {
	// LocalTargetsDir can be omitted if caching is disabled
	if update.cfg.LocalTargetsDir == "" && !update.cfg.DisableLocalCache {
		return "", &metadata.ErrValue{Msg: "LocalTargetsDir must be set if filepath is not given"}
	}
	// Use URL encoded target path as filename
	return filepath.Join(update.cfg.LocalTargetsDir, url.PathEscape(tf.Path)), nil
}

// loadLocalMetadata reads a local <roleName>.json file and returns its bytes
func (update *Updater) loadLocalMetadata(roleName string) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("%s.json", roleName))
}

// GetTopLevelTargets returns the top-level target files
func (update *Updater) GetTopLevelTargets() map[string]*metadata.TargetFiles {
	return update.trusted.Targets[metadata.TARGETS].Signed.Targets
}

// GetTrustedMetadataSet returns the trusted metadata set
func (update *Updater) GetTrustedMetadataSet() trustedmetadata.TrustedMetadata {
	return *update.trusted
}

// UnsafeSetRefTime sets the reference time that the updater uses.
// This should only be done in tests.
// Using this function is useful when testing time-related behavior in go-tuf.
func (update *Updater) UnsafeSetRefTime(t time.Time) {
	update.trusted.RefTime = t
}

func IsWindowsPath(path string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z]:\\`, path)
	return match
}

// ensureTrailingSlash ensures url ends with a slash
func ensureTrailingSlash(url string) string {
	if IsWindowsPath(url) {
		slash := string(filepath.Separator)
		if strings.HasSuffix(url, slash) {
			return url
		}
		return url + slash
	}
	if strings.HasSuffix(url, "/") {
		return url
	}
	return url + "/"
}
