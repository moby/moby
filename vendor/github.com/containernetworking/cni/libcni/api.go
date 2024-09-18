// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package libcni

// Note this is the actual implementation of the CNI specification, which
// is reflected in the SPEC.md file.
// it is typically bundled into runtime providers (i.e. containerd or cri-o would use this
// before calling runc or hcsshim).  It is also bundled into CNI providers as well, for example,
// to add an IP to a container, to parse the configuration of the CNI and so on.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/create"
	"github.com/containernetworking/cni/pkg/utils"
	"github.com/containernetworking/cni/pkg/version"
)

var (
	CacheDir = "/var/lib/cni"
	// slightly awkward wording to preserve anyone matching on error strings
	ErrorCheckNotSupp = fmt.Errorf("does not support the CHECK command")
)

const (
	CNICacheV1 = "cniCacheV1"
)

// A RuntimeConf holds the arguments to one invocation of a CNI plugin
// excepting the network configuration, with the nested exception that
// the `runtimeConfig` from the network configuration is included
// here.
type RuntimeConf struct {
	ContainerID string
	NetNS       string
	IfName      string
	Args        [][2]string
	// A dictionary of capability-specific data passed by the runtime
	// to plugins as top-level keys in the 'runtimeConfig' dictionary
	// of the plugin's stdin data.  libcni will ensure that only keys
	// in this map which match the capabilities of the plugin are passed
	// to the plugin
	CapabilityArgs map[string]interface{}

	// DEPRECATED. Will be removed in a future release.
	CacheDir string
}

type NetworkConfig struct {
	Network *types.NetConf
	Bytes   []byte
}

type NetworkConfigList struct {
	Name         string
	CNIVersion   string
	DisableCheck bool
	DisableGC    bool
	Plugins      []*NetworkConfig
	Bytes        []byte
}

type NetworkAttachment struct {
	ContainerID    string
	Network        string
	IfName         string
	Config         []byte
	NetNS          string
	CniArgs        [][2]string
	CapabilityArgs map[string]interface{}
}

type GCArgs struct {
	ValidAttachments []types.GCAttachment
}

type CNI interface {
	AddNetworkList(ctx context.Context, net *NetworkConfigList, rt *RuntimeConf) (types.Result, error)
	CheckNetworkList(ctx context.Context, net *NetworkConfigList, rt *RuntimeConf) error
	DelNetworkList(ctx context.Context, net *NetworkConfigList, rt *RuntimeConf) error
	GetNetworkListCachedResult(net *NetworkConfigList, rt *RuntimeConf) (types.Result, error)
	GetNetworkListCachedConfig(net *NetworkConfigList, rt *RuntimeConf) ([]byte, *RuntimeConf, error)

	AddNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) (types.Result, error)
	CheckNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) error
	DelNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) error
	GetNetworkCachedResult(net *NetworkConfig, rt *RuntimeConf) (types.Result, error)
	GetNetworkCachedConfig(net *NetworkConfig, rt *RuntimeConf) ([]byte, *RuntimeConf, error)

	ValidateNetworkList(ctx context.Context, net *NetworkConfigList) ([]string, error)
	ValidateNetwork(ctx context.Context, net *NetworkConfig) ([]string, error)

	GCNetworkList(ctx context.Context, net *NetworkConfigList, args *GCArgs) error
	GetStatusNetworkList(ctx context.Context, net *NetworkConfigList) error

	GetCachedAttachments(containerID string) ([]*NetworkAttachment, error)

	GetVersionInfo(ctx context.Context, pluginType string) (version.PluginInfo, error)
}

type CNIConfig struct {
	Path     []string
	exec     invoke.Exec
	cacheDir string
}

// CNIConfig implements the CNI interface
var _ CNI = &CNIConfig{}

// NewCNIConfig returns a new CNIConfig object that will search for plugins
// in the given paths and use the given exec interface to run those plugins,
// or if the exec interface is not given, will use a default exec handler.
func NewCNIConfig(path []string, exec invoke.Exec) *CNIConfig {
	return NewCNIConfigWithCacheDir(path, "", exec)
}

