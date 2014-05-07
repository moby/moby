package api

import (
	"fmt"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/pkg/version"
	"github.com/dotcloud/docker/utils"
	"mime"
	"strings"
)

const (
	APIVERSION        version.Version = "1.11"
	DEFAULTHTTPHOST                   = "127.0.0.1"
	DEFAULTUNIXSOCKET                 = "/var/run/docker.sock"
)

func ValidateHost(val string) (string, error) {
	host, err := utils.ParseHost(DEFAULTHTTPHOST, DEFAULTUNIXSOCKET, val)
	if err != nil {
		return val, err
	}
	return host, nil
}

//TODO remove, used on < 1.5 in getContainersJSON
func DisplayablePorts(ports *engine.Table) string {
	result := []string{}
	ports.SetKey("PublicPort")
	ports.Sort()
	for _, port := range ports.Data {
		if port.Get("IP") == "" {
			result = append(result, fmt.Sprintf("%d/%s", port.GetInt("PublicPort"), port.Get("Type")))
		} else {
			result = append(result, fmt.Sprintf("%s:%d->%d/%s", port.Get("IP"), port.GetInt("PublicPort"), port.GetInt("PrivatePort"), port.Get("Type")))
		}
	}
	return strings.Join(result, ", ")
}

func MatchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		utils.Errorf("Error parsing media type: %s error: %s", contentType, err.Error())
	}
	return err == nil && mimetype == expectedType
}
