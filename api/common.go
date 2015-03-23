package api

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/libtrust"
)

const (
	APIVERSION            version.Version = "1.18"
	DEFAULTHTTPHOST                       = "127.0.0.1"
	DEFAULTUNIXSOCKET                     = "/var/run/docker.sock"
	DefaultDockerfileName string          = "Dockerfile"
)

func ValidateHost(val string) (string, error) {
	host, err := parsers.ParseHost(DEFAULTHTTPHOST, DEFAULTUNIXSOCKET, val)
	if err != nil {
		return val, err
	}
	return host, nil
}

// TODO remove, used on < 1.5 in getContainersJSON
func DisplayablePorts(ports *engine.Table) string {
	var (
		result          = []string{}
		hostMappings    = []string{}
		firstInGroupMap map[string]int
		lastInGroupMap  map[string]int
	)
	firstInGroupMap = make(map[string]int)
	lastInGroupMap = make(map[string]int)
	ports.SetKey("PrivatePort")
	ports.Sort()
	for _, port := range ports.Data {
		var (
			current      = port.GetInt("PrivatePort")
			portKey      = port.Get("Type")
			firstInGroup int
			lastInGroup  int
		)
		if port.Get("IP") != "" {
			if port.GetInt("PublicPort") != current {
				hostMappings = append(hostMappings, fmt.Sprintf("%s:%d->%d/%s", port.Get("IP"), port.GetInt("PublicPort"), port.GetInt("PrivatePort"), port.Get("Type")))
				continue
			}
			portKey = fmt.Sprintf("%s/%s", port.Get("IP"), port.Get("Type"))
		}
		firstInGroup = firstInGroupMap[portKey]
		lastInGroup = lastInGroupMap[portKey]

		if firstInGroup == 0 {
			firstInGroupMap[portKey] = current
			lastInGroupMap[portKey] = current
			continue
		}

		if current == (lastInGroup + 1) {
			lastInGroupMap[portKey] = current
			continue
		}
		result = append(result, FormGroup(portKey, firstInGroup, lastInGroup))
		firstInGroupMap[portKey] = current
		lastInGroupMap[portKey] = current
	}
	for portKey, firstInGroup := range firstInGroupMap {
		result = append(result, FormGroup(portKey, firstInGroup, lastInGroupMap[portKey]))
	}
	result = append(result, hostMappings...)
	return strings.Join(result, ", ")
}

func FormGroup(key string, start, last int) string {
	var (
		group     string
		parts     = strings.Split(key, "/")
		groupType = parts[0]
		ip        = ""
	)
	if len(parts) > 1 {
		ip = parts[0]
		groupType = parts[1]
	}
	if start == last {
		group = fmt.Sprintf("%d", start)
	} else {
		group = fmt.Sprintf("%d-%d", start, last)
	}
	if ip != "" {
		group = fmt.Sprintf("%s:%s->%s", ip, group, group)
	}
	return fmt.Sprintf("%s/%s", group, groupType)
}

func MatchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		log.Errorf("Error parsing media type: %s error: %v", contentType, err)
	}
	return err == nil && mimetype == expectedType
}

// LoadOrCreateTrustKey attempts to load the libtrust key at the given path,
// otherwise generates a new one
func LoadOrCreateTrustKey(trustKeyPath string) (libtrust.PrivateKey, error) {
	err := os.MkdirAll(filepath.Dir(trustKeyPath), 0700)
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
		return nil, fmt.Errorf("Error loading key file %s: %s", trustKeyPath, err)
	}
	return trustKey, nil
}