// NewCNIConfigWithCacheDir returns a new CNIConfig object that will search for plugins
// in the given paths use the given exec interface to run those plugins,
// or if the exec interface is not given, will use a default exec handler.
// The given cache directory will be used for temporary data storage when needed.
func NewCNIConfigWithCacheDir(path []string, cacheDir string, exec invoke.Exec) *CNIConfig {
	return &CNIConfig{
		Path:     path,
		cacheDir: cacheDir,
		exec:     exec,
	}
}

func buildOneConfig(name, cniVersion string, orig *NetworkConfig, prevResult types.Result, rt *RuntimeConf) (*NetworkConfig, error) {
	var err error

	inject := map[string]interface{}{
		"name":       name,
		"cniVersion": cniVersion,
	}
	// Add previous plugin result
	if prevResult != nil {
		inject["prevResult"] = prevResult
	}

	// Ensure every config uses the same name and version
	orig, err = InjectConf(orig, inject)
	if err != nil {
		return nil, err
	}
	if rt != nil {
		return injectRuntimeConfig(orig, rt)
	}

	return orig, nil
}

// This function takes a libcni RuntimeConf structure and injects values into
// a "runtimeConfig" dictionary in the CNI network configuration JSON that
// will be passed to the plugin on stdin.
//
// Only "capabilities arguments" passed by the runtime are currently injected.
// These capabilities arguments are filtered through the plugin's advertised
// capabilities from its config JSON, and any keys in the CapabilityArgs
// matching plugin capabilities are added to the "runtimeConfig" dictionary
// sent to the plugin via JSON on stdin.  For example, if the plugin's
// capabilities include "portMappings", and the CapabilityArgs map includes a
// "portMappings" key, that key and its value are added to the "runtimeConfig"
// dictionary to be passed to the plugin's stdin.
func injectRuntimeConfig(orig *NetworkConfig, rt *RuntimeConf) (*NetworkConfig, error) {
	var err error

	rc := make(map[string]interface{})
	for capability, supported := range orig.Network.Capabilities {
		if !supported {
			continue
		}
		if data, ok := rt.CapabilityArgs[capability]; ok {
			rc[capability] = data
		}
	}

	if len(rc) > 0 {
		orig, err = InjectConf(orig, map[string]interface{}{"runtimeConfig": rc})
		if err != nil {
			return nil, err
		}
	}

	return orig, nil
}

// ensure we have a usable exec if the CNIConfig was not given one
func (c *CNIConfig) ensureExec() invoke.Exec {
	if c.exec == nil {
		c.exec = &invoke.DefaultExec{
			RawExec:       &invoke.RawExec{Stderr: os.Stderr},
			PluginDecoder: version.PluginDecoder{},
		}
	}
	return c.exec
}

type cachedInfo struct {
	Kind           string                 `json:"kind"`
	ContainerID    string                 `json:"containerId"`
	Config         []byte                 `json:"config"`
	IfName         string                 `json:"ifName"`
	NetworkName    string                 `json:"networkName"`
	NetNS          string                 `json:"netns,omitempty"`
	CniArgs        [][2]string            `json:"cniArgs,omitempty"`
	CapabilityArgs map[string]interface{} `json:"capabilityArgs,omitempty"`
	RawResult      map[string]interface{} `json:"result,omitempty"`
	Result         types.Result           `json:"-"`
}

// getCacheDir returns the cache directory in this order:
// 1) global cacheDir from CNIConfig object
// 2) deprecated cacheDir from RuntimeConf object
// 3) fall back to default cache directory
func (c *CNIConfig) getCacheDir(rt *RuntimeConf) string {
	if c.cacheDir != "" {
		return c.cacheDir
	}
	if rt.CacheDir != "" {
		return rt.CacheDir
	}
	return CacheDir
}

func (c *CNIConfig) getCacheFilePath(netName string, rt *RuntimeConf) (string, error) {
	if netName == "" || rt.ContainerID == "" || rt.IfName == "" {
		return "", fmt.Errorf("cache file path requires network name (%q), container ID (%q), and interface name (%q)", netName, rt.ContainerID, rt.IfName)
	}
	return filepath.Join(c.getCacheDir(rt), "results", fmt.Sprintf("%s-%s-%s", netName, rt.ContainerID, rt.IfName)), nil
}

