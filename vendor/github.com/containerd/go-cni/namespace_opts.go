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

type NamespaceOpts func(s *Namespace) error

// WithCapabilityPortMap adds support for port mappings
func WithCapabilityPortMap(portMapping []PortMapping) NamespaceOpts {
	return func(c *Namespace) error {
		c.capabilityArgs["portMappings"] = portMapping
		return nil
	}
}

// WithCapabilityIPRanges adds support for ip ranges
func WithCapabilityIPRanges(ipRanges []IPRanges) NamespaceOpts {
	return func(c *Namespace) error {
		c.capabilityArgs["ipRanges"] = ipRanges
		return nil
	}
}

// WithCapabilityBandWitdh adds support for bandwidth limits
func WithCapabilityBandWidth(bandWidth BandWidth) NamespaceOpts {
	return func(c *Namespace) error {
		c.capabilityArgs["bandwidth"] = bandWidth
		return nil
	}
}

// WithCapabilityDNS adds support for dns
func WithCapabilityDNS(dns DNS) NamespaceOpts {
	return func(c *Namespace) error {
		c.capabilityArgs["dns"] = dns
		return nil
	}
}

// WithCapability support well-known capabilities
// https://www.cni.dev/docs/conventions/#well-known-capabilities
func WithCapability(name string, capability interface{}) NamespaceOpts {
	return func(c *Namespace) error {
		c.capabilityArgs[name] = capability
		return nil
	}
}

// Args
func WithLabels(labels map[string]string) NamespaceOpts {
	return func(c *Namespace) error {
		for k, v := range labels {
			c.args[k] = v
		}
		return nil
	}
}

func WithArgs(k, v string) NamespaceOpts {
	return func(c *Namespace) error {
		c.args[k] = v
		return nil
	}
}
