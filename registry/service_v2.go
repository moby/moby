package registry // import "github.com/docker/docker/registry"

import (
	"net/url"
	"strings"

	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
)

func (s *DefaultService) lookupV2Endpoints(hostname string) (endpoints []APIEndpoint, err error) {
	tlsConfig := tlsconfig.ServerDefault()
	if hostname == DefaultNamespace || hostname == IndexHostname {
		// v2 mirrors
		for _, mirror := range s.config.Mirrors {
			uri, err := url.Parse(mirror)
			if err != nil {
				return nil, err // should not happen
			}
			if uri.Scheme == "" {
				if strings.HasPrefix(mirror, "/") {
					// in future, we may want to let `mirror = "unix://" + mirror` in this case
					return nil, errdefs.InvalidParameter(errors.Errorf("invalid mirror server address %q", mirror))
				}
				mirror = "https://" + mirror
			}
			mirrorURL, err := url.Parse(mirror)
			if err != nil {
				return nil, err
			}
			mirrorTLSConfig, err := s.tlsConfigForMirror(mirrorURL)
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, APIEndpoint{
				URL: mirrorURL,
				// guess mirrors are v2
				Version:      APIVersion2,
				Mirror:       true,
				TrimHostname: true,
				TLSConfig:    mirrorTLSConfig,
			})
		}
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

	ana := allowNondistributableArtifacts(s.config, hostname)

	tlsConfig, err = s.tlsConfig(hostname)
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL: &url.URL{
				Scheme: "https",
				Host:   hostname,
			},
			Version: APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			TLSConfig:                      tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{
			URL: &url.URL{
				Scheme: "http",
				Host:   hostname,
			},
			Version: APIVersion2,
			AllowNondistributableArtifacts: ana,
			TrimHostname:                   true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}