func (c *CNIConfig) cacheAdd(result types.Result, config []byte, netName string, rt *RuntimeConf) error {
	cached := cachedInfo{
		Kind:           CNICacheV1,
		ContainerID:    rt.ContainerID,
		Config:         config,
		IfName:         rt.IfName,
		NetworkName:    netName,
		NetNS:          rt.NetNS,
		CniArgs:        rt.Args,
		CapabilityArgs: rt.CapabilityArgs,
	}

	// We need to get type.Result into cachedInfo as JSON map
	// Marshal to []byte, then Unmarshal into cached.RawResult
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &cached.RawResult)
	if err != nil {
		return err
	}

	newBytes, err := json.Marshal(&cached)
	if err != nil {
		return err
	}

	fname, err := c.getCacheFilePath(netName, rt)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0o700); err != nil {
		return err
	}

	return os.WriteFile(fname, newBytes, 0o600)
}

func (c *CNIConfig) cacheDel(netName string, rt *RuntimeConf) error {
	fname, err := c.getCacheFilePath(netName, rt)
	if err != nil {
		// Ignore error
		return nil
	}
	return os.Remove(fname)
}

func (c *CNIConfig) getCachedConfig(netName string, rt *RuntimeConf) ([]byte, *RuntimeConf, error) {
	var bytes []byte

	fname, err := c.getCacheFilePath(netName, rt)
	if err != nil {
		return nil, nil, err
	}
	bytes, err = os.ReadFile(fname)
	if err != nil {
		// Ignore read errors; the cached result may not exist on-disk
		return nil, nil, nil
	}

	unmarshaled := cachedInfo{}
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal cached network %q config: %w", netName, err)
	}
	if unmarshaled.Kind != CNICacheV1 {
		return nil, nil, fmt.Errorf("read cached network %q config has wrong kind: %v", netName, unmarshaled.Kind)
	}

	newRt := *rt
	if unmarshaled.CniArgs != nil {
		newRt.Args = unmarshaled.CniArgs
	}
	newRt.CapabilityArgs = unmarshaled.CapabilityArgs

	return unmarshaled.Config, &newRt, nil
}

func (c *CNIConfig) getLegacyCachedResult(netName, cniVersion string, rt *RuntimeConf) (types.Result, error) {
	fname, err := c.getCacheFilePath(netName, rt)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fname)
	if err != nil {
		// Ignore read errors; the cached result may not exist on-disk
		return nil, nil
	}

	// Load the cached result
	result, err := create.CreateFromBytes(data)
	if err != nil {
		return nil, err
	}

	// Convert to the config version to ensure plugins get prevResult
	// in the same version as the config.  The cached result version
	// should match the config version unless the config was changed
	// while the container was running.
	result, err = result.GetAsVersion(cniVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cached result to config version %q: %w", cniVersion, err)
	}
	return result, nil
}

func (c *CNIConfig) getCachedResult(netName, cniVersion string, rt *RuntimeConf) (types.Result, error) {
	fname, err := c.getCacheFilePath(netName, rt)
	if err != nil {
		return nil, err
	}
	fdata, err := os.ReadFile(fname)
	if err != nil {
		// Ignore read errors; the cached result may not exist on-disk
		return nil, nil
	}

	cachedInfo := cachedInfo{}
	if err := json.Unmarshal(fdata, &cachedInfo); err != nil || cachedInfo.Kind != CNICacheV1 {
		return c.getLegacyCachedResult(netName, cniVersion, rt)
	}

	newBytes, err := json.Marshal(&cachedInfo.RawResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cached network %q config: %w", netName, err)
	}

	// Load the cached result
	result, err := create.CreateFromBytes(newBytes)
	if err != nil {
		return nil, err
	}

	// Convert to the config version to ensure plugins get prevResult
	// in the same version as the config.  The cached result version
	// should match the config version unless the config was changed
	// while the container was running.
	result, err = result.GetAsVersion(cniVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cached result to config version %q: %w", cniVersion, err)
	}
	return result, nil
}

// GetNetworkListCachedResult returns the cached Result of the previous
// AddNetworkList() operation for a network list, or an error.
func (c *CNIConfig) GetNetworkListCachedResult(list *NetworkConfigList, rt *RuntimeConf) (types.Result, error) {
	return c.getCachedResult(list.Name, list.CNIVersion, rt)
}

