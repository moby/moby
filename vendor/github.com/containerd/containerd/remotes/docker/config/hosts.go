/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package config contains utilities for helping configure the Docker resolver
package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/containerd/log"
	"github.com/pelletier/go-toml"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes/docker"
)

// UpdateClientFunc is a function that lets you to amend http Client behavior used by registry clients.
type UpdateClientFunc func(client *http.Client) error

type hostConfig struct {
	scheme string
	host   string
	path   string

	capabilities docker.HostCapabilities

	caCerts     []string
	clientPairs [][2]string
	skipVerify  *bool

	header http.Header

	// TODO: Add credential configuration (domain alias, username)
}

// HostOptions is used to configure registry hosts
type HostOptions struct {
	HostDir       func(string) (string, error)
	Credentials   func(host string) (string, string, error)
	DefaultTLS    *tls.Config
	DefaultScheme string
	// UpdateClient will be called after creating http.Client object, so clients can provide extra configuration
	UpdateClient   UpdateClientFunc
	AuthorizerOpts []docker.AuthorizerOpt
}

// ConfigureHosts creates a registry hosts function from the provided
// host creation options. The host directory can read hosts.toml or
// certificate files laid out in the Docker specific layout.
// If a `HostDir` function is not required, defaults are used.
func ConfigureHosts(ctx context.Context, options HostOptions) docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		var hosts []hostConfig
		if options.HostDir != nil {
			dir, err := options.HostDir(host)
			if err != nil && !errdefs.IsNotFound(err) {
				return nil, err
			}
			if dir != "" {
				log.G(ctx).WithField("dir", dir).Debug("loading host directory")
				hosts, err = loadHostDir(ctx, dir)
				if err != nil {
					return nil, err
				}
			}
		}

		// If hosts was not set, add a default host
		// NOTE: Check nil here and not empty, the host may be
		// intentionally configured to not have any endpoints
		if hosts == nil {
			hosts = make([]hostConfig, 1)
		}
		if len(hosts) > 0 && hosts[len(hosts)-1].host == "" {
			if host == "docker.io" {
				hosts[len(hosts)-1].scheme = "https"
				hosts[len(hosts)-1].host = "registry-1.docker.io"
			} else if docker.IsLocalhost(host) {
				hosts[len(hosts)-1].host = host
				if options.DefaultScheme == "" {
					_, port, _ := net.SplitHostPort(host)
					if port == "" || port == "443" {
						// If port is default or 443, only use https
						hosts[len(hosts)-1].scheme = "https"
					} else {
						// HTTP fallback logic will be used when protocol is ambiguous
						hosts[len(hosts)-1].scheme = "http"
					}

					// When port is 80, protocol is not ambiguous
					if port != "80" {
						// Skipping TLS verification for localhost
						var skipVerify = true
						hosts[len(hosts)-1].skipVerify = &skipVerify
					}
				} else {
					hosts[len(hosts)-1].scheme = options.DefaultScheme
				}
			} else {
				hosts[len(hosts)-1].host = host
				if options.DefaultScheme != "" {
					hosts[len(hosts)-1].scheme = options.DefaultScheme
				} else {
					hosts[len(hosts)-1].scheme = "https"
				}
			}
			hosts[len(hosts)-1].path = "/v2"
			hosts[len(hosts)-1].capabilities = docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush
		}

		// tlsConfigured indicates that TLS was configured and HTTP endpoints should
		// attempt to use the TLS configuration before falling back to HTTP
		var tlsConfigured bool

		var defaultTLSConfig *tls.Config
		if options.DefaultTLS != nil {
			tlsConfigured = true
			defaultTLSConfig = options.DefaultTLS
		} else {
			defaultTLSConfig = &tls.Config{}
		}

		defaultTransport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:       30 * time.Second,
				KeepAlive:     30 * time.Second,
				FallbackDelay: 300 * time.Millisecond,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			TLSClientConfig:       defaultTLSConfig,
			ExpectContinueTimeout: 5 * time.Second,
		}

		client := &http.Client{
			Transport: defaultTransport,
		}
		if options.UpdateClient != nil {
			if err := options.UpdateClient(client); err != nil {
				return nil, err
			}
		}

		authOpts := []docker.AuthorizerOpt{docker.WithAuthClient(client)}
		if options.Credentials != nil {
			authOpts = append(authOpts, docker.WithAuthCreds(options.Credentials))
		}
		authOpts = append(authOpts, options.AuthorizerOpts...)
		authorizer := docker.NewDockerAuthorizer(authOpts...)

		rhosts := make([]docker.RegistryHost, len(hosts))
		for i, host := range hosts {
			// Allow setting for each host as well
			explicitTLS := tlsConfigured

			if host.caCerts != nil || host.clientPairs != nil || host.skipVerify != nil {
				explicitTLS = true
				tr := defaultTransport.Clone()
				tlsConfig := tr.TLSClientConfig
				if host.skipVerify != nil {
					tlsConfig.InsecureSkipVerify = *host.skipVerify
				}
				if host.caCerts != nil {
					if tlsConfig.RootCAs == nil {
						rootPool, err := rootSystemPool()
						if err != nil {
							return nil, fmt.Errorf("unable to initialize cert pool: %w", err)
						}
						tlsConfig.RootCAs = rootPool
					}
					for _, f := range host.caCerts {
						data, err := os.ReadFile(f)
						if err != nil {
							return nil, fmt.Errorf("unable to read CA cert %q: %w", f, err)
						}
						if !tlsConfig.RootCAs.AppendCertsFromPEM(data) {
							return nil, fmt.Errorf("unable to load CA cert %q", f)
						}
					}
				}

				if host.clientPairs != nil {
					for _, pair := range host.clientPairs {
						certPEMBlock, err := os.ReadFile(pair[0])
						if err != nil {
							return nil, fmt.Errorf("unable to read CERT file %q: %w", pair[0], err)
						}
						var keyPEMBlock []byte
						if pair[1] != "" {
							keyPEMBlock, err = os.ReadFile(pair[1])
							if err != nil {
								return nil, fmt.Errorf("unable to read CERT file %q: %w", pair[1], err)
							}
						} else {
							// Load key block from same PEM file
							keyPEMBlock = certPEMBlock
						}
						cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
						if err != nil {
							return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
						}

						tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
					}
				}

				c := *client
				c.Transport = tr
				if options.UpdateClient != nil {
					if err := options.UpdateClient(&c); err != nil {
						return nil, err
					}
				}

				rhosts[i].Client = &c
				rhosts[i].Authorizer = docker.NewDockerAuthorizer(append(authOpts, docker.WithAuthClient(&c))...)
			} else {
				rhosts[i].Client = client
				rhosts[i].Authorizer = authorizer
			}

			// When TLS has been configured for the operation or host and
			// the protocol from the port number is ambiguous, use the
			// docker.NewHTTPFallback roundtripper to catch TLS errors and re-attempt the
			// request as http. This allows preference for https when configured but
			// also catches TLS errors early enough in the request to avoid sending
			// the request twice or consuming the request body.
			if host.scheme == "http" && explicitTLS {
				_, port, _ := net.SplitHostPort(host.host)
				if port != "" && port != "80" {
					log.G(ctx).WithField("host", host.host).Info("host will try HTTPS first since it is configured for HTTP with a TLS configuration, consider changing host to HTTPS or removing unused TLS configuration")
					host.scheme = "https"
					rhosts[i].Client.Transport = docker.NewHTTPFallback(rhosts[i].Client.Transport)
				}
			}

			rhosts[i].Scheme = host.scheme
			rhosts[i].Host = host.host
			rhosts[i].Path = host.path
			rhosts[i].Capabilities = host.capabilities
			rhosts[i].Header = host.header
		}

		return rhosts, nil
	}

}

