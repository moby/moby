package plugins // import "github.com/docker/docker/pkg/plugins"

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

var (
	// ErrNotFound plugin not found
	ErrNotFound = errors.New("plugin not found")
	socketsPath = "/run/docker/plugins"
)

// localRegistry defines a registry that is local (using unix socket).
type localRegistry struct{}

func newLocalRegistry() localRegistry {
	return localRegistry{}
}

// Scan scans all the plugin paths and returns all the names it found
func Scan() ([]string, error) {
	var names []string
	dirEntries, err := os.ReadDir(socketsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "error reading dir entries")
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			fi, err := os.Stat(filepath.Join(socketsPath, entry.Name(), entry.Name()+".sock"))
			if err != nil {
				continue
			}

			entry = fileInfoToDirEntry(fi)
		}

		if entry.Type()&os.ModeSocket != 0 {
			names = append(names, strings.TrimSuffix(filepath.Base(entry.Name()), filepath.Ext(entry.Name())))
		}
	}

	for _, p := range specsPaths {
		dirEntries, err := os.ReadDir(p)
		if err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "error reading dir entries")
		}

		for _, fi := range dirEntries {
			if fi.IsDir() {
				infos, err := os.ReadDir(filepath.Join(p, fi.Name()))
				if err != nil {
					continue
				}

				for _, info := range infos {
					if strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())) == fi.Name() {
						fi = info
						break
					}
				}
			}

			ext := filepath.Ext(fi.Name())
			switch ext {
			case ".spec", ".json":
				plugin := strings.TrimSuffix(fi.Name(), ext)
				names = append(names, plugin)
			default:
			}
		}
	}
	return names, nil
}

// Plugin returns the plugin registered with the given name (or returns an error).
func (l *localRegistry) Plugin(name string) (*Plugin, error) {
	socketpaths := pluginPaths(socketsPath, name, ".sock")

	for _, p := range socketpaths {
		if fi, err := os.Stat(p); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return NewLocalPlugin(name, "unix://"+p), nil
		}
	}

	var txtspecpaths []string
	for _, p := range specsPaths {
		txtspecpaths = append(txtspecpaths, pluginPaths(p, name, ".spec")...)
		txtspecpaths = append(txtspecpaths, pluginPaths(p, name, ".json")...)
	}

	for _, p := range txtspecpaths {
		if _, err := os.Stat(p); err == nil {
			if strings.HasSuffix(p, ".json") {
				return readPluginJSONInfo(name, p)
			}
			return readPluginInfo(name, p)
		}
	}
	return nil, errors.Wrapf(ErrNotFound, "could not find plugin %s in v1 plugin registry", name)
}

func readPluginInfo(name, path string) (*Plugin, error) {
	content, err := os.ReadFile(path)
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

	return NewLocalPlugin(name, addr), nil
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
	p.name = name
	if p.TLSConfig != nil && len(p.TLSConfig.CAFile) == 0 {
		p.TLSConfig.InsecureSkipVerify = true
	}
	p.activateWait = sync.NewCond(&sync.Mutex{})

	return &p, nil
}

func pluginPaths(base, name, ext string) []string {
	return []string{
		filepath.Join(base, name+ext),
		filepath.Join(base, name, name+ext),
	}
}