// GetNetworkCachedResult returns the cached Result of the previous
// AddNetwork() operation for a network, or an error.
func (c *CNIConfig) GetNetworkCachedResult(net *NetworkConfig, rt *RuntimeConf) (types.Result, error) {
	return c.getCachedResult(net.Network.Name, net.Network.CNIVersion, rt)
}

// GetNetworkListCachedConfig copies the input RuntimeConf to output
// RuntimeConf with fields updated with info from the cached Config.
func (c *CNIConfig) GetNetworkListCachedConfig(list *NetworkConfigList, rt *RuntimeConf) ([]byte, *RuntimeConf, error) {
	return c.getCachedConfig(list.Name, rt)
}

// GetNetworkCachedConfig copies the input RuntimeConf to output
// RuntimeConf with fields updated with info from the cached Config.
func (c *CNIConfig) GetNetworkCachedConfig(net *NetworkConfig, rt *RuntimeConf) ([]byte, *RuntimeConf, error) {
	return c.getCachedConfig(net.Network.Name, rt)
}

// GetCachedAttachments returns a list of network attachments from the cache.
// The returned list will be filtered by the containerID if the value is not empty.
func (c *CNIConfig) GetCachedAttachments(containerID string) ([]*NetworkAttachment, error) {
	dirPath := filepath.Join(c.getCacheDir(&RuntimeConf{}), "results")
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	fileNames := make([]string, 0, len(entries))
	for _, e := range entries {
		fileNames = append(fileNames, e.Name())
	}
	sort.Strings(fileNames)

	attachments := []*NetworkAttachment{}
	for _, fname := range fileNames {
		if len(containerID) > 0 {
			part := fmt.Sprintf("-%s-", containerID)
			pos := strings.Index(fname, part)
			if pos <= 0 || pos+len(part) >= len(fname) {
				continue
			}
		}

		cacheFile := filepath.Join(dirPath, fname)
		bytes, err := os.ReadFile(cacheFile)
		if err != nil {
			continue
		}

		cachedInfo := cachedInfo{}

		if err := json.Unmarshal(bytes, &cachedInfo); err != nil {
			continue
		}
		if cachedInfo.Kind != CNICacheV1 {
			continue
		}
		if len(containerID) > 0 && cachedInfo.ContainerID != containerID {
			continue
		}
		if cachedInfo.IfName == "" || cachedInfo.NetworkName == "" {
			continue
		}

		attachments = append(attachments, &NetworkAttachment{
			ContainerID:    cachedInfo.ContainerID,
			Network:        cachedInfo.NetworkName,
			IfName:         cachedInfo.IfName,
			Config:         cachedInfo.Config,
			NetNS:          cachedInfo.NetNS,
			CniArgs:        cachedInfo.CniArgs,
			CapabilityArgs: cachedInfo.CapabilityArgs,
		})
	}
	return attachments, nil
}

func (c *CNIConfig) addNetwork(ctx context.Context, name, cniVersion string, net *NetworkConfig, prevResult types.Result, rt *RuntimeConf) (types.Result, error) {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(net.Network.Type, c.Path)
	if err != nil {
		return nil, err
	}
	if err := utils.ValidateContainerID(rt.ContainerID); err != nil {
		return nil, err
	}
	if err := utils.ValidateNetworkName(name); err != nil {
		return nil, err
	}
	if err := utils.ValidateInterfaceName(rt.IfName); err != nil {
		return nil, err
	}

	newConf, err := buildOneConfig(name, cniVersion, net, prevResult, rt)
	if err != nil {
		return nil, err
	}

	return invoke.ExecPluginWithResult(ctx, pluginPath, newConf.Bytes, c.args("ADD", rt), c.exec)
}

// AddNetworkList executes a sequence of plugins with the ADD command
func (c *CNIConfig) AddNetworkList(ctx context.Context, list *NetworkConfigList, rt *RuntimeConf) (types.Result, error) {
	var err error
	var result types.Result
	for _, net := range list.Plugins {
		result, err = c.addNetwork(ctx, list.Name, list.CNIVersion, net, result, rt)
		if err != nil {
			return nil, fmt.Errorf("plugin %s failed (add): %w", pluginDescription(net.Network), err)
		}
	}

	if err = c.cacheAdd(result, list.Bytes, list.Name, rt); err != nil {
		return nil, fmt.Errorf("failed to set network %q cached result: %w", list.Name, err)
	}

	return result, nil
}

