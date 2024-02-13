package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/containerd/remotes/docker"
	hostconfig "github.com/containerd/containerd/remotes/docker/config"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/registry"
	"github.com/pkg/errors"
)

const (
	defaultPath = "/v2"
)

// RegistryHosts returns the registry hosts configuration for the host component
// of a distribution image reference.
func (daemon *Daemon) RegistryHosts(host string) ([]docker.RegistryHost, error) {
	hosts, err := hostconfig.ConfigureHosts(context.Background(), hostconfig.HostOptions{
		// TODO: Also check containerd path when updating containerd to use multiple host directories
		HostDir: hostconfig.HostDirFromRoot(registry.CertsDir()),
	})(host)
	if err != nil {
		return nil, err
	}

	// Merge in legacy configuration if provided and only a single configuration
	if sc := daemon.registryService.ServiceConfig(); len(hosts) == 1 && sc != nil {
		hosts, err = daemon.mergeLegacyConfig(host, hosts)
		if err != nil {
			return nil, err
		}
	}

	return hosts, nil
}

func (daemon *Daemon) mergeLegacyConfig(host string, hosts []docker.RegistryHost) ([]docker.RegistryHost, error) {
	if len(hosts) == 0 {
		return hosts, nil
	}
	sc := daemon.registryService.ServiceConfig()
	if host == "docker.io" && len(sc.Mirrors) > 0 {
		var mirrorHosts []docker.RegistryHost
		for _, mirror := range sc.Mirrors {
			h := hosts[0]
			h.Capabilities = docker.HostCapabilityPull | docker.HostCapabilityResolve

			u, err := url.Parse(mirror)
			if err != nil || u.Host == "" {
				u, err = url.Parse(fmt.Sprintf("//%s", mirror))
			}
			if err == nil && u.Host != "" {
				h.Host = u.Host
				h.Path = strings.TrimRight(u.Path, "/")
				if !strings.HasSuffix(h.Path, defaultPath) {
					h.Path = path.Join(defaultPath, h.Path)
				}
			} else {
				h.Host = mirror
				h.Path = defaultPath
			}

			mirrorHosts = append(mirrorHosts, h)
		}
		hosts = append(mirrorHosts, hosts[0])
	}
	hostDir := hostconfig.HostDirFromRoot(registry.CertsDir())
	for i := range hosts {
		t, ok := hosts[i].Client.Transport.(*http.Transport)
		if !ok {
			continue
		}
		if t.TLSClientConfig == nil {
			certsDir, err := hostDir(host)
			if err != nil && !cerrdefs.IsNotFound(err) {
				return nil, err
			} else if err == nil {
				c, err := loadTLSConfig(certsDir)
				if err != nil {
					return nil, err
				}
				t.TLSClientConfig = c
			}
		}
		if daemon.registryService.IsInsecureRegistry(hosts[i].Host) {
			if t.TLSClientConfig != nil {
				isLocalhost, err := docker.MatchLocalhost(hosts[i].Host)
				if err != nil {
					continue
				}
				if isLocalhost {
					hosts[i].Client.Transport = docker.NewHTTPFallback(hosts[i].Client.Transport)
				}
				t.TLSClientConfig.InsecureSkipVerify = true
			} else {
				hosts[i].Scheme = "http"
			}
		}
	}
	return hosts, nil
}

func loadTLSConfig(d string) (*tls.Config, error) {
	fs, err := os.ReadDir(d)
	if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrPermission) {
		return nil, errors.WithStack(err)
	}
	type keyPair struct {
		Certificate string
		Key         string
	}
	var (
		rootCAs  []string
		keyPairs []keyPair
	)
	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".crt") {
			rootCAs = append(rootCAs, filepath.Join(d, f.Name()))
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			keyPairs = append(keyPairs, keyPair{
				Certificate: filepath.Join(d, f.Name()),
				Key:         filepath.Join(d, strings.TrimSuffix(f.Name(), ".cert")+".key"),
			})
		}
	}

	tc := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if len(rootCAs) > 0 {
		systemPool, err := x509.SystemCertPool()
		if err != nil {
			if runtime.GOOS == "windows" {
				systemPool = x509.NewCertPool()
			} else {
				return nil, errors.Wrapf(err, "unable to get system cert pool")
			}
		}
		tc.RootCAs = systemPool
	}

	for _, p := range rootCAs {
		dt, err := os.ReadFile(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", p)
		}
		tc.RootCAs.AppendCertsFromPEM(dt)
	}

	for _, kp := range keyPairs {
		cert, err := tls.LoadX509KeyPair(kp.Certificate, kp.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load keypair for %s", kp.Certificate)
		}
		tc.Certificates = append(tc.Certificates, cert)
	}
	return tc, nil
}
