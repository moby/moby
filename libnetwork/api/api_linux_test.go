package api

import (
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/netlabel"
)

func GetOpsMap(bridgeName, defaultMTU string) map[string]string {
	if defaultMTU == "" {
		return map[string]string{
			bridge.BridgeName: bridgeName,
		}
	}
	return map[string]string{
		bridge.BridgeName:  bridgeName,
		netlabel.DriverMTU: defaultMTU,
	}
}