func (c *CNIConfig) checkNetwork(ctx context.Context, name, cniVersion string, net *NetworkConfig, prevResult types.Result, rt *RuntimeConf) error {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(net.Network.Type, c.Path)
	if err != nil {
		return err
	}

	newConf, err := buildOneConfig(name, cniVersion, net, prevResult, rt)
	if err != nil {
		return err
	}

	return invoke.ExecPluginWithoutResult(ctx, pluginPath, newConf.Bytes, c.args("CHECK", rt), c.exec)
}

// CheckNetworkList executes a sequence of plugins with the CHECK command
func (c *CNIConfig) CheckNetworkList(ctx context.Context, list *NetworkConfigList, rt *RuntimeConf) error {
	// CHECK was added in CNI spec version 0.4.0 and higher
	if gtet, err := version.GreaterThanOrEqualTo(list.CNIVersion, "0.4.0"); err != nil {
		return err
	} else if !gtet {
		return fmt.Errorf("configuration version %q %w", list.CNIVersion, ErrorCheckNotSupp)
	}

	if list.DisableCheck {
		return nil
	}

	cachedResult, err := c.getCachedResult(list.Name, list.CNIVersion, rt)
	if err != nil {
		return fmt.Errorf("failed to get network %q cached result: %w", list.Name, err)
	}

	for _, net := range list.Plugins {
		if err := c.checkNetwork(ctx, list.Name, list.CNIVersion, net, cachedResult, rt); err != nil {
			return err
		}
	}

	return nil
}

func (c *CNIConfig) delNetwork(ctx context.Context, name, cniVersion string, net *NetworkConfig, prevResult types.Result, rt *RuntimeConf) error {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(net.Network.Type, c.Path)
	if err != nil {
		return err
	}

	newConf, err := buildOneConfig(name, cniVersion, net, prevResult, rt)
	if err != nil {
		return err
	}

	return invoke.ExecPluginWithoutResult(ctx, pluginPath, newConf.Bytes, c.args("DEL", rt), c.exec)
}

// DelNetworkList executes a sequence of plugins with the DEL command
func (c *CNIConfig) DelNetworkList(ctx context.Context, list *NetworkConfigList, rt *RuntimeConf) error {
	var cachedResult types.Result

	// Cached result on DEL was added in CNI spec version 0.4.0 and higher
	if gtet, err := version.GreaterThanOrEqualTo(list.CNIVersion, "0.4.0"); err != nil {
		return err
	} else if gtet {
		if cachedResult, err = c.getCachedResult(list.Name, list.CNIVersion, rt); err != nil {
			_ = c.cacheDel(list.Name, rt)
			cachedResult = nil
		}
	}

	for i := len(list.Plugins) - 1; i >= 0; i-- {
		net := list.Plugins[i]
		if err := c.delNetwork(ctx, list.Name, list.CNIVersion, net, cachedResult, rt); err != nil {
			return fmt.Errorf("plugin %s failed (delete): %w", pluginDescription(net.Network), err)
		}
	}

	_ = c.cacheDel(list.Name, rt)

	return nil
}

func pluginDescription(net *types.NetConf) string {
	if net == nil {
		return "<missing>"
	}
	pluginType := net.Type
	out := fmt.Sprintf("type=%q", pluginType)
	name := net.Name
	if name != "" {
		out += fmt.Sprintf(" name=%q", name)
	}
	return out
}

// AddNetwork executes the plugin with the ADD command
func (c *CNIConfig) AddNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) (types.Result, error) {
	result, err := c.addNetwork(ctx, net.Network.Name, net.Network.CNIVersion, net, nil, rt)
	if err != nil {
		return nil, err
	}

	if err = c.cacheAdd(result, net.Bytes, net.Network.Name, rt); err != nil {
		return nil, fmt.Errorf("failed to set network %q cached result: %w", net.Network.Name, err)
	}

	return result, nil
}

