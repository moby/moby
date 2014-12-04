package api

import (
	"fmt"
	"mime"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/libtrust"
)

const (
	APIVERSION        version.Version = "1.16"
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

// LoadOrCreateTrustKey attempts to load the libtrust key at the given path,
// otherwise generates a new one
func LoadOrCreateTrustKey(trustKeyPath string) (libtrust.PrivateKey, error) {
	err := os.MkdirAll(path.Dir(trustKeyPath), 0700)
	if err != nil {
		return nil, err
	}
	trustKey, err := libtrust.LoadKeyFile(trustKeyPath)
	if err == libtrust.ErrKeyFileDoesNotExist {
		trustKey, err = libtrust.GenerateECP256PrivateKey()
		if err != nil {
			return nil, fmt.Errorf("Error generating key: %s", err)
		}
		if err := libtrust.SaveKey(trustKeyPath, trustKey); err != nil {
			return nil, fmt.Errorf("Error saving key file: %s", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("Error loading key file: %s", err)
	}
	return trustKey, nil
}
