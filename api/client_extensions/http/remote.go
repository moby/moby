package http

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/registry"
)

var (
	ErrConnectionRefused = errors.New("Cannot connect to the Docker daemon. Is 'docker -d' running on this host?")
)

func init() {
	client.RegisterRemote("http", Init)
}

type HttpRemote struct {
	name string
}

var Init = func() (client.CliRemote, error) {
	return &HttpRemote{
		name: "base",
	}, nil
}

func (hr *HttpRemote) Dial(cli *client.DockerCli) (net.Conn, error) {
	if cli.TlsConfig != nil && cli.Proto != "unix" {
		return tls.Dial(cli.Proto, cli.Address, cli.TlsConfig)
	}

	return net.Dial(cli.Proto, cli.Address)
}

func (hr *HttpRemote) Call(cli *client.DockerCli, callDetails *client.CallDetails) (io.ReadCloser, int, error) {
	params := bytes.NewBuffer(nil)
	if callDetails.Data != nil {
		if env, ok := callDetails.Data.(engine.Env); ok {
			if err := env.Encode(params); err != nil {
				return nil, -1, err
			}
		} else {
			buf, err := json.Marshal(callDetails.Data)
			if err != nil {
				return nil, -1, err
			}
			if _, err := params.Write(buf); err != nil {
				return nil, -1, err
			}
		}
	}

	req, err := http.NewRequest(callDetails.Method, fmt.Sprintf("/v%s%s", api.APIVERSION, callDetails.Path), params)
	if err != nil {
		return nil, -1, err
	}
	if callDetails.PassAuthInfo {
		cli.LoadConfigFile()
		// Resolve the Auth config relevant for this server
		authConfig := cli.ConfigFile.ResolveAuthConfig(registry.IndexServerAddress())
		getHeaders := func(authConfig registry.AuthConfig) (map[string][]string, error) {
			buf, err := json.Marshal(authConfig)
			if err != nil {
				return nil, err
			}
			registryAuthHeader := []string{
				base64.URLEncoding.EncodeToString(buf),
			}
			return map[string][]string{"X-Registry-Auth": registryAuthHeader}, nil
		}
		if headers, err := getHeaders(authConfig); err == nil && headers != nil {
			for k, v := range headers {
				req.Header[k] = v
			}
		}
	}
	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION)
	req.URL.Host = cli.Address
	req.URL.Scheme = cli.Scheme
	if callDetails.Data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if callDetails.Method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := cli.HTTPClient().Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, -1, err
		}
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(resp.StatusCode), req.URL)
		}
		return nil, resp.StatusCode, fmt.Errorf("Error: %s", bytes.TrimSpace(body))
	}
	return resp.Body, resp.StatusCode, nil
}

func (hr *HttpRemote) String() string {
	return hr.name
}
