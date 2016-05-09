package trustmanager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// SimpleFileStore implements FileStore
type SimpleFileStore struct {
	baseDir string
	fileExt string
	perms   os.FileMode
}

// NewFileStore creates a fully configurable file store
func NewFileStore(baseDir, fileExt string, perms os.FileMode) (*SimpleFileStore, error) {
	baseDir = filepath.Clean(baseDir)
	if err := createDirectory(baseDir, perms); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(fileExt, ".") {
		fileExt = "." + fileExt
	}

	return &SimpleFileStore{
		baseDir: baseDir,
		fileExt: fileExt,
		perms:   perms,
	}, nil
}

// NewSimpleFileStore is a convenience wrapper to create a world readable,
// owner writeable filestore
func NewSimpleFileStore(baseDir, fileExt string) (*SimpleFileStore, error) {
	return NewFileStore(baseDir, fileExt, visible)
}

// NewPrivateSimpleFileStore is a wrapper to create an owner readable/writeable
// _only_ filestore
func NewPrivateSimpleFileStore(baseDir, fileExt string) (*SimpleFileStore, error) {
	return NewFileStore(baseDir, fileExt, private)
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
			trimmed := strings.TrimSuffix(fp, f.fileExt)
			files = append(files, trimmed)
		}
		return nil
	})
	return files
}

// genFileName returns the name using the right extension
func (f *SimpleFileStore) genFileName(name string) string {
	return fmt.Sprintf("%s%s", name, f.fileExt)
}

// BaseDir returns the base directory of the filestore
func (f *SimpleFileStore) BaseDir() string {
	return f.baseDir
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
