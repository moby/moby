package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const defaultLocalRegistry = "/usr/share/docker/plugins"

var (
	ErrNotFound = errors.New("Plugin not found")
)

type Registry interface {
	Plugins() ([]*Plugin, error)
	Plugin(name string) (*Plugin, error)
}

type LocalRegistry struct {
	path string
}

func newLocalRegistry(path string) *LocalRegistry {
	if len(path) == 0 {
		path = defaultLocalRegistry
	}

	return &LocalRegistry{path}
}

func (l *LocalRegistry) Plugin(name string) (*Plugin, error) {
	filepath := filepath.Join(l.path, name)
	specpath := filepath + ".spec"
	if fi, err := os.Stat(specpath); err == nil {
		return readPluginSpecInfo(specpath, fi)
	}

	socketpath := filepath + ".sock"
	if fi, err := os.Stat(socketpath); err == nil {
		return readPluginSocketInfo(socketpath, fi)
	}

	jsonpath := filepath + ".json"
	if _, err := os.Stat(jsonpath); err == nil {
		return readPluginJSONInfo(name, jsonpath)
	}

	return nil, ErrNotFound
}

func readPluginSpecInfo(path string, fi os.FileInfo) (*Plugin, error) {
	name := strings.Split(fi.Name(), ".")[0]

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	addr := strings.TrimSpace(string(content))

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	if len(u.Scheme) == 0 {
		return nil, fmt.Errorf("Unknown protocol")
	}

	return newLocalPlugin(name, addr), nil
}

func readPluginSocketInfo(path string, fi os.FileInfo) (*Plugin, error) {
	name := strings.Split(fi.Name(), ".")[0]

	if fi.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("%s is not a socket", path)
	}

	return newLocalPlugin(name, "unix://"+path), nil
}

func readPluginJSONInfo(name, path string) (*Plugin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var p Plugin
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		return nil, err
	}
	p.Name = name
	if len(p.TLSConfig.CAFile) == 0 {
		p.TLSConfig.InsecureSkipVerify = true
	}

	return &p, nil
}
