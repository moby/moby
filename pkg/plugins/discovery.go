package plugins

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
)

const DefaultLocalRegistry = "/usr/share/docker/plugins"

var (
	ErrNotFound = errors.New("Plugin not found")
)

type Registry interface {
	Get(name, impl string) (*Plugin, error)
}

type LocalRegistry struct {
	storage plugins
	path    string
}

func NewLocalRegistry(path string) Registry {
	if len(path) == 0 {
		path = DefaultLocalRegistry
	}

	return &LocalRegistry{
		path:    path,
		storage: plugins{plugins: make(map[string]*Plugin)},
	}
}

func (l *LocalRegistry) Get(name, imp string) (*Plugin, error) {
	pl, err := l.get(name)
	if err != nil {
		return nil, err
	}
	for _, driver := range pl.Manifest.Implements {
		logrus.Debugf("%s implements: %s", name, driver)
		if driver == imp {
			return pl, nil
		}
	}
	return nil, ErrNotImplements
}

func (l *LocalRegistry) plugin(name string) (*Plugin, error) {
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

func (l *LocalRegistry) load(name string) (*Plugin, error) {
	pl, err := l.plugin(name)
	if err != nil {
		return nil, err
	}
	if err := pl.activate(); err != nil {
		return nil, err
	}
	return pl, nil
}

func (l *LocalRegistry) get(name string) (*Plugin, error) {
	l.storage.Lock()
	defer l.storage.Unlock()

	pl, ok := l.storage.plugins[name]
	if ok {
		return pl, nil
	}
	pl, err := l.load(name)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Plugin: %v", pl)
	l.storage.plugins[name] = pl
	return pl, nil
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
