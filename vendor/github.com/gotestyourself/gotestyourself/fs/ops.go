package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// PathOp is a function which accepts a Path to perform some operation
type PathOp func(path Path) error

// WithContent writes content to a file at Path
func WithContent(content string) PathOp {
	return func(path Path) error {
		return ioutil.WriteFile(path.Path(), []byte(content), 0644)
	}
}

// WithBytes write bytes to a file at Path
func WithBytes(raw []byte) PathOp {
	return func(path Path) error {
		return ioutil.WriteFile(path.Path(), raw, 0644)
	}
}

// AsUser changes ownership of the file system object at Path
func AsUser(uid, gid int) PathOp {
	return func(path Path) error {
		return os.Chown(path.Path(), uid, gid)
	}
}

// WithFile creates a file in the directory at path with content
func WithFile(filename, content string) PathOp {
	return func(path Path) error {
		return createFile(path.Path(), filename, content)
	}
}

func createFile(dir, filename, content string) error {
	fullpath := filepath.Join(dir, filepath.FromSlash(filename))
	return ioutil.WriteFile(fullpath, []byte(content), 0644)
}

// WithFiles creates all the files in the directory at path with their content
func WithFiles(files map[string]string) PathOp {
	return func(path Path) error {
		for filename, content := range files {
			if err := createFile(path.Path(), filename, content); err != nil {
				return err
			}
		}
		return nil
	}
}

// FromDir copies the directory tree from the source path into the new Dir
func FromDir(source string) PathOp {
	return func(path Path) error {
		return copyDirectory(source, path.Path())
	}
}

func copyDirectory(source, dest string) error {
	entries, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := os.Mkdir(destPath, 0755); err != nil {
				return err
			}
			if err := copyDirectory(sourcePath, destPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sourcePath, destPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, dest string) error {
	content, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, content, 0644)
}
