package libnetwork

import (
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
)

// OptionHostname function returns an option setter for hostname option to
// be passed to NewSandbox method.
func OptionHostname(name string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.hostName = name
	}
}

// OptionDomainname function returns an option setter for domainname option to
// be passed to NewSandbox method.
func OptionDomainname(name string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.domainName = name
	}
}

// OptionHostsPath function returns an option setter for hostspath option to
// be passed to NewSandbox method.
func OptionHostsPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.hostsPath = path
	}
}

// OptionOriginHostsPath function returns an option setter for origin hosts file path
// to be passed to NewSandbox method.
func OptionOriginHostsPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.originHostsPath = path
	}
}

// OptionExtraHost function returns an option setter for extra /etc/hosts options
// which is a name and IP as strings.
func OptionExtraHost(name string, IP string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.extraHosts = append(sb.config.extraHosts, extraHost{name: name, IP: IP})
	}
}

// OptionParentUpdate function returns an option setter for parent container
// which needs to update the IP address for the linked container.
func OptionParentUpdate(cid string, name, ip string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.parentUpdates = append(sb.config.parentUpdates, parentUpdate{cid: cid, name: name, ip: ip})
	}
}

// OptionResolvConfPath function returns an option setter for resolvconfpath option to
// be passed to net container methods.
func OptionResolvConfPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.resolvConfPath = path
	}
}

// OptionOriginResolvConfPath function returns an option setter to set the path to the
// origin resolv.conf file to be passed to net container methods.
func OptionOriginResolvConfPath(path string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.originResolvConfPath = path
	}
}

// OptionDNS function returns an option setter for dns entry option to
// be passed to container Create method.
func OptionDNS(dns []string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsList = dns
	}
}

// OptionDNSSearch function returns an option setter for dns search entry option to
// be passed to container Create method.
func OptionDNSSearch(search []string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsSearchList = search
	}
}

// OptionDNSOptions function returns an option setter for dns options entry option to
// be passed to container Create method.
func OptionDNSOptions(options []string) SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsOptionsList = options
	}
}

// OptionUseDefaultSandbox function returns an option setter for using default sandbox
// (host namespace) to be passed to container Create method.
func OptionUseDefaultSandbox() SandboxOption {
	return func(sb *Sandbox) {
		sb.config.useDefaultSandBox = true
	}
}

// OptionUseExternalKey function returns an option setter for using provided namespace
// instead of creating one.
func OptionUseExternalKey() SandboxOption {
	return func(sb *Sandbox) {
		sb.config.useExternalKey = true
	}
}

// OptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// net container creation method. Container Labels are a good example.
func OptionGeneric(generic map[string]interface{}) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{}, len(generic))
		}
		for k, v := range generic {
			sb.config.generic[k] = v
		}
	}
}

// OptionExposedPorts function returns an option setter for the container exposed
// ports option to be passed to container Create method.
func OptionExposedPorts(exposedPorts []types.TransportPort) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{})
		}
		// Defensive copy
		eps := make([]types.TransportPort, len(exposedPorts))
		copy(eps, exposedPorts)
		// Store endpoint label and in generic because driver needs it
		sb.config.exposedPorts = eps
		sb.config.generic[netlabel.ExposedPorts] = eps
	}
}

// OptionPortMapping function returns an option setter for the mapping
// ports option to be passed to container Create method.
func OptionPortMapping(portBindings []types.PortBinding) SandboxOption {
	return func(sb *Sandbox) {
		if sb.config.generic == nil {
			sb.config.generic = make(map[string]interface{})
		}
		// Store a copy of the bindings as generic data to pass to the driver
		pbs := make([]types.PortBinding, len(portBindings))
		copy(pbs, portBindings)
		sb.config.generic[netlabel.PortMap] = pbs
	}
}

// OptionIngress function returns an option setter for marking a
// sandbox as the controller's ingress sandbox.
func OptionIngress() SandboxOption {
	return func(sb *Sandbox) {
		sb.ingress = true
		sb.oslTypes = append(sb.oslTypes, osl.SandboxTypeIngress)
	}
}

// OptionLoadBalancer function returns an option setter for marking a
// sandbox as a load balancer sandbox.
func OptionLoadBalancer(nid string) SandboxOption {
	return func(sb *Sandbox) {
		sb.loadBalancerNID = nid
		sb.oslTypes = append(sb.oslTypes, osl.SandboxTypeLoadBalancer)
	}
}
