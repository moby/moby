package cachedir

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	digest "github.com/opencontainers/go-digest"
)

// Manager handles the creation and deletion of cache directories and has a
// very particular way of managing the cache. It was designed to be used by a
// caller that manages the lifetime of the directories, but delegates the
// mechanics of how to store and locate the directories to the Manager
type Manager struct {
	cacheRoot string
	mu        sync.Mutex
	inUse     map[digest.Digest]struct{}
}

// NewManager creates a new cachedir manager
func NewManager(path string) *Manager {
	return &Manager{
		cacheRoot: path,
		inUse:     map[digest.Digest]struct{}{},
	}
}

func (m *Manager) cachePathForKeyHash(keyHash digest.Digest) string {
	return filepath.Join(m.cacheRoot, keyHash.Algorithm().String(), keyHash.Hex())
}

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

func (m *Manager) sizeEstimateByKeyHash(keyHash digest.Digest) uint64 {
	dir := m.cachePathForKeyHash(keyHash)
	size, err := dirSize(dir)
	if err != nil {
		logrus.WithError(err).WithField("cachedir", dir).Warn("failed to determine size of directory")
		return 0
	}
	return size
}

func (m *Manager) cleanUpByKeyHash(keyHash digest.Digest) error {
	return os.RemoveAll(m.cachePathForKeyHash(keyHash))
}

// GetDir creates a new cache dir or returns an existing one if one already
// exists for this key from a previous download. It also keeps track of how
// many references were made to the cache dir during
// ReleaseDir or DeleteDir have to be called exactly once per GetDir call
// and GetDir shouldn't be called again until one or the other is called.
func (m *Manager) GetDir(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	hashedKey := digest.FromString(key)
	if _, ok := m.inUse[hashedKey]; ok {
		panic(fmt.Errorf("tried to get a cachedir key (%s) twice without releasing/deleting", key))
	}
	m.inUse[hashedKey] = struct{}{}

	return m.cachePathForKeyHash(hashedKey)
}

// ReleaseDir lets the Manager know that the dir is no longer in use but
// should be kept around in case we need it again.
func (m *Manager) ReleaseDir(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hashedKey := digest.FromString(key)
	if _, ok := m.inUse[hashedKey]; !ok {
		panic(fmt.Errorf("tried to release a cachedir key (%s) without getting it first", key))
	}
	delete(m.inUse, hashedKey)
}

// DeleteDir lets the manager know that the dir is no longer used and can be
// deleted. If the delete operation fails and returns an error, the key is
// still considered released.
func (m *Manager) DeleteDir(key string) error {
	m.mu.Lock()
	hashedKey := digest.FromString(key)
	logrus.WithField("key", key).WithField("hashedKey", hashedKey).Debug("cachedir manager delete")
	if _, ok := m.inUse[hashedKey]; !ok {
		m.mu.Unlock()
		panic(fmt.Errorf("tried to delete a cachedir key (%s) without getting it first", key))
	}
	delete(m.inUse, hashedKey)
	m.mu.Unlock()
	return m.cleanUpByKeyHash(hashedKey)
}

func (m *Manager) getAllKeyHashes() map[digest.Digest]struct{} {
	allKeyHashes := map[digest.Digest]struct{}{}
	algorithmDirs, err := ioutil.ReadDir(m.cacheRoot)
	if err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logrus.WithError(err).WithField("cacheRoot", m.cacheRoot).Warn("Failed to list cache root")
		}
	}
	for _, algorithmDir := range algorithmDirs {
		algorithmDirName := algorithmDir.Name()
		algorithm := filepath.Base(algorithmDirName)
		cacheDirs, err := ioutil.ReadDir(filepath.Join(m.cacheRoot, algorithm))
		if err != nil {
			logrus.WithError(err).WithField("cacheRoot", m.cacheRoot).WithField("algorithmDir", algorithmDir).Warn("Failed to list cache directory")
		}
		for _, cacheDir := range cacheDirs {
			cacheDirName := cacheDir.Name()
			keyHash := digest.NewDigestFromHex(algorithm, filepath.Base(cacheDirName))

			// safety check to make sure we don't delete anything that's not a hash
			// we created
			err := keyHash.Validate()
			if err != nil {
				logrus.WithError(err).WithField("cachedir", cacheDirName).Debug("Ignoring unexpected cache directory")
				continue
			}

			allKeyHashes[keyHash] = struct{}{}
		}
	}
	return allKeyHashes
}

// Usage returns information about how much space the download cache is
// using. If any directories or files are unreadable, we just log the
// errors and don't count the files.
func (m *Manager) Usage() types.DownloadCacheUsage {
	m.mu.Lock()
	defer m.mu.Unlock()

	allKeyHashes := m.getAllKeyHashes()

	var total uint64
	var active uint64
	var totalBytes uint64
	var activeBytes uint64
	for keyHash := range allKeyHashes {
		size := m.sizeEstimateByKeyHash(keyHash)
		totalBytes += size
		total++
		if _, ok := m.inUse[keyHash]; ok {
			active++
			activeBytes += size
		}
	}

	return types.DownloadCacheUsage{
		TotalNumber:  int64(total),
		ActiveNumber: int64(active),
		TotalBytes:   int64(totalBytes),
		ActiveBytes:  int64(activeBytes),
	}
}

// CollectGarbage deletes all dirs that are not currently in use (ones that
// don't have a GetDir without a matching ReleaseDir/DeleteDir call) and
// returns the amount of space saved in bytes.
func (m *Manager) CollectGarbage() (uint64, error) {
	// safety check to make sure we don't try to delete files in / or pwd
	if m.cacheRoot == "" {
		return 0, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// mark
	allKeyHashes := m.getAllKeyHashes()

	// set subtraction allKeyHashes = allKeyHashes - c.inUse
	for keyHash := range m.inUse {
		delete(allKeyHashes, keyHash)
	}

	// sweep
	var total uint64
	errs := []string{}
	for keyHash := range allKeyHashes {
		total += m.sizeEstimateByKeyHash(keyHash)
		err := m.cleanUpByKeyHash(keyHash)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return total, fmt.Errorf("failed to delete some files during cachedir manager CollectGarbage: %s", strings.Join(errs, "; "))
	}
	return total, nil
}
