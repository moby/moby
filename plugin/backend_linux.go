// +build linux

package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/plugin/distribution"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/reference"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var (
	validFullID    = regexp.MustCompile(`^([a-f0-9]{64})$`)
	validPartialID = regexp.MustCompile(`^([a-f0-9]{1,64})$`)
)

// Disable deactivates a plugin, which implies that they cannot be used by containers.
func (pm *Manager) Disable(name string) error {
	p, err := pm.pluginStore.GetByName(name)
	if err != nil {
		return err
	}
	pm.mu.RLock()
	c := pm.cMap[p]
	pm.mu.RUnlock()

	if err := pm.disable(p, c); err != nil {
		return err
	}
	pm.pluginEventLogger(p.GetID(), name, "disable")
	return nil
}

// Enable activates a plugin, which implies that they are ready to be used by containers.
func (pm *Manager) Enable(name string, config *types.PluginEnableConfig) error {
	p, err := pm.pluginStore.GetByName(name)
	if err != nil {
		return err
	}

	c := &controller{timeoutInSecs: config.Timeout}
	if err := pm.enable(p, c, false); err != nil {
		return err
	}
	pm.pluginEventLogger(p.GetID(), name, "enable")
	return nil
}

// Inspect examines a plugin config
func (pm *Manager) Inspect(refOrID string) (tp types.Plugin, err error) {
	// Match on full ID
	if validFullID.MatchString(refOrID) {
		p, err := pm.pluginStore.GetByID(refOrID)
		if err == nil {
			return p.PluginObj, nil
		}
	}

	// Match on full name
	if pluginName, err := getPluginName(refOrID); err == nil {
		if p, err := pm.pluginStore.GetByName(pluginName); err == nil {
			return p.PluginObj, nil
		}
	}

	// Match on partial ID
	if validPartialID.MatchString(refOrID) {
		p, err := pm.pluginStore.Search(refOrID)
		if err == nil {
			return p.PluginObj, nil
		}
		return tp, err
	}

	return tp, fmt.Errorf("no such plugin name or ID associated with %q", refOrID)
}

func (pm *Manager) pull(name string, metaHeader http.Header, authConfig *types.AuthConfig) (reference.Named, distribution.PullData, error) {
	ref, err := distribution.GetRef(name)
	if err != nil {
		logrus.Debugf("error in distribution.GetRef: %v", err)
		return nil, nil, err
	}
	name = ref.String()

	if p, _ := pm.pluginStore.GetByName(name); p != nil {
		logrus.Debug("plugin already exists")
		return nil, nil, fmt.Errorf("%s exists", name)
	}

	pd, err := distribution.Pull(ref, pm.registryService, metaHeader, authConfig)
	if err != nil {
		logrus.Debugf("error in distribution.Pull(): %v", err)
		return nil, nil, err
	}
	return ref, pd, nil
}

func computePrivileges(pd distribution.PullData) (types.PluginPrivileges, error) {
	config, err := pd.Config()
	if err != nil {
		return nil, err
	}

	var c types.PluginConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, err
	}

	var privileges types.PluginPrivileges
	if c.Network.Type != "null" && c.Network.Type != "bridge" && c.Network.Type != "" {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "network",
			Description: "permissions to access a network",
			Value:       []string{c.Network.Type},
		})
	}
	for _, mount := range c.Mounts {
		if mount.Source != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "mount",
				Description: "host path to mount",
				Value:       []string{*mount.Source},
			})
		}
	}
	for _, device := range c.Linux.Devices {
		if device.Path != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "device",
				Description: "host device to access",
				Value:       []string{*device.Path},
			})
		}
	}
	if c.Linux.DeviceCreation {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "device-creation",
			Description: "allow creating devices inside plugin",
			Value:       []string{"true"},
		})
	}
	if len(c.Linux.Capabilities) > 0 {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "capabilities",
			Description: "list of additional capabilities required",
			Value:       c.Linux.Capabilities,
		})
	}

	return privileges, nil
}

// Privileges pulls a plugin config and computes the privileges required to install it.
func (pm *Manager) Privileges(name string, metaHeader http.Header, authConfig *types.AuthConfig) (types.PluginPrivileges, error) {
	_, pd, err := pm.pull(name, metaHeader, authConfig)
	if err != nil {
		return nil, err
	}
	return computePrivileges(pd)
}

// Pull pulls a plugin, check if the correct privileges are provided and install the plugin.
func (pm *Manager) Pull(name string, metaHeader http.Header, authConfig *types.AuthConfig, privileges types.PluginPrivileges) (err error) {
	ref, pd, err := pm.pull(name, metaHeader, authConfig)
	if err != nil {
		return err
	}

	requiredPrivileges, err := computePrivileges(pd)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(privileges, requiredPrivileges) {
		return errors.New("incorrect privileges")
	}

	pluginID := stringid.GenerateNonCryptoID()
	pluginDir := filepath.Join(pm.libRoot, pluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		logrus.Debugf("error in MkdirAll: %v", err)
		return err
	}

	defer func() {
		if err != nil {
			if delErr := os.RemoveAll(pluginDir); delErr != nil {
				logrus.Warnf("unable to remove %q from failed plugin pull: %v", pluginDir, delErr)
			}
		}
	}()

	err = distribution.WritePullData(pd, filepath.Join(pm.libRoot, pluginID), true)
	if err != nil {
		logrus.Debugf("error in distribution.WritePullData(): %v", err)
		return err
	}

	tag := distribution.GetTag(ref)
	p := v2.NewPlugin(ref.Name(), pluginID, pm.runRoot, pm.libRoot, tag)
	err = p.InitPlugin()
	if err != nil {
		return err
	}
	pm.pluginStore.Add(p)

	pm.pluginEventLogger(pluginID, ref.String(), "pull")

	return nil
}

