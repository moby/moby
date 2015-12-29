package container

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// Secret info
type Secret struct {
	Name      string
	IsDir     bool
	HostBased bool
}

// SecretData info
type SecretData struct {
	Name string
	Data []byte
}

// SaveTo saves secret data to given directory
func (s SecretData) SaveTo(dir string) error {
	path := filepath.Join(dir, s.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return ioutil.WriteFile(path, s.Data, 0755)
}

func readAll(root, prefix string) ([]SecretData, error) {
	path := filepath.Join(root, prefix)

	data := []SecretData{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}

		return nil, err
	}

	for _, f := range files {
		fileData, err := readFile(root, filepath.Join(prefix, f.Name()))
		if err != nil {
			// If the file did not exist, might be a dangling symlink
			// Ignore the error
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		data = append(data, fileData...)
	}

	return data, nil
}

func readFile(root, name string) ([]SecretData, error) {
	path := filepath.Join(root, name)

	s, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if s.IsDir() {
		dirData, err := readAll(root, name)
		if err != nil {
			return nil, err
		}
		return dirData, nil
	}
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return []SecretData{{Name: name, Data: bytes}}, nil
}

func getHostSecretData() ([]SecretData, error) {
	return readAll("/usr/share/rhel/secrets", "")
}