// HostDirFromRoot returns a function which finds a host directory
// based at the given root.
func HostDirFromRoot(root string) func(string) (string, error) {
	return func(host string) (string, error) {
		for _, p := range hostPaths(root, host) {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
		return "", errdefs.ErrNotFound
	}
}

// hostDirectory converts ":port" to "_port_" in directory names
func hostDirectory(host string) string {
	idx := strings.LastIndex(host, ":")
	if idx > 0 {
		return host[:idx] + "_" + host[idx+1:] + "_"
	}
	return host
}

func loadHostDir(ctx context.Context, hostsDir string) ([]hostConfig, error) {
	b, err := os.ReadFile(filepath.Join(hostsDir, "hosts.toml"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if len(b) == 0 {
		// If hosts.toml does not exist, fallback to checking for
		// certificate files based on Docker's certificate file
		// pattern (".crt", ".cert", ".key" files)
		return loadCertFiles(ctx, hostsDir)
	}

	hosts, err := parseHostsFile(hostsDir, b)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to decode hosts.toml")
		// Fallback to checking certificate files
		return loadCertFiles(ctx, hostsDir)
	}

	return hosts, nil
}

type hostFileConfig struct {
	// Capabilities determine what operations a host is
	// capable of performing. Allowed values
	//  - pull
	//  - resolve
	//  - push
	Capabilities []string `toml:"capabilities"`

	// CACert are the public key certificates for TLS
	// Accepted types
	// - string - Single file with certificate(s)
	// - []string - Multiple files with certificates
	CACert interface{} `toml:"ca"`

	// Client keypair(s) for TLS with client authentication
	// Accepted types
	// - string - Single file with public and private keys
	// - []string - Multiple files with public and private keys
	// - [][2]string - Multiple keypairs with public and private keys in separate files
	Client interface{} `toml:"client"`

	// SkipVerify skips verification of the server's certificate chain
	// and host name. This should only be used for testing or in
	// combination with other methods of verifying connections.
	SkipVerify *bool `toml:"skip_verify"`

	// Header are additional header files to send to the server
	Header map[string]interface{} `toml:"header"`

	// OverridePath indicates the API root endpoint is defined in the URL
	// path rather than by the API specification.
	// This may be used with non-compliant OCI registries to override the
	// API root endpoint.
	OverridePath bool `toml:"override_path"`

	// TODO: Credentials: helper? name? username? alternate domain? token?
}

func parseHostsFile(baseDir string, b []byte) ([]hostConfig, error) {
	tree, err := toml.LoadBytes(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	// HACK: we want to keep toml parsing structures private in this package, however go-toml ignores private embedded types.
	// so we remap it to a public type within the func body, so technically it's public, but not possible to import elsewhere.
	type HostFileConfig = hostFileConfig

	c := struct {
		HostFileConfig
		// Server specifies the default server. When `host` is
		// also specified, those hosts are tried first.
		Server string `toml:"server"`
		// HostConfigs store the per-host configuration
		HostConfigs map[string]hostFileConfig `toml:"host"`
	}{}

	orderedHosts, err := getSortedHosts(tree)
	if err != nil {
		return nil, err
	}

	var (
		hosts []hostConfig
	)

	if err := tree.Unmarshal(&c); err != nil {
		return nil, err
	}

	// Parse hosts array
	for _, host := range orderedHosts {
		config := c.HostConfigs[host]

		parsed, err := parseHostConfig(host, baseDir, config)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, parsed)
	}

	// Parse root host config and append it as the last element
	parsed, err := parseHostConfig(c.Server, baseDir, c.HostFileConfig)
	if err != nil {
		return nil, err
	}
	hosts = append(hosts, parsed)

	return hosts, nil
}

func parseHostConfig(server string, baseDir string, config hostFileConfig) (hostConfig, error) {
	var (
		result = hostConfig{}
		err    error
	)

	if server != "" {
		if !strings.HasPrefix(server, "http") {
			server = "https://" + server
		}
		u, err := url.Parse(server)
		if err != nil {
			return hostConfig{}, fmt.Errorf("unable to parse server %v: %w", server, err)
		}
		result.scheme = u.Scheme
		result.host = u.Host
		if len(u.Path) > 0 {
			u.Path = path.Clean(u.Path)
			if !strings.HasSuffix(u.Path, "/v2") && !config.OverridePath {
				u.Path = u.Path + "/v2"
			}
		} else if !config.OverridePath {
			u.Path = "/v2"
		}
		result.path = u.Path
	}

	result.skipVerify = config.SkipVerify

	if len(config.Capabilities) > 0 {
		for _, c := range config.Capabilities {
			switch strings.ToLower(c) {
			case "pull":
				result.capabilities |= docker.HostCapabilityPull
			case "resolve":
				result.capabilities |= docker.HostCapabilityResolve
			case "push":
				result.capabilities |= docker.HostCapabilityPush
			default:
				return hostConfig{}, fmt.Errorf("unknown capability %v", c)
			}
		}
	} else {
		result.capabilities = docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush
	}

	if config.CACert != nil {
		switch cert := config.CACert.(type) {
		case string:
			result.caCerts = []string{makeAbsPath(cert, baseDir)}
		case []interface{}:
			result.caCerts, err = makeStringSlice(cert, func(p string) string {
				return makeAbsPath(p, baseDir)
			})
			if err != nil {
				return hostConfig{}, err
			}
		default:
			return hostConfig{}, fmt.Errorf("invalid type %v for \"ca\"", cert)
		}
	}

	if config.Client != nil {
		switch client := config.Client.(type) {
		case string:
			result.clientPairs = [][2]string{{makeAbsPath(client, baseDir), ""}}
		case []interface{}:
			// []string or [][2]string
			for _, pairs := range client {
				switch p := pairs.(type) {
				case string:
					result.clientPairs = append(result.clientPairs, [2]string{makeAbsPath(p, baseDir), ""})
				case []interface{}:
					slice, err := makeStringSlice(p, func(s string) string {
						return makeAbsPath(s, baseDir)
					})
					if err != nil {
						return hostConfig{}, err
					}
					if len(slice) != 2 {
						return hostConfig{}, fmt.Errorf("invalid pair %v for \"client\"", p)
					}

					var pair [2]string
					copy(pair[:], slice)
					result.clientPairs = append(result.clientPairs, pair)
				default:
					return hostConfig{}, fmt.Errorf("invalid type %T for \"client\"", p)
				}
			}
		default:
			return hostConfig{}, fmt.Errorf("invalid type %v for \"client\"", client)
		}
	}

	if config.Header != nil {
		header := http.Header{}
		for key, ty := range config.Header {
			switch value := ty.(type) {
			case string:
				header[key] = []string{value}
			case []interface{}:
				header[key], err = makeStringSlice(value, nil)
				if err != nil {
					return hostConfig{}, err
				}
			default:
				return hostConfig{}, fmt.Errorf("invalid type %v for header %q", ty, key)
			}
		}
		result.header = header
	}

	return result, nil
}

// getSortedHosts returns the list of hosts as they defined in the file.
func getSortedHosts(root *toml.Tree) ([]string, error) {
	iter, ok := root.Get("host").(*toml.Tree)
	if !ok {
		return nil, errors.New("invalid `host` tree")
	}

	list := append([]string{}, iter.Keys()...)

	// go-toml stores TOML sections in the map object, so no order guaranteed.
	// We retrieve line number for each key and sort the keys by position.
	sort.Slice(list, func(i, j int) bool {
		h1 := iter.GetPath([]string{list[i]}).(*toml.Tree)
		h2 := iter.GetPath([]string{list[j]}).(*toml.Tree)
		return h1.Position().Line < h2.Position().Line
	})

	return list, nil
}

// makeStringSlice is a helper func to convert from []interface{} to []string.
// Additionally an optional cb func may be passed to perform string mapping.
func makeStringSlice(slice []interface{}, cb func(string) string) ([]string, error) {
	out := make([]string, len(slice))
	for i, value := range slice {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("unable to cast %v to string", value)
		}

		if cb != nil {
			out[i] = cb(str)
		} else {
			out[i] = str
		}
	}
	return out, nil
}

func makeAbsPath(p string, base string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// loadCertsDir loads certs from certsDir like "/etc/docker/certs.d" .
// Compatible with Docker file layout
//   - files ending with ".crt" are treated as CA certificate files
//   - files ending with ".cert" are treated as client certificates, and
//     files with the same name but ending with ".key" are treated as the
//     corresponding private key.
//     NOTE: If a ".key" file is missing, this function will just return
//     the ".cert", which may contain the private key. If the ".cert" file
//     does not contain the private key, the caller should detect and error.
func loadCertFiles(ctx context.Context, certsDir string) ([]hostConfig, error) {
	fs, err := os.ReadDir(certsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	hosts := make([]hostConfig, 1)
	for _, f := range fs {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".crt") {
			hosts[0].caCerts = append(hosts[0].caCerts, filepath.Join(certsDir, f.Name()))
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			var pair [2]string
			certFile := f.Name()
			pair[0] = filepath.Join(certsDir, certFile)
			// Check if key also exists
			keyFile := filepath.Join(certsDir, certFile[:len(certFile)-5]+".key")
			if _, err := os.Stat(keyFile); err == nil {
				pair[1] = keyFile
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			hosts[0].clientPairs = append(hosts[0].clientPairs, pair)
		}
	}
	return hosts, nil
}
