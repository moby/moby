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
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/pkg/errors"
)

type IPConfig struct {
	IP      net.IP
	Gateway net.IP
}

type CNIResult struct {
	Interfaces map[string]*Config
	DNS        []types.DNS
	Routes     []*types.Route
}

type Config struct {
	IPConfigs []*IPConfig
	Mac       string
	Sandbox   string
}

// GetCNIResultFromResults returns a structured data containing the
// interface configuration for each of the interfaces created in the namespace.
// Conforms with
// Result:
// a) Interfaces list. Depending on the plugin, this can include the sandbox
// (eg, container or hypervisor) interface name and/or the host interface
// name, the hardware addresses of each interface, and details about the
// sandbox (if any) the interface is in.
// b) IP configuration assigned to each  interface. The IPv4 and/or IPv6 addresses,
// gateways, and routes assigned to sandbox and/or host interfaces.
// c) DNS information. Dictionary that includes DNS information for nameservers,
// domain, search domains and options.
func (c *libcni) GetCNIResultFromResults(results []*current.Result) (*CNIResult, error) {
	c.RLock()
	defer c.RUnlock()

	r := &CNIResult{
		Interfaces: make(map[string]*Config),
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
				return nil, errors.Wrapf(ErrInvalidResult, "failed to valid interface config: %v", err)
			}
			name := c.getInterfaceName(result.Interfaces, ipConf)
			r.Interfaces[name].IPConfigs = append(r.Interfaces[name].IPConfigs,
				&IPConfig{IP: ipConf.Address.IP, Gateway: ipConf.Gateway})
		}
		r.DNS = append(r.DNS, result.DNS)
		r.Routes = append(r.Routes, result.Routes...)
	}
	if _, ok := r.Interfaces[defaultInterface(c.prefix)]; !ok {
		return nil, errors.Wrapf(ErrNotFound, "default network not found")
	}
	return r, nil
}

// getInterfaceName returns the interface name if the plugins
// return the result with associated interfaces. If interface
// is not present then default interface name is used
func (c *libcni) getInterfaceName(interfaces []*current.Interface,
	ipConf *current.IPConfig) string {
	if ipConf.Interface != nil {
		return interfaces[*ipConf.Interface].Name
	}
	return defaultInterface(c.prefix)
}