// CheckNetwork executes the plugin with the CHECK command
func (c *CNIConfig) CheckNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) error {
	// CHECK was added in CNI spec version 0.4.0 and higher
	if gtet, err := version.GreaterThanOrEqualTo(net.Network.CNIVersion, "0.4.0"); err != nil {
		return err
	} else if !gtet {
		return fmt.Errorf("configuration version %q %w", net.Network.CNIVersion, ErrorCheckNotSupp)
	}

	cachedResult, err := c.getCachedResult(net.Network.Name, net.Network.CNIVersion, rt)
	if err != nil {
		return fmt.Errorf("failed to get network %q cached result: %w", net.Network.Name, err)
	}
	return c.checkNetwork(ctx, net.Network.Name, net.Network.CNIVersion, net, cachedResult, rt)
}

// DelNetwork executes the plugin with the DEL command
func (c *CNIConfig) DelNetwork(ctx context.Context, net *NetworkConfig, rt *RuntimeConf) error {
	var cachedResult types.Result

	// Cached result on DEL was added in CNI spec version 0.4.0 and higher
	if gtet, err := version.GreaterThanOrEqualTo(net.Network.CNIVersion, "0.4.0"); err != nil {
		return err
	} else if gtet {
		cachedResult, err = c.getCachedResult(net.Network.Name, net.Network.CNIVersion, rt)
		if err != nil {
			return fmt.Errorf("failed to get network %q cached result: %w", net.Network.Name, err)
		}
	}

	if err := c.delNetwork(ctx, net.Network.Name, net.Network.CNIVersion, net, cachedResult, rt); err != nil {
		return err
	}
	_ = c.cacheDel(net.Network.Name, rt)
	return nil
}