// List displays the list of plugins and associated metadata.
func (pm *Manager) List() ([]types.Plugin, error) {
	plugins := pm.pluginStore.GetAll()
	out := make([]types.Plugin, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, p.PluginObj)
	}
	return out, nil
}

// Push pushes a plugin to the store.
func (pm *Manager) Push(name string, metaHeader http.Header, authConfig *types.AuthConfig) error {
	p, err := pm.pluginStore.GetByName(name)
	if err != nil {
		return err
	}
	dest := filepath.Join(pm.libRoot, p.GetID())
	config, err := ioutil.ReadFile(filepath.Join(dest, "config.json"))
	if err != nil {
		return err
	}

	var dummy types.Plugin
	err = json.Unmarshal(config, &dummy)
	if err != nil {
		return err
	}

	rootfs, err := archive.Tar(p.Rootfs, archive.Gzip)
	if err != nil {
		return err
	}
	defer rootfs.Close()

	_, err = distribution.Push(name, pm.registryService, metaHeader, authConfig, ioutil.NopCloser(bytes.NewReader(config)), rootfs)
	// XXX: Ignore returning digest for now.
	// Since digest needs to be written to the ProgressWriter.
	return err
}

// Remove deletes plugin's root directory.
func (pm *Manager) Remove(name string, config *types.PluginRmConfig) (err error) {
	p, err := pm.pluginStore.GetByName(name)
	pm.mu.RLock()
	c := pm.cMap[p]
	pm.mu.RUnlock()

	if err != nil {
		return err
	}

	if !config.ForceRemove {
		if p.GetRefCount() > 0 {
			return fmt.Errorf("plugin %s is in use", p.Name())
		}
		if p.IsEnabled() {
			return fmt.Errorf("plugin %s is enabled", p.Name())
		}
	}

	if p.IsEnabled() {
		if err := pm.disable(p, c); err != nil {
			logrus.Errorf("failed to disable plugin '%s': %s", p.Name(), err)
		}
	}

	id := p.GetID()
	pluginDir := filepath.Join(pm.libRoot, id)

	defer func() {
		if err == nil || config.ForceRemove {
			pm.pluginStore.Remove(p)
			pm.pluginEventLogger(id, name, "remove")
		}
	}()

	if err = os.RemoveAll(pluginDir); err != nil {
		return errors.Wrap(err, "failed to remove plugin directory")
	}
	return nil
}

// Set sets plugin args
func (pm *Manager) Set(name string, args []string) error {
	p, err := pm.pluginStore.GetByName(name)
	if err != nil {
		return err
	}
	return p.Set(args)
}

// CreateFromContext creates a plugin from the given pluginDir which contains
// both the rootfs and the config.json and a repoName with optional tag.
func (pm *Manager) CreateFromContext(ctx context.Context, tarCtx io.Reader, options *types.PluginCreateOptions) error {
	pluginID := stringid.GenerateNonCryptoID()

	pluginDir := filepath.Join(pm.libRoot, pluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}

	// In case an error happens, remove the created directory.
	if err := pm.createFromContext(ctx, pluginID, pluginDir, tarCtx, options); err != nil {
		if err := os.RemoveAll(pluginDir); err != nil {
			logrus.Warnf("unable to remove %q from failed plugin creation: %v", pluginDir, err)
		}
		return err
	}

	return nil
}

func (pm *Manager) createFromContext(ctx context.Context, pluginID, pluginDir string, tarCtx io.Reader, options *types.PluginCreateOptions) error {
	if err := chrootarchive.Untar(tarCtx, pluginDir, nil); err != nil {
		return err
	}

	repoName := options.RepoName
	ref, err := distribution.GetRef(repoName)
	if err != nil {
		return err
	}
	name := ref.Name()
	tag := distribution.GetTag(ref)

	p := v2.NewPlugin(name, pluginID, pm.runRoot, pm.libRoot, tag)
	if err := p.InitPlugin(); err != nil {
		return err
	}

	if err := pm.pluginStore.Add(p); err != nil {
		return err
	}

	pm.pluginEventLogger(p.GetID(), repoName, "create")

	return nil
}

func getPluginName(name string) (string, error) {
	named, err := reference.ParseNamed(name) // FIXME: validate
	if err != nil {
		return "", err
	}
	if reference.IsNameOnly(named) {
		named = reference.WithDefaultTag(named)
	}
	ref, ok := named.(reference.NamedTagged)
	if !ok {
		return "", fmt.Errorf("invalid name: %s", named.String())
	}
	return ref.String(), nil
}
