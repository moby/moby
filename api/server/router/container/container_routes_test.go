package container

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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
			expError:      "if a container-wide MAC address is supplied, HostConfig.NetworkMode must match the identity of a network in NetworkSettings.Networks",
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
				v := v
				epConfig[k] = v
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