// ValidateNetworkList checks that a configuration is reasonably valid.
// - all the specified plugins exist on disk
// - every plugin supports the desired version.
//
// Returns a list of all capabilities supported by the configuration, or error
func (c *CNIConfig) ValidateNetworkList(ctx context.Context, list *NetworkConfigList) ([]string, error) {
	version := list.CNIVersion

	// holding map for seen caps (in case of duplicates)
	caps := map[string]interface{}{}

	errs := []error{}
	for _, net := range list.Plugins {
		if err := c.validatePlugin(ctx, net.Network.Type, version); err != nil {
			errs = append(errs, err)
		}
		for c, enabled := range net.Network.Capabilities {
			if !enabled {
				continue
			}
			caps[c] = struct{}{}
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}

	// make caps list
	cc := make([]string, 0, len(caps))
	for c := range caps {
		cc = append(cc, c)
	}

	return cc, nil
}

// ValidateNetwork checks that a configuration is reasonably valid.
// It uses the same logic as ValidateNetworkList)
// Returns a list of capabilities
func (c *CNIConfig) ValidateNetwork(ctx context.Context, net *NetworkConfig) ([]string, error) {
	caps := []string{}
	for c, ok := range net.Network.Capabilities {
		if ok {
			caps = append(caps, c)
		}
	}
	if err := c.validatePlugin(ctx, net.Network.Type, net.Network.CNIVersion); err != nil {
		return nil, err
	}
	return caps, nil
}

// validatePlugin checks that an individual plugin's configuration is sane
func (c *CNIConfig) validatePlugin(ctx context.Context, pluginName, expectedVersion string) error {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(pluginName, c.Path)
	if err != nil {
		return err
	}
	if expectedVersion == "" {
		expectedVersion = "0.1.0"
	}

	vi, err := invoke.GetVersionInfo(ctx, pluginPath, c.exec)
	if err != nil {
		return err
	}
	for _, vers := range vi.SupportedVersions() {
		if vers == expectedVersion {
			return nil
		}
	}
	return fmt.Errorf("plugin %s does not support config version %q", pluginName, expectedVersion)
}

// GetVersionInfo reports which versions of the CNI spec are supported by
// the given plugin.
func (c *CNIConfig) GetVersionInfo(ctx context.Context, pluginType string) (version.PluginInfo, error) {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(pluginType, c.Path)
	if err != nil {
		return nil, err
	}

	return invoke.GetVersionInfo(ctx, pluginPath, c.exec)
}

// GCNetworkList will do two things
// - dump the list of cached attachments, and issue deletes as necessary
// - issue a GC to the underlying plugins (if the version is high enough)
func (c *CNIConfig) GCNetworkList(ctx context.Context, list *NetworkConfigList, args *GCArgs) error {
	// If DisableGC is set, then don't bother GCing at all.
	if list.DisableGC {
		return nil
	}

	// First, get the list of cached attachments
	cachedAttachments, err := c.GetCachedAttachments("")
	if err != nil {
		return nil
	}

	var validAttachments map[types.GCAttachment]interface{}
	if args != nil {
		validAttachments = make(map[types.GCAttachment]interface{}, len(args.ValidAttachments))
		for _, a := range args.ValidAttachments {
			validAttachments[a] = nil
		}
	}

	var errs []error

	for _, cachedAttachment := range cachedAttachments {
		if cachedAttachment.Network != list.Name {
			continue
		}
		// we found this attachment
		gca := types.GCAttachment{
			ContainerID: cachedAttachment.ContainerID,
			IfName:      cachedAttachment.IfName,
		}
		if _, ok := validAttachments[gca]; ok {
			continue
		}
		// otherwise, this attachment wasn't valid and we should issue a CNI DEL
		rt := RuntimeConf{
			ContainerID:    cachedAttachment.ContainerID,
			NetNS:          cachedAttachment.NetNS,
			IfName:         cachedAttachment.IfName,
			Args:           cachedAttachment.CniArgs,
			CapabilityArgs: cachedAttachment.CapabilityArgs,
		}
		if err := c.DelNetworkList(ctx, list, &rt); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete stale attachment %s %s: %w", rt.ContainerID, rt.IfName, err))
		}
	}

	// now, if the version supports it, issue a GC
	if gt, _ := version.GreaterThanOrEqualTo(list.CNIVersion, "1.1.0"); gt {
		inject := map[string]interface{}{
			"name":       list.Name,
			"cniVersion": list.CNIVersion,
		}
		if args != nil {
			inject["cni.dev/valid-attachments"] = args.ValidAttachments
		}

		for _, plugin := range list.Plugins {
			// build config here
			pluginConfig, err := InjectConf(plugin, inject)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to generate configuration to GC plugin %s: %w", plugin.Network.Type, err))
			}
			if err := c.gcNetwork(ctx, pluginConfig); err != nil {
				errs = append(errs, fmt.Errorf("failed to GC plugin %s: %w", plugin.Network.Type, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (c *CNIConfig) gcNetwork(ctx context.Context, net *NetworkConfig) error {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(net.Network.Type, c.Path)
	if err != nil {
		return err
	}
	args := c.args("GC", &RuntimeConf{})

	return invoke.ExecPluginWithoutResult(ctx, pluginPath, net.Bytes, args, c.exec)
}

func (c *CNIConfig) GetStatusNetworkList(ctx context.Context, list *NetworkConfigList) error {
	// If the version doesn't support status, abort.
	if gt, _ := version.GreaterThanOrEqualTo(list.CNIVersion, "1.1.0"); !gt {
		return nil
	}

	inject := map[string]interface{}{
		"name":       list.Name,
		"cniVersion": list.CNIVersion,
	}

	for _, plugin := range list.Plugins {
		// build config here
		pluginConfig, err := InjectConf(plugin, inject)
		if err != nil {
			return fmt.Errorf("failed to generate configuration to get plugin STATUS %s: %w", plugin.Network.Type, err)
		}
		if err := c.getStatusNetwork(ctx, pluginConfig); err != nil {
			return err // Don't collect errors here, so we return a clean error code.
		}
	}
	return nil
}

func (c *CNIConfig) getStatusNetwork(ctx context.Context, net *NetworkConfig) error {
	c.ensureExec()
	pluginPath, err := c.exec.FindInPath(net.Network.Type, c.Path)
	if err != nil {
		return err
	}
	args := c.args("STATUS", &RuntimeConf{})

	return invoke.ExecPluginWithoutResult(ctx, pluginPath, net.Bytes, args, c.exec)
}

// =====
func (c *CNIConfig) args(action string, rt *RuntimeConf) *invoke.Args {
	return &invoke.Args{
		Command:     action,
		ContainerID: rt.ContainerID,
		NetNS:       rt.NetNS,
		PluginArgs:  rt.Args,
		IfName:      rt.IfName,
		Path:        strings.Join(c.Path, string(os.PathListSeparator)),
	}
}
