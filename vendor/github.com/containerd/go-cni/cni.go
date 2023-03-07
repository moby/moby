/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cni

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	cnilibrary "github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
)

type CNI interface {
	// Setup setup the network for the namespace
	Setup(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error)
	// SetupSerially sets up each of the network interfaces for the namespace in serial
	SetupSerially(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error)
	// Remove tears down the network of the namespace.
	Remove(ctx context.Context, id string, path string, opts ...NamespaceOpts) error
	// Check checks if the network is still in desired state
	Check(ctx context.Context, id string, path string, opts ...NamespaceOpts) error
	// Load loads the cni network config
	Load(opts ...Opt) error
	// Status checks the status of the cni initialization
	Status() error
	// GetConfig returns a copy of the CNI plugin configurations as parsed by CNI
	GetConfig() *ConfigResult
}

type ConfigResult struct {
	PluginDirs       []string
	PluginConfDir    string
	PluginMaxConfNum int
	Prefix           string
	Networks         []*ConfNetwork
}

type ConfNetwork struct {
	Config *NetworkConfList
	IFName string
}

// NetworkConfList is a source bytes to string version of cnilibrary.NetworkConfigList
type NetworkConfList struct {
	Name       string
	CNIVersion string
	Plugins    []*NetworkConf
	Source     string
}

// NetworkConf is a source bytes to string conversion of cnilibrary.NetworkConfig
type NetworkConf struct {
	Network *types.NetConf
	Source  string
}

type libcni struct {
	config

	cniConfig    cnilibrary.CNI
	networkCount int // minimum network plugin configurations needed to initialize cni
	networks     []*Network
	sync.RWMutex
}

func defaultCNIConfig() *libcni {
	return &libcni{
		config: config{
			pluginDirs:       []string{DefaultCNIDir},
			pluginConfDir:    DefaultNetDir,
			pluginMaxConfNum: DefaultMaxConfNum,
			prefix:           DefaultPrefix,
		},
		cniConfig: cnilibrary.NewCNIConfig(
			[]string{
				DefaultCNIDir,
			},
			&invoke.DefaultExec{
				RawExec:       &invoke.RawExec{Stderr: os.Stderr},
				PluginDecoder: version.PluginDecoder{},
			},
		),
		networkCount: 1,
	}
}

// New creates a new libcni instance.
func New(config ...Opt) (CNI, error) {
	cni := defaultCNIConfig()
	var err error
	for _, c := range config {
		if err = c(cni); err != nil {
			return nil, err
		}
	}
	return cni, nil
}

// Load loads the latest config from cni config files.
func (c *libcni) Load(opts ...Opt) error {
	var err error
	c.Lock()
	defer c.Unlock()
	// Reset the networks on a load operation to ensure
	// config happens on a clean slate
	c.reset()

	for _, o := range opts {
		if err = o(c); err != nil {
			return fmt.Errorf("cni config load failed: %v: %w", err, ErrLoad)
		}
	}
	return nil
}

// Status returns the status of CNI initialization.
func (c *libcni) Status() error {
	c.RLock()
	defer c.RUnlock()
	if len(c.networks) < c.networkCount {
		return ErrCNINotInitialized
	}
	return nil
}

// Networks returns all the configured networks.
// NOTE: Caller MUST NOT modify anything in the returned array.
func (c *libcni) Networks() []*Network {
	c.RLock()
	defer c.RUnlock()
	return append([]*Network{}, c.networks...)
}

// Setup setups the network in the namespace and returns a Result
func (c *libcni) Setup(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error) {
	if err := c.Status(); err != nil {
		return nil, err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return nil, err
	}
	result, err := c.attachNetworks(ctx, ns)
	if err != nil {
		return nil, err
	}
	return c.createResult(result)
}

// SetupSerially setups the network in the namespace and returns a Result
func (c *libcni) SetupSerially(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error) {
	if err := c.Status(); err != nil {
		return nil, err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return nil, err
	}
	result, err := c.attachNetworksSerially(ctx, ns)
	if err != nil {
		return nil, err
	}
	return c.createResult(result)
}

func (c *libcni) attachNetworksSerially(ctx context.Context, ns *Namespace) ([]*types100.Result, error) {
	var results []*types100.Result
	for _, network := range c.Networks() {
		r, err := network.Attach(ctx, ns)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

type asynchAttachResult struct {
	index int
	res   *types100.Result
	err   error
}

func asynchAttach(ctx context.Context, index int, n *Network, ns *Namespace, wg *sync.WaitGroup, rc chan asynchAttachResult) {
	defer wg.Done()
	r, err := n.Attach(ctx, ns)
	rc <- asynchAttachResult{index: index, res: r, err: err}
}

func (c *libcni) attachNetworks(ctx context.Context, ns *Namespace) ([]*types100.Result, error) {
	var wg sync.WaitGroup
	var firstError error
	results := make([]*types100.Result, len(c.Networks()))
	rc := make(chan asynchAttachResult)

	for i, network := range c.Networks() {
		wg.Add(1)
		go asynchAttach(ctx, i, network, ns, &wg, rc)
	}

	for range c.Networks() {
		rs := <-rc
		if rs.err != nil && firstError == nil {
			firstError = rs.err
		}
		results[rs.index] = rs.res
	}
	wg.Wait()

	return results, firstError
}

// Remove removes the network config from the namespace
func (c *libcni) Remove(ctx context.Context, id string, path string, opts ...NamespaceOpts) error {
	if err := c.Status(); err != nil {
		return err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return err
	}
	for _, network := range c.Networks() {
		if err := network.Remove(ctx, ns); err != nil {
			// Based on CNI spec v0.7.0, empty network namespace is allowed to
			// do best effort cleanup. However, it is not handled consistently
			// right now:
			// https://github.com/containernetworking/plugins/issues/210
			// TODO(random-liu): Remove the error handling when the issue is
			// fixed and the CNI spec v0.6.0 support is deprecated.
			// NOTE(claudiub): Some CNIs could return a "not found" error, which could mean that
			// it was already deleted.
			if (path == "" && strings.Contains(err.Error(), "no such file or directory")) || strings.Contains(err.Error(), "not found") {
				continue
			}
			return err
		}
	}
	return nil
}

// Check checks if the network is still in desired state
func (c *libcni) Check(ctx context.Context, id string, path string, opts ...NamespaceOpts) error {
	if err := c.Status(); err != nil {
		return err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return err
	}
	for _, network := range c.Networks() {
		err := network.Check(ctx, ns)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetConfig returns a copy of the CNI plugin configurations as parsed by CNI
func (c *libcni) GetConfig() *ConfigResult {
	c.RLock()
	defer c.RUnlock()
	r := &ConfigResult{
		PluginDirs:       c.config.pluginDirs,
		PluginConfDir:    c.config.pluginConfDir,
		PluginMaxConfNum: c.config.pluginMaxConfNum,
		Prefix:           c.config.prefix,
	}
	for _, network := range c.networks {
		conf := &NetworkConfList{
			Name:       network.config.Name,
			CNIVersion: network.config.CNIVersion,
			Source:     string(network.config.Bytes),
		}
		for _, plugin := range network.config.Plugins {
			conf.Plugins = append(conf.Plugins, &NetworkConf{
				Network: plugin.Network,
				Source:  string(plugin.Bytes),
			})
		}
		r.Networks = append(r.Networks, &ConfNetwork{
			Config: conf,
			IFName: network.ifName,
		})
	}
	return r
}

func (c *libcni) reset() {
	c.networks = nil
}
