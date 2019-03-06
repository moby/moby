package client

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
)

// ErrRedirect returned when unexpected redirects encountered
var ErrRedirect = errors.New("unexpected redirect in response")

func checkRedirect(req *http.Request, via []*http.Request) error {
	if via[0].Method == http.MethodGet {
		return http.ErrUseLastResponse
	}
	return ErrRedirect
}

// SettingsFromEnv creates a Settings based on the standard Docker env vars
func SettingsFromEnv() (*Settings, error) {
	settings := Settings{
		Host:  DefaultDockerHost,
		Proto: defaultProto,
		Addr:  defaultAddr,
	}
	transport := new(http.Transport)
	sockets.ConfigureTransport(transport, settings.Proto, settings.Addr)
	settings.Client = &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		settings.Client = &http.Client{
			Transport:     &http.Transport{TLSClientConfig: tlsc},
			CheckRedirect: checkRedirect,
		}
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		protoAddrParts := strings.SplitN(host, "://", 2)
		if len(protoAddrParts) == 1 {
			return nil, fmt.Errorf("unable to parse docker host `%s`", host)
		}

		var basePath string
		proto, addr := protoAddrParts[0], protoAddrParts[1]
		if proto == "tcp" {
			parsed, err := url.Parse("tcp://" + addr)
			if err != nil {
				return nil, err
			}
			addr = parsed.Host
			basePath = parsed.Path
		}
		settings.Host = host
		settings.Proto = proto
		settings.Addr = addr
		settings.BasePath = basePath
		if transport, ok := settings.Client.Transport.(*http.Transport); ok {
			err := sockets.ConfigureTransport(transport, settings.Proto, settings.Addr)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.Errorf("cannot apply host to transport: %T", settings.Client.Transport)
		}
	}

	if version := os.Getenv("DOCKER_API_VERSION"); version != "" {
		settings.Version = version
	}
	settings.Scheme = "http"
	switch tr := settings.Client.Transport.(type) {
	case *http.Transport:
		if tr.TLSClientConfig != nil {
			settings.Scheme = "https"
		}
	}

	return &settings, nil
}
