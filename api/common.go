package api

import (
	"fmt"
	"mime"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/version"
)

const (
	APIVERSION        version.Version = "1.14"
	DEFAULTHTTPHOST                   = "127.0.0.1"
	DEFAULTUNIXSOCKET                 = "/var/run/docker.sock"
)

func ValidateHost(val string) (string, error) {
	host, err := parsers.ParseHost(DEFAULTHTTPHOST, DEFAULTUNIXSOCKET, val)
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
			result = append(result, fmt.Sprintf("%d/%s", port.GetInt("PrivatePort"), port.Get("Type")))
		} else {
			result = append(result, fmt.Sprintf("%s:%d->%d/%s", port.Get("IP"), port.GetInt("PublicPort"), port.GetInt("PrivatePort"), port.Get("Type")))
		}
	}
	return strings.Join(result, ", ")
}

func MatchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		log.Errorf("Error parsing media type: %s error: %s", contentType, err.Error())
	}
	return err == nil && mimetype == expectedType
}
