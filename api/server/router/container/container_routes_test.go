package container

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/libnetwork/netlabel"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHandleMACAddressBC(t *testing.T) {
	testcases := []struct {
		name                string
		apiVersion          string
		ctrWideMAC          string
		networkMode         container.NetworkMode
		epConfig            map[string]*network.EndpointSettings
		expEpWithCtrWideMAC string
		expEpWithNoMAC      string
		expCtrWideMAC       string
		expWarning          string
		expError            string
	}{
		{
			name:                "old api ctr-wide mac mix id and name",
			apiVersion:          "1.43",
			ctrWideMAC:          "11:22:33:44:55:66",
			networkMode:         "aNetId",
			epConfig:            map[string]*network.EndpointSettings{"aNetName": {}},
			expEpWithCtrWideMAC: "aNetName",
			expCtrWideMAC:       "11:22:33:44:55:66",
		},
		{
			name:           "old api clear ep mac",
			apiVersion:     "1.43",
			networkMode:    "aNetId",
			epConfig:       map[string]*network.EndpointSettings{"aNetName": {MacAddress: "11:22:33:44:55:66"}},
			expEpWithNoMAC: "aNetName",
		},
		{
			name:          "old api no-network ctr-wide mac",
			apiVersion:    "1.43",
			networkMode:   "none",
			ctrWideMAC:    "11:22:33:44:55:66",
			expError:      "conflicting options: mac-address and the network mode",
			expCtrWideMAC: "11:22:33:44:55:66",
		},
		{
			name:                "old api create ep",
			apiVersion:          "1.43",
			networkMode:         "aNetId",
			ctrWideMAC:          "11:22:33:44:55:66",
			epConfig:            map[string]*network.EndpointSettings{},
			expEpWithCtrWideMAC: "aNetId",
			expCtrWideMAC:       "11:22:33:44:55:66",
		},
		{
			name:                "old api migrate ctr-wide mac",
			apiVersion:          "1.43",
			ctrWideMAC:          "11:22:33:44:55:66",
			networkMode:         "aNetName",
			epConfig:            map[string]*network.EndpointSettings{"aNetName": {}},
			expEpWithCtrWideMAC: "aNetName",
			expCtrWideMAC:       "11:22:33:44:55:66",
		},
		{
			name:        "new api no macs",
			apiVersion:  "1.44",
			networkMode: "aNetId",
			epConfig:    map[string]*network.EndpointSettings{"aNetName": {}},
		},
		{
			name:        "new api ep specific mac",
			apiVersion:  "1.44",
			networkMode: "aNetName",
			epConfig:    map[string]*network.EndpointSettings{"aNetName": {MacAddress: "11:22:33:44:55:66"}},
		},
		{
			name:                "new api migrate ctr-wide mac to new ep",
			apiVersion:          "1.44",
			ctrWideMAC:          "11:22:33:44:55:66",
			networkMode:         "aNetName",
			epConfig:            map[string]*network.EndpointSettings{},
			expEpWithCtrWideMAC: "aNetName",
			expWarning:          "The container-wide MacAddress field is now deprecated",
			expCtrWideMAC:       "",
		},
		{
			name:                "new api migrate ctr-wide mac to existing ep",
			apiVersion:          "1.44",
			ctrWideMAC:          "11:22:33:44:55:66",
			networkMode:         "aNetName",
			epConfig:            map[string]*network.EndpointSettings{"aNetName": {}},
			expEpWithCtrWideMAC: "aNetName",
			expWarning:          "The container-wide MacAddress field is now deprecated",
			expCtrWideMAC:       "",
		},
		{
			name:          "new api mode vs name mismatch",
			apiVersion:    "1.44",
			ctrWideMAC:    "11:22:33:44:55:66",
			networkMode:   "aNetId",
			epConfig:      map[string]*network.EndpointSettings{"aNetName": {}},
			expError:      "unable to migrate container-wide MAC address to a specific network: HostConfig.NetworkMode must match the identity of a network in NetworkSettings.Networks",
			expCtrWideMAC: "11:22:33:44:55:66",
		},
		{
			name:          "new api mac mismatch",
			apiVersion:    "1.44",
			ctrWideMAC:    "11:22:33:44:55:66",
			networkMode:   "aNetName",
			epConfig:      map[string]*network.EndpointSettings{"aNetName": {MacAddress: "00:11:22:33:44:55"}},
			expError:      "the container-wide MAC address must match the endpoint-specific MAC address",
			expCtrWideMAC: "11:22:33:44:55:66",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &container.Config{
				MacAddress: tc.ctrWideMAC, //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
			}
			hostCfg := &container.HostConfig{
				NetworkMode: tc.networkMode,
			}
			epConfig := make(map[string]*network.EndpointSettings, len(tc.epConfig))
			for k, v := range tc.epConfig {
				v := *v
				epConfig[k] = &v
			}
			netCfg := &network.NetworkingConfig{
				EndpointsConfig: epConfig,
			}

			warning, err := handleMACAddressBC(cfg, hostCfg, netCfg, tc.apiVersion)

			if tc.expError == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.ErrorContains(err, tc.expError))
			}
			if tc.expWarning == "" {
				assert.Check(t, is.Equal(warning, ""))
			} else {
				assert.Check(t, is.Contains(warning, tc.expWarning))
			}
			if tc.expEpWithCtrWideMAC != "" {
				got := netCfg.EndpointsConfig[tc.expEpWithCtrWideMAC].MacAddress
				assert.Check(t, is.Equal(got, tc.ctrWideMAC))
			}
			if tc.expEpWithNoMAC != "" {
				got := netCfg.EndpointsConfig[tc.expEpWithNoMAC].MacAddress
				assert.Check(t, is.Equal(got, ""))
			}
			gotCtrWideMAC := cfg.MacAddress //nolint:staticcheck // ignore SA1019: field is deprecated, but still used on API < v1.44.
			assert.Check(t, is.Equal(gotCtrWideMAC, tc.expCtrWideMAC))
		})
	}
}

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
