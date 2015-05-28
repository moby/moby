package plugins

import (
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
		return readPluginInfo(specpath, fi)
	}
	socketpath := filepath + ".sock"
	if fi, err := os.Stat(socketpath); err == nil {
		return readPluginInfo(socketpath, fi)
	}
	return nil, ErrNotFound
}

func readPluginInfo(path string, fi os.FileInfo) (*Plugin, error) {
	name := strings.Split(fi.Name(), ".")[0]

	if fi.Mode()&os.ModeSocket != 0 {
		return &Plugin{
			Name: name,
			Addr: "unix://" + path,
		}, nil
	}

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

	return &Plugin{
		Name: name,
		Addr: addr,
	}, nil
}
