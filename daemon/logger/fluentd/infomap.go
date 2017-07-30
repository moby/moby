package fluentd

import (
	"strings"

	"github.com/docker/docker/daemon/logger"
)

const defaultInfoKeys = "container_id,container_name"

// InfoMap returns information about a container as a map.
func infoMap(info logger.Info) map[string]string {
	infoMap := make(map[string]string)

	infoKeys, ok := info.Config["info"]
	if !ok || len(infoKeys) == 0 {
		infoKeys = defaultInfoKeys
	}

	for _, k := range strings.Split(infoKeys, ",") {
		if v, vok := infoValue(info, k); vok {
			infoMap[k] = v
		}
	}

	return infoMap
}

func infoValue(info logger.Info, key string) (string, bool) {
	switch key {
	case "container_id":
		return info.ContainerID, true
	case "container_name":
		return info.ContainerName, true
	case "containerEntrypoint":
		return info.ContainerEntrypoint, true
	case "imageID":
		return info.ContainerImageID, true
	case "imageName":
		return info.ContainerImageName, true
	case "logPath":
		return info.LogPath, true
	case "daemonName":
		return info.DaemonName, true
	default:
	}
	return "", false
}
