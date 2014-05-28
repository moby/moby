package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dotcloud/docker/engine"
)

type Secrets struct {
	root    string
	hostDir string
}

type Secret struct {
	Name      string
	IsDir     bool
	HostBased bool
}

type SecretData struct {
	Name string
	Data []byte
}

func (s SecretData) SaveTo(dir string) error {
	path := filepath.Join(dir, s.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := ioutil.WriteFile(path, s.Data, 0755); err != nil {
		return err
	}
	return nil
}

func NewSecrets(root, hostDir string) (*Secrets, error) {
	s := &Secrets{root: filepath.Join(root, "secrets"), hostDir: hostDir}

	if err := os.MkdirAll(s.root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return s, nil
}

func listDir(dirPath, prefix string, hostBased bool, all bool) []Secret {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	s := []Secret{}

	for _, f := range files {
		fullName := filepath.Join(prefix, f.Name())
		secret := Secret{Name: fullName}
		secret.IsDir = f.IsDir()
		secret.HostBased = hostBased
		s = append(s, secret)

		if secret.IsDir && all {
			subs := listDir(filepath.Join(dirPath, fullName), fullName, hostBased, all)
			s = append(s, subs...)
		}
	}

	return s
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
	} else {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []SecretData{{Name: name, Data: bytes}}, nil
	}
}

func (s *Secrets) GetHostData() ([]SecretData, error) {
	return readAll(s.hostDir, "")
}

func (s *Secrets) GetData(secret string) ([]SecretData, error) {
	data, err := readFile(s.root, secret)
	if data != nil {
		return data, nil
	}

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	data, err = readFile(s.hostDir, secret)
	if err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("Secret %s doesn't exists", secret)
	}
	return data, err
}

func (s *Secrets) List(all bool) []Secret {
	dirs := listDir(s.root, "", false, all)
	hostDirs := listDir(s.hostDir, "", true, all)

	return append(dirs, hostDirs...)
}

func (s *Secrets) Add(name string, data []byte) error {
	hostPath := filepath.Join(s.hostDir, filepath.Clean(name))
	if _, err := os.Stat(hostPath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("Secret %s already exists", name)
	}
	rootPath := filepath.Join(s.root, filepath.Clean(name))
	if _, err := os.Stat(rootPath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("Secret %s already exists", name)
	}
	if err := os.MkdirAll(filepath.Dir(rootPath), 0700); err != nil && !os.IsExist(err) {
		return err
	}

	if err := ioutil.WriteFile(rootPath, data, 0600); err != nil {
		return err
	}
	return nil
}

func (s *Secrets) Remove(name string) error {
	hostPath := filepath.Join(s.hostDir, filepath.Clean(name))
	if _, err := os.Stat(hostPath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("Can't remove host-based secret")
	}
	rootPath := filepath.Join(s.root, filepath.Clean(name))
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return fmt.Errorf("Secret %s doesn't exists", name)
	}
	if err := os.RemoveAll(rootPath); err != nil {
		return err
	}
	return nil
}

func (s *Secrets) ListSecrets(job *engine.Job) engine.Status {
	if len(job.Args) != 0 {
		return job.Errorf("usage: secrets list")
	}
	secrets := s.List(job.GetenvBool("all"))

	outs := engine.NewTable("Name", 0)

	for _, secret := range secrets {
		out := &engine.Env{}
		out.Set("Name", secret.Name)
		out.SetBool("IsDir", secret.IsDir)
		out.SetBool("HostBased", secret.HostBased)
		outs.Add(out)
	}

	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (s *Secrets) SecretAdd(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: secret add SECRET")
	}

	name := job.Args[0]

	data, err := ioutil.ReadAll(job.Stdin)
	if err != nil {
		return job.Error(err)
	}

	if err := s.Add(name, data); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (s *Secrets) SecretDelete(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: secret rm SECRET")
	}
	name := job.Args[0]
	if err := s.Remove(name); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (s *Secrets) Install(eng *engine.Engine) error {
	if err := eng.Register("secrets_list", s.ListSecrets); err != nil {
		return err
	}

	if err := eng.Register("secret_delete", s.SecretDelete); err != nil {
		return err
	}

	if err := eng.Register("secret_add", s.SecretAdd); err != nil {
		return err
	}

	return nil
}
