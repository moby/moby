package container

import (
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEpConfigForNetMode(t *testing.T) {
	testcases := []struct {
		name        string
		apiVersion  string
		networkMode string
		epConfig    map[string]*network.EndpointSettings
		expEpId     string
		expNumEps   int
		expError    bool
	}{
		{
			name:        "old api no eps",
			apiVersion:  "1.43",
			networkMode: "mynet",
			expNumEps:   1,
		},
		{
			name:        "new api no eps",
			apiVersion:  "1.44",
			networkMode: "mynet",
			expNumEps:   1,
		},
		{
			name:        "old api with ep",
			apiVersion:  "1.43",
			networkMode: "mynet",
			epConfig: map[string]*network.EndpointSettings{
				"anything": {EndpointID: "epone"},
			},
			expEpId:   "epone",
			expNumEps: 1,
		},
		{
			name:        "new api with matching ep",
			apiVersion:  "1.44",
			networkMode: "mynet",
			epConfig: map[string]*network.EndpointSettings{
				"mynet": {EndpointID: "epone"},
			},
			expEpId:   "epone",
			expNumEps: 1,
		},
		{
			name:        "new api with mismatched ep",
			apiVersion:  "1.44",
			networkMode: "mynet",
			epConfig: map[string]*network.EndpointSettings{
				"shortid": {EndpointID: "epone"},
			},
			expError: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			netConfig := &network.NetworkingConfig{
				EndpointsConfig: tc.epConfig,
			}
			ep, err := epConfigForNetMode(tc.apiVersion, container.NetworkMode(tc.networkMode), netConfig)
			if tc.expError {
				assert.Check(t, is.ErrorContains(err, "HostConfig.NetworkMode must match the identity of a network in NetworkSettings.Networks"))
			} else {
				assert.Assert(t, err)
				assert.Check(t, is.Equal(ep.EndpointID, tc.expEpId))
				assert.Check(t, is.Len(netConfig.EndpointsConfig, tc.expNumEps))
			}
		})
	}
}

func TestHandleSysctlBC(t *testing.T) {
	testcases := []struct {
		name               string
		apiVersion         string
		networkMode        string
		sysctls            map[string]string
		epConfig           map[string]*network.EndpointSettings
		expEpSysctls       []string
		expSysctls         map[string]string
		expWarningContains []string
		expError           string
	}{
		{
			name:        "migrate to new ep",
			apiVersion:  "1.46",
			networkMode: "mynet",
			sysctls: map[string]string{
				"net.ipv6.conf.all.disable_ipv6": "0",
				"net.ipv6.conf.eth0.accept_ra":   "2",
				"net.ipv6.conf.eth0.forwarding":  "1",
			},
			expSysctls: map[string]string{
				"net.ipv6.conf.all.disable_ipv6": "0",
			},
			expEpSysctls: []string{"net.ipv6.conf.IFNAME.forwarding=1", "net.ipv6.conf.IFNAME.accept_ra=2"},
			expWarningContains: []string{
				"Migrated",
				"net.ipv6.conf.eth0.accept_ra", "net.ipv6.conf.IFNAME.accept_ra=2",
				"net.ipv6.conf.eth0.forwarding", "net.ipv6.conf.IFNAME.forwarding=1",
			},
		},
		{
			name:        "migrate nothing",
			apiVersion:  "1.46",
			networkMode: "mynet",
			sysctls: map[string]string{
				"net.ipv6.conf.all.disable_ipv6": "0",
			},
			expSysctls: map[string]string{
				"net.ipv6.conf.all.disable_ipv6": "0",
			},
		},
		{
			name:        "migration disabled for newer api",
			apiVersion:  "1.48",
			networkMode: "mynet",
			sysctls: map[string]string{
				"net.ipv6.conf.eth0.accept_ra": "2",
			},
			expError: "must be supplied using driver option 'com.docker.network.endpoint.sysctls'",
		},
		{
			name:        "only migrate eth0",
			apiVersion:  "1.46",
			networkMode: "mynet",
			sysctls: map[string]string{
				"net.ipv6.conf.eth1.accept_ra": "2",
			},
			expError: "unable to determine network endpoint",
		},
		{
			name:        "net name mismatch",
			apiVersion:  "1.46",
			networkMode: "mynet",
			epConfig: map[string]*network.EndpointSettings{
				"shortid": {EndpointID: "epone"},
			},
			sysctls: map[string]string{
				"net.ipv6.conf.eth1.accept_ra": "2",
			},
			expError: "unable to find a network for sysctl",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			hostCfg := &container.HostConfig{
				NetworkMode: container.NetworkMode(tc.networkMode),
				Sysctls:     map[string]string{},
			}
			for k, v := range tc.sysctls {
				hostCfg.Sysctls[k] = v
			}
			netCfg := &network.NetworkingConfig{
				EndpointsConfig: tc.epConfig,
			}

			warnings, err := handleSysctlBC(hostCfg, netCfg, tc.apiVersion)

			for _, s := range tc.expWarningContains {
				assert.Check(t, is.Contains(warnings, s))
			}

			if tc.expError != "" {
				assert.Check(t, is.ErrorContains(err, tc.expError))
			} else {
				assert.Check(t, err)

				assert.Check(t, is.DeepEqual(hostCfg.Sysctls, tc.expSysctls))

				ep := netCfg.EndpointsConfig[tc.networkMode]
				if ep == nil {
					assert.Check(t, is.Nil(tc.expEpSysctls))
				} else {
					got, ok := ep.DriverOpts[netlabel.EndpointSysctls]
					assert.Check(t, ok)
					// Check for expected ep-sysctls.
					for _, want := range tc.expEpSysctls {
						assert.Check(t, is.Contains(got, want))
					}
					// Check for unexpected ep-sysctls.
					assert.Check(t, is.Len(got, len(strings.Join(tc.expEpSysctls, ","))))
				}
			}
		})
	}
}
