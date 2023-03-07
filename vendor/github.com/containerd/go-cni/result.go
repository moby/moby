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
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
)

type IPConfig struct {
	IP      net.IP
	Gateway net.IP
}

// Result contains the network information returned by CNI.Setup
//
// a) Interfaces list. Depending on the plugin, this can include the sandbox
//    (eg, container or hypervisor) interface name and/or the host interface
//    name, the hardware addresses of each interface, and details about the
//    sandbox (if any) the interface is in.
// b) IP configuration assigned to each  interface. The IPv4 and/or IPv6 addresses,
//    gateways, and routes assigned to sandbox and/or host interfaces.
// c) DNS information. Dictionary that includes DNS information for nameservers,
//     domain, search domains and options.
type Result struct {
	Interfaces map[string]*Config
	DNS        []types.DNS
	Routes     []*types.Route
	raw        []*types100.Result
}

// Raw returns the raw CNI results of multiple networks.
func (r *Result) Raw() []*types100.Result {
	return r.raw
}

type Config struct {
	IPConfigs []*IPConfig
	Mac       string
	Sandbox   string
}

// createResult creates a Result from the given slice of types100.Result, adding
// structured data containing the interface configuration for each of the
// interfaces created in the namespace. It returns an error if validation of
// results fails, or if a network could not be found.
func (c *libcni) createResult(results []*types100.Result) (*Result, error) {
	c.RLock()
	defer c.RUnlock()
	r := &Result{
		Interfaces: make(map[string]*Config),
		raw:        results,
	}

	// Plugins may not need to return Interfaces in result if
	// if there are no multiple interfaces created. In that case
	// all configs should be applied against default interface
	r.Interfaces[defaultInterface(c.prefix)] = &Config{}

	// Walk through all the results
	for _, result := range results {
		// Walk through all the interface in each result
		for _, intf := range result.Interfaces {
			r.Interfaces[intf.Name] = &Config{
				Mac:     intf.Mac,
				Sandbox: intf.Sandbox,
			}
		}
		// Walk through all the IPs in the result and attach it to corresponding
		// interfaces
		for _, ipConf := range result.IPs {
			if err := validateInterfaceConfig(ipConf, len(result.Interfaces)); err != nil {
				return nil, fmt.Errorf("invalid interface config: %v: %w", err, ErrInvalidResult)
			}
			name := c.getInterfaceName(result.Interfaces, ipConf)
			r.Interfaces[name].IPConfigs = append(r.Interfaces[name].IPConfigs,
				&IPConfig{IP: ipConf.Address.IP, Gateway: ipConf.Gateway})
		}
		r.DNS = append(r.DNS, result.DNS)
		r.Routes = append(r.Routes, result.Routes...)
	}
	if _, ok := r.Interfaces[defaultInterface(c.prefix)]; !ok {
		return nil, fmt.Errorf("default network not found for: %s: %w", defaultInterface(c.prefix), ErrNotFound)
	}
	return r, nil
}

// getInterfaceName returns the interface name if the plugins
// return the result with associated interfaces. If interface
// is not present then default interface name is used
func (c *libcni) getInterfaceName(interfaces []*types100.Interface,
	ipConf *types100.IPConfig) string {
	if ipConf.Interface != nil {
		return interfaces[*ipConf.Interface].Name
	}
	return defaultInterface(c.prefix)
}
