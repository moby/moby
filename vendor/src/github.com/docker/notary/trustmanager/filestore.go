package trustmanager

import (
	"errors"
	"fmt"
	"github.com/docker/notary"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	visible = notary.PubCertPerms
	private = notary.PrivKeyPerms
)

var (
	// ErrPathOutsideStore indicates that the returned path would be
	// outside the store
	ErrPathOutsideStore = errors.New("path outside file store")
)

// LimitedFileStore implements the bare bones primitives (no hierarchy)
type LimitedFileStore interface {
	Add(fileName string, data []byte) error
	Remove(fileName string) error
	Get(fileName string) ([]byte, error)
	ListFiles() []string
}

// FileStore is the interface for full-featured FileStores
type FileStore interface {
	LimitedFileStore

	RemoveDir(directoryName string) error
	GetPath(fileName string) (string, error)
	ListDir(directoryName string) []string
	BaseDir() string
}

// SimpleFileStore implements FileStore
type SimpleFileStore struct {
	baseDir string
	fileExt string
	perms   os.FileMode
}

// NewSimpleFileStore creates a directory with 755 permissions
func NewSimpleFileStore(baseDir string, fileExt string) (*SimpleFileStore, error) {
	baseDir = filepath.Clean(baseDir)

	if err := CreateDirectory(baseDir); err != nil {
		return nil, err
	}

	return &SimpleFileStore{
		baseDir: baseDir,
		fileExt: fileExt,
		perms:   visible,
	}, nil
}

// NewPrivateSimpleFileStore creates a directory with 700 permissions
func NewPrivateSimpleFileStore(baseDir string, fileExt string) (*SimpleFileStore, error) {
	if err := CreatePrivateDirectory(baseDir); err != nil {
		return nil, err
	}

	return &SimpleFileStore{
		baseDir: baseDir,
		fileExt: fileExt,
		perms:   private,
	}, nil
}

// Add writes data to a file with a given name
func (f *SimpleFileStore) Add(name string, data []byte) error {
	filePath, err := f.GetPath(name)
	if err != nil {
		return err
	}
	createDirectory(filepath.Dir(filePath), f.perms)
	return ioutil.WriteFile(filePath, data, f.perms)
}

// Remove removes a file identified by name
func (f *SimpleFileStore) Remove(name string) error {
	// Attempt to remove
	filePath, err := f.GetPath(name)
	if err != nil {
		return err
	}
	return os.Remove(filePath)
}

// RemoveDir removes the directory identified by name
func (f *SimpleFileStore) RemoveDir(name string) error {
	dirPath := filepath.Join(f.baseDir, name)

	// Check to see if directory exists
	fi, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	// Check to see if it is a directory
	if !fi.IsDir() {
		return fmt.Errorf("directory not found: %s", name)
	}

	return os.RemoveAll(dirPath)
}

// Get returns the data given a file name
func (f *SimpleFileStore) Get(name string) ([]byte, error) {
	filePath, err := f.GetPath(name)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetPath returns the full final path of a file with a given name
func (f *SimpleFileStore) GetPath(name string) (string, error) {
	fileName := f.genFileName(name)
	fullPath := filepath.Clean(filepath.Join(f.baseDir, fileName))

	if !strings.HasPrefix(fullPath, f.baseDir) {
		return "", ErrPathOutsideStore
	}
	return fullPath, nil
}

// ListFiles lists all the files inside of a store
func (f *SimpleFileStore) ListFiles() []string {
	return f.list(f.baseDir)
}

// ListDir lists all the files inside of a directory identified by a name
func (f *SimpleFileStore) ListDir(name string) []string {
	fullPath := filepath.Join(f.baseDir, name)
	return f.list(fullPath)
}

// list lists all the files in a directory given a full path. Ignores symlinks.
func (f *SimpleFileStore) list(path string) []string {
	files := make([]string, 0, 0)
	filepath.Walk(path, func(fp string, fi os.FileInfo, err error) error {
		// If there are errors, ignore this particular file
		if err != nil {
			return nil
		}
		// Ignore if it is a directory
		if fi.IsDir() {
			return nil
		}

		// If this is a symlink, ignore it
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		// Only allow matches that end with our certificate extension (e.g. *.crt)
		matched, _ := filepath.Match("*"+f.fileExt, fi.Name())

		if matched {
			// Find the relative path for this file relative to the base path.
			fp, err = filepath.Rel(path, fp)
			if err != nil {
				return err
			}
			files = append(files, fp)
		}
		return nil
	})
	return files
}

// genFileName returns the name using the right extension
func (f *SimpleFileStore) genFileName(name string) string {
	return fmt.Sprintf("%s.%s", name, f.fileExt)
}

// BaseDir returns the base directory of the filestore
func (f *SimpleFileStore) BaseDir() string {
	return f.baseDir
}

// CreateDirectory uses createDirectory to create a chmod 755 Directory
func CreateDirectory(dir string) error {
	return createDirectory(dir, visible)
}

// CreatePrivateDirectory uses createDirectory to create a chmod 700 Directory
func CreatePrivateDirectory(dir string) error {
	return createDirectory(dir, private)
}

// createDirectory receives a string of the path to a directory.
// It does not support passing files, so the caller has to remove
// the filename by doing filepath.Dir(full_path_to_file)
func createDirectory(dir string, perms os.FileMode) error {
	// This prevents someone passing /path/to/dir and 'dir' not being created
	// If two '//' exist, MkdirAll deals it with correctly
	dir = dir + "/"
	return os.MkdirAll(dir, perms)
}

// MemoryFileStore is an implementation of LimitedFileStore that keeps
// the contents in memory.
type MemoryFileStore struct {
	sync.Mutex

	files map[string][]byte
}

// NewMemoryFileStore creates a MemoryFileStore
func NewMemoryFileStore() *MemoryFileStore {
	return &MemoryFileStore{
		files: make(map[string][]byte),
	}
}

// ErrMemFileNotFound is returned for a nonexistent "file" in the memory file
// store
var ErrMemFileNotFound = errors.New("key not found in memory file store")

// Add writes data to a file with a given name
func (f *MemoryFileStore) Add(name string, data []byte) error {
	f.Lock()
	defer f.Unlock()

	f.files[name] = data
	return nil
}

// Remove removes a file identified by name
func (f *MemoryFileStore) Remove(name string) error {
	f.Lock()
	defer f.Unlock()

	if _, present := f.files[name]; !present {
		return ErrMemFileNotFound
	}
	delete(f.files, name)

	return nil
}

// Get returns the data given a file name
func (f *MemoryFileStore) Get(name string) ([]byte, error) {
	f.Lock()
	defer f.Unlock()

	fileData, present := f.files[name]
	if !present {
		return nil, ErrMemFileNotFound
	}

	return fileData, nil
}

// ListFiles lists all the files inside of a store
func (f *MemoryFileStore) ListFiles() []string {
	var list []string

	for name := range f.files {
		list = append(list, name)
	}

	return list
}
