package registry

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/reference"
	"github.com/docker/go-connections/tlsconfig"
)

func (s *Service) lookupV2Endpoints(repoName reference.Named) (endpoints []APIEndpoint, err error) {
	var cfg = tlsconfig.ServerDefault
	tlsConfig := &cfg
	nameString := repoName.FullName()
	logrus.Debugf("repoName is %s", nameString)
	var Main string
	//check mirror configuration
	logrus.Debugf("There are %d mirror configuration", len(s.Config.Mirrors))
	logrus.Debugf("Mirror map is %v", s.Config.Mirrors)
	if len(s.Config.Mirrors) > 0 {
		//get addr of main hub and mirror hub
		for mirror, main := range s.Config.Mirrors {
			mirrorTLSConfig, err := s.tlsConfigForMirror(mirror)
			if err != nil {
				return nil, err
			}
			//compare repo prefix to main hub
			logrus.Debugf("main is %s, and mirror is %s", main, mirror)
			if tlsConfig.InsecureSkipVerify {
				Main = strings.TrimLeft(main, "http://")
			} else {
				Main = strings.TrimLeft(main, "https://")
			}
			//get official image
			if strings.HasPrefix(nameString, DefaultNamespace+"/") {
				endpoints = append(endpoints, APIEndpoint{
					URL:          mirror,
					Version:      APIVersion2,
					Official:     true,
					TrimHostname: true,
					TLSConfig:    tlsConfig,
				})
				endpoints = append(endpoints, APIEndpoint{
					URL:          DefaultV2Registry,
					Version:      APIVersion2,
					Official:     true,
					TrimHostname: true,
					TLSConfig:    tlsConfig,
				})
			} else if strings.HasPrefix(nameString, Main+"/") { //get image from private registry
				//add mirror endpoint
				endpoints = append(endpoints, APIEndpoint{
					URL:          mirror,
					Version:      APIVersion2,
					Mirror:       true,
					TrimHostname: true,
					TLSConfig:    mirrorTLSConfig,
				})
				//add main endpoint
				endpoints = append(endpoints, APIEndpoint{
					URL:          main,
					Version:      APIVersion2,
					TrimHostname: true,
					TLSConfig:    tlsConfig,
				})
			}
		}
		logrus.Debugf("There are %d endpoints", len(endpoints))
		if len(endpoints) > 0 {
			for _, endpoint := range endpoints {
				logrus.Debugf("endpoint is %s", endpoint.URL)
			}
			return endpoints, nil
		}
	}
	//no mirror hub endpoint find
	if strings.HasPrefix(nameString, DefaultNamespace+"/") {
		// v2 registry
		endpoints = append(endpoints, APIEndpoint{
			URL:          DefaultV2Registry,
			Version:      APIVersion2,
			Official:     true,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		})

		return endpoints, nil
	}

	slashIndex := strings.IndexRune(nameString, '/')
	if slashIndex <= 0 {
		return nil, fmt.Errorf("invalid repo name: missing '/':  %s", nameString)
	}
	hostname := nameString[:slashIndex]

	tlsConfig, err = s.TLSConfig(hostname)
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL:          "https://" + hostname,
			Version:      APIVersion2,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{
			URL:          "http://" + hostname,
			Version:      APIVersion2,
			TrimHostname: true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}
