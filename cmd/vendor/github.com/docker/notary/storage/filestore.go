package storage

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/notary"
)

// NewFilesystemStore creates a new store in a directory tree
func NewFilesystemStore(baseDir, subDir, extension string) (*FilesystemStore, error) {
	baseDir = filepath.Join(baseDir, subDir)

	return NewFileStore(baseDir, extension, notary.PrivKeyPerms)
}

// NewFileStore creates a fully configurable file store
func NewFileStore(baseDir, fileExt string, perms os.FileMode) (*FilesystemStore, error) {
	baseDir = filepath.Clean(baseDir)
	if err := createDirectory(baseDir, perms); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(fileExt, ".") {
		fileExt = "." + fileExt
	}

	return &FilesystemStore{
		baseDir: baseDir,
		ext:     fileExt,
		perms:   perms,
	}, nil
}

// NewSimpleFileStore is a convenience wrapper to create a world readable,
// owner writeable filestore
func NewSimpleFileStore(baseDir, fileExt string) (*FilesystemStore, error) {
	return NewFileStore(baseDir, fileExt, notary.PubCertPerms)
}

// NewPrivateKeyFileStorage initializes a new filestore for private keys, appending
// the notary.PrivDir to the baseDir.
func NewPrivateKeyFileStorage(baseDir, fileExt string) (*FilesystemStore, error) {
	baseDir = filepath.Join(baseDir, notary.PrivDir)
	return NewFileStore(baseDir, fileExt, notary.PrivKeyPerms)
}

// NewPrivateSimpleFileStore is a wrapper to create an owner readable/writeable
// _only_ filestore
func NewPrivateSimpleFileStore(baseDir, fileExt string) (*FilesystemStore, error) {
	return NewFileStore(baseDir, fileExt, notary.PrivKeyPerms)
}

// FilesystemStore is a store in a locally accessible directory
type FilesystemStore struct {
	baseDir string
	ext     string
	perms   os.FileMode
}

func (f *FilesystemStore) getPath(name string) (string, error) {
	fileName := fmt.Sprintf("%s%s", name, f.ext)
	fullPath := filepath.Join(f.baseDir, fileName)

	if !strings.HasPrefix(fullPath, f.baseDir) {
		return "", ErrPathOutsideStore
	}
	return fullPath, nil
}

// GetSized returns the meta for the given name (a role) up to size bytes
// If size is "NoSizeLimit", this corresponds to "infinite," but we cut off at a
// predefined threshold "notary.MaxDownloadSize". If the file is larger than size
// we return ErrMaliciousServer for consistency with the HTTPStore
func (f *FilesystemStore) GetSized(name string, size int64) ([]byte, error) {
	p, err := f.getPath(name)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(p, os.O_RDONLY, f.perms)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrMetaNotFound{Resource: name}
		}
		return nil, err
	}
	defer file.Close()

	if size == NoSizeLimit {
		size = notary.MaxDownloadSize
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > size {
		return nil, ErrMaliciousServer{}
	}

	l := io.LimitReader(file, size)
	return ioutil.ReadAll(l)
}

// Get returns the meta for the given name.
func (f *FilesystemStore) Get(name string) ([]byte, error) {
	p, err := f.getPath(name)
	if err != nil {
		return nil, err
	}
	meta, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrMetaNotFound{Resource: name}
		}
		return nil, err
	}
	return meta, nil
}

// SetMulti sets the metadata for multiple roles in one operation
func (f *FilesystemStore) SetMulti(metas map[string][]byte) error {
	for role, blob := range metas {
		err := f.Set(role, blob)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set sets the meta for a single role
func (f *FilesystemStore) Set(name string, meta []byte) error {
	fp, err := f.getPath(name)
	if err != nil {
		return err
	}

	// Ensures the parent directories of the file we are about to write exist
	err = os.MkdirAll(filepath.Dir(fp), f.perms)
	if err != nil {
		return err
	}

	// if something already exists, just delete it and re-write it
	os.RemoveAll(fp)

	// Write the file to disk
	if err = ioutil.WriteFile(fp, meta, f.perms); err != nil {
		return err
	}
	return nil
}

// RemoveAll clears the existing filestore by removing its base directory
func (f *FilesystemStore) RemoveAll() error {
	return os.RemoveAll(f.baseDir)
}

// Remove removes the metadata for a single role - if the metadata doesn't
// exist, no error is returned
func (f *FilesystemStore) Remove(name string) error {
	p, err := f.getPath(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(p) // RemoveAll succeeds if path doesn't exist
}

// Location returns a human readable name for the storage location
func (f FilesystemStore) Location() string {
	return f.baseDir
}

// ListFiles returns a list of all the filenames that can be used with Get*
// to retrieve content from this filestore
func (f FilesystemStore) ListFiles() []string {
	files := make([]string, 0, 0)
	filepath.Walk(f.baseDir, func(fp string, fi os.FileInfo, err error) error {
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
		matched, _ := filepath.Match("*"+f.ext, fi.Name())

		if matched {
			// Find the relative path for this file relative to the base path.
			fp, err = filepath.Rel(f.baseDir, fp)
			if err != nil {
				return err
			}
			trimmed := strings.TrimSuffix(fp, f.ext)
			files = append(files, trimmed)
		}
		return nil
	})
	return files
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
