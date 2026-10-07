package bbolt

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	bolt "go.etcd.io/bbolt"
)

func openDatabase(dataRoot string) (*bolt.DB, error) {
	root, err := os.OpenRoot(dataRoot)
	if err != nil {
		return nil, fmt.Errorf("open daemon data root: %w", err)
	}
	defer root.Close()
	dir, err := privateDirectory(root, "extensions")
	if err != nil {
		return nil, fmt.Errorf("prepare extension storage: %w", err)
	}
	defer dir.Close()
	if err := syncDirectory(root); err != nil {
		return nil, fmt.Errorf("sync extension storage parent: %w", err)
	}

	db, err := bolt.Open("records.db", 0o600, &bolt.Options{
		// Another host must not silently share the daemon's database or block
		// extension initialization indefinitely while waiting for its lock.
		Timeout: time.Second,
		OpenFile: func(name string, _ int, _ os.FileMode) (*os.File, error) {
			return privateFile(dir, name)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open extension storage database: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		db.Close()
		return nil, fmt.Errorf("sync extension storage directory: %w", err)
	}
	return db, nil
}

// privateFile rejects symlinks and checks the opened file before allowing Bolt
// to modify it, including when the database already exists.
func privateFile(root *os.Root, name string) (*os.File, error) {
	f, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		return f, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("storage file %q is not a regular file (symlinks are not allowed)", name)
	}
	f, err = root.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err == nil && !os.SameFile(info, opened) {
		err = fmt.Errorf("storage file %q changed while opening", name)
	}
	if err == nil {
		err = f.Chmod(0o600)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// privateDirectory creates or opens a single directory without accepting a
// symlink, including a symlink to a sibling namespace within the same root.
// Root operations contain traversal; SameFile detects replacement between the
// no-follow inspection and opening, before any permissions are changed.
func privateDirectory(parent *os.Root, name string) (*os.Root, error) {
	if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("storage path %q is not a directory (symlinks are not allowed)", name)
	}
	dir, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	// Chmod the opened directory, not its possibly replaced pathname.
	f, err := dir.Open(".")
	if err != nil {
		dir.Close()
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err == nil && !os.SameFile(info, opened) {
		err = fmt.Errorf("storage directory %q changed while opening", name)
	}
	if err == nil {
		err = f.Chmod(0o700)
	}
	if err != nil {
		dir.Close()
		return nil, err
	}
	return dir, nil
}

func syncDirectory(root *os.Root) error {
	// Windows does not support flushing directory handles with File.Sync.
	// Bolt still flushes its database file on every commit.
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
