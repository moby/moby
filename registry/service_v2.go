package registry

import (
	"context"
	"net/url"
	"strings"

	"github.com/docker/go-connections/tlsconfig"
)

func (s *Service) lookupV2Endpoints(ctx context.Context, hostname string, includeMirrors bool) ([]APIEndpoint, error) {
	var endpoints []APIEndpoint
	if hostname == DefaultNamespace || hostname == IndexHostname {
		if includeMirrors {
			for _, mirror := range s.config.Mirrors {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if !strings.HasPrefix(mirror, "http://") && !strings.HasPrefix(mirror, "https://") {
					mirror = "https://" + mirror
				}
				mirrorURL, err := url.Parse(mirror)
				if err != nil {
					return nil, invalidParam(err)
				}
				// TODO(thaJeztah); this should all be memoized when loading the config. We're resolving mirrors and loading TLS config every time.
				mirrorTLSConfig, err := newTLSConfig(ctx, mirrorURL.Host, s.config.isSecureIndex(mirrorURL.Host))
				if err != nil {
					return nil, err
				}
				endpoints = append(endpoints, APIEndpoint{
					URL:       mirrorURL,
					Mirror:    true,
					TLSConfig: mirrorTLSConfig,
				})
			}
		}
		endpoints = append(endpoints, APIEndpoint{
			URL:       DefaultV2Registry,
			Official:  true,
			TLSConfig: tlsconfig.ServerDefault(),
		})

		return endpoints, nil
	}

	tlsConfig, err := newTLSConfig(ctx, hostname, s.config.isSecureIndex(hostname))
	if err != nil {
		return nil, err
	}

	endpoints = []APIEndpoint{
		{
			URL: &url.URL{
				Scheme: "https",
				Host:   hostname,
			},
			TLSConfig: tlsConfig,
		},
	}

	if tlsConfig.InsecureSkipVerify {
		endpoints = append(endpoints, APIEndpoint{
			URL: &url.URL{
				Scheme: "http",
				Host:   hostname,
			},
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}
