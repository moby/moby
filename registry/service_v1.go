package registry

import (
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/tlsconfig"
)

func (s *Service) lookupV1Endpoints(repoName string) (endpoints []APIEndpoint, err error) {
	var cfg = tlsconfig.ServerDefault
	tlsConfig := &cfg
	if strings.HasPrefix(repoName, DefaultNamespace+"/") {
		endpoints = append(endpoints, APIEndpoint{
			URL:          DefaultV1Registry,
			Version:      APIVersion1,
			Official:     true,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		})
		return endpoints, nil
	}

	slashIndex := strings.IndexRune(repoName, '/')
	if slashIndex <= 0 {
		return nil, fmt.Errorf("invalid repo name: missing '/':  %s", repoName)
	}
	hostname := repoName[:slashIndex]

	tlsConfig, err = s.TLSConfig(hostname)
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL:          "https://" + hostname,
			Version:      APIVersion1,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{ // or this
			URL:          "http://" + hostname,
			Version:      APIVersion1,
			TrimHostname: true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}
	return endpoints, nil
}
