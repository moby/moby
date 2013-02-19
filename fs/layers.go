package fs

import (
	"errors"
	"path"
	"path/filepath"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"fmt"
	"github.com/dotcloud/docker/future"
)

type LayerStore struct {
	Root	string
}

type Compression uint32

const (
	Uncompressed	Compression = iota
	Bzip2
	Gzip
)

func NewLayerStore(root string) (*LayerStore, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// Create the root directory if it doesn't exists
	if err := os.Mkdir(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	return &LayerStore{
		Root: abspath,
	}, nil
}

func (store *LayerStore) List() []string {
	files, err := ioutil.ReadDir(store.Root)
	if err != nil {
		return []string{}
	}
	var layers []string
	for _, st := range files {
		if st.IsDir() {
			layers = append(layers, path.Join(store.Root, st.Name()))
		}
	}
	return layers
}

func (store *LayerStore) Get(id string) string {
	if !store.Exists(id) {
		return ""
	}
	return store.layerPath(id)
}

func (store *LayerStore) rootExists() (bool, error) {
	if stat, err := os.Stat(store.Root); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	} else if !stat.IsDir() {
		return false, errors.New("Not a directory: " + store.Root)
	}
	return true, nil
}

func (store *LayerStore) Init() error {
	if exists, err := store.rootExists(); err != nil {
		return err
	} else if exists {
		return nil
	}
	return os.Mkdir(store.Root, 0700)
}


func (store *LayerStore) Mktemp() (string, error) {
	tmpName := future.RandomId()
	tmpPath := path.Join(store.Root, "tmp-" + tmpName)
	if err := os.Mkdir(tmpPath, 0700); err != nil {
		return "", err
	}
	return tmpPath, nil
}

func (store *LayerStore) layerPath(id string) string {
	return path.Join(store.Root, id)
}


func (store *LayerStore) AddLayer(id string, archive Archive, stderr io.Writer, compression Compression) (string, error) {
	if _, err := os.Stat(store.layerPath(id)); err == nil {
		return "", errors.New("Layer already exists: " + id)
	}
	tmp, err := store.Mktemp()
	defer os.RemoveAll(tmp)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Mktemp failed: %s", err))
	}
	extractFlags := "-x"
	if compression == Bzip2 {
		extractFlags += "j"
	} else if compression == Gzip {
		extractFlags += "z"
	}
	untarCmd := exec.Command("tar", "-C", tmp, extractFlags)
	untarW, err := untarCmd.StdinPipe()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Could not obtain stdin pipe: %s", err))
	}
	untarStderr, err := untarCmd.StderrPipe()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Could not obtain stderr pipe: %s", err))
	}
	go io.Copy(stderr, untarStderr)
	untarStdout, err := untarCmd.StdoutPipe()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Could not obtain stdout pipe: %s", err))
	}
	go io.Copy(stderr, untarStdout)
	untarCmd.Start()
	job_copy := future.Go(func() error {
		_, err := io.Copy(untarW, archive)
		untarW.Close()
		return err
	})

	if err := untarCmd.Wait(); err != nil {
		return "", errors.New(fmt.Sprintf("Error while waiting for untar command to complete: %s", err))
	}
	if err := <-job_copy; err != nil {
		return "", errors.New(fmt.Sprintf("Error while copying: %s", err))
	}
	layer := store.layerPath(id)
	if !store.Exists(id) {
		if err := os.Rename(tmp, layer); err != nil {
			return "", errors.New(fmt.Sprintf("Could not rename temp dir to layer %s: %s", layer, err))
		}
	}
	return layer, nil
}

func (store *LayerStore) Exists(id string) bool {
	st, err := os.Stat(store.layerPath(id))
	if err != nil {
		return false
	}
	return st.IsDir()
}
