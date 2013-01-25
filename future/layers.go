package future

import (
	"errors"
	"path"
	"path/filepath"
	"io"
	"os"
	"os/exec"
)

type Store struct {
	Root	string
}


func NewStore(root string) (*Store, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Store{
		Root: abspath,
	}, nil
}

func (store *Store) Get(id string) (*Layer, bool) {
	layer := &Layer{Path: store.layerPath(id)}
	if !layer.Exists() {
		return nil, false
	}
	return layer, true
}

func (store *Store) Exists() (bool, error) {
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

func (store *Store) Init() error {
	if exists, err := store.Exists(); err != nil {
		return err
	} else if exists {
		return nil
	}
	return os.Mkdir(store.Root, 0700)
}


func (store *Store) Mktemp() (string, error) {
	tmpName := RandomId()
	tmpPath := path.Join(store.Root, "tmp-" + tmpName)
	if err := os.Mkdir(tmpPath, 0700); err != nil {
		return "", err
	}
	return tmpPath, nil
}

func (store *Store) layerPath(id string) string {
	return path.Join(store.Root, id)
}


func (store *Store) AddLayer(archive io.Reader, stderr io.Writer) (*Layer, error) {
	tmp, err := store.Mktemp()
	defer os.RemoveAll(tmp)
	if err != nil {
		return nil, err
	}
	untarCmd := exec.Command("tar", "-C", tmp, "-x")
	untarW, err := untarCmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	untarStderr, err := untarCmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	go io.Copy(stderr, untarStderr)
	untarStdout, err := untarCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	go io.Copy(stderr, untarStdout)
	untarCmd.Start()
	hashR, hashW := io.Pipe()
	job_copy := Go(func() error {
		_, err := io.Copy(io.MultiWriter(hashW, untarW), archive)
		hashW.Close()
		untarW.Close()
		return err
	})
	id, err := ComputeId(hashR)
	if err != nil {
		return nil, err
	}
	if err := untarCmd.Wait(); err != nil {
		return nil, err
	}
	if err := <-job_copy; err != nil {
		return nil, err
	}
	layer := &Layer{Path: store.layerPath(id)}
	if !layer.Exists() {
		if err := os.Rename(tmp, layer.Path); err != nil {
			return nil, err
		}
	}

	return layer, nil
}


type Layer struct {
	Path	string
}

func (layer *Layer) Exists() bool {
	st, err := os.Stat(layer.Path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

func (layer *Layer) Id() string {
	return path.Base(layer.Path)
}
