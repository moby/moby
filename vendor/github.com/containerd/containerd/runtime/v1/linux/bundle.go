// +build linux

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

package linux

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	"github.com/containerd/containerd/runtime/v1/shim"
	"github.com/containerd/containerd/runtime/v1/shim/client"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// loadBundle loads an existing bundle from disk
func loadBundle(id, path, workdir string) *bundle {
	return &bundle{
		id:      id,
		path:    path,
		workDir: workdir,
	}
}

// newBundle creates a new bundle on disk at the provided path for the given id
func newBundle(id, path, workDir string, spec []byte) (b *bundle, err error) {
	if err := os.MkdirAll(path, 0711); err != nil {
		return nil, err
	}
	path = filepath.Join(path, id)
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()
	if err := prepareBundleDirectoryPermissions(path, spec); err != nil {
		return nil, err
	}
	workDir = filepath.Join(workDir, id)
	if err := os.MkdirAll(workDir, 0711); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()
	rootfs := filepath.Join(path, "rootfs")
	if err := os.MkdirAll(rootfs, 0711); err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filepath.Join(path, configFilename), spec, 0666)
	return &bundle{
		id:      id,
		path:    path,
		workDir: workDir,
	}, err
}

// prepareBundleDirectoryPermissions prepares the permissions of the bundle
// directory. When user namespaces are enabled, the permissions are modified
// to allow the remapped root GID to access the bundle.
func prepareBundleDirectoryPermissions(path string, spec []byte) error {
	gid, err := remappedGID(spec)
	if err != nil {
		return err
	}
	if gid == 0 {
		return nil
	}
	if err := os.Chown(path, -1, int(gid)); err != nil {
		return err
	}
	return os.Chmod(path, 0710)
}

// ociSpecUserNS is a subset of specs.Spec used to reduce garbage during
// unmarshal.
type ociSpecUserNS struct {
	Linux *linuxSpecUserNS
}

// linuxSpecUserNS is a subset of specs.Linux used to reduce garbage during
// unmarshal.
type linuxSpecUserNS struct {
	GIDMappings []specs.LinuxIDMapping
}

// remappedGID reads the remapped GID 0 from the OCI spec, if it exists. If
// there is no remapping, remappedGID returns 0. If the spec cannot be parsed,
// remappedGID returns an error.
func remappedGID(spec []byte) (uint32, error) {
	var ociSpec ociSpecUserNS
	err := json.Unmarshal(spec, &ociSpec)
	if err != nil {
		return 0, err
	}
	if ociSpec.Linux == nil || len(ociSpec.Linux.GIDMappings) == 0 {
		return 0, nil
	}
	for _, mapping := range ociSpec.Linux.GIDMappings {
		if mapping.ContainerID == 0 {
			return mapping.HostID, nil
		}
	}
	return 0, nil
}

type bundle struct {
	id      string
	path    string
	workDir string
}

// ShimOpt specifies shim options for initialization and connection
type ShimOpt func(*bundle, string, *runctypes.RuncOptions) (shim.Config, client.Opt)

// ShimRemote is a ShimOpt for connecting and starting a remote shim
func ShimRemote(c *Config, daemonAddress, cgroup string, exitHandler func()) ShimOpt {
	return func(b *bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		config := b.shimConfig(ns, c, ropts)
		return config,
			client.WithStart(c.Shim, b.shimAddress(ns, daemonAddress), daemonAddress, cgroup, c.ShimDebug, exitHandler)
	}
}

// ShimLocal is a ShimOpt for using an in process shim implementation
func ShimLocal(c *Config, exchange *exchange.Exchange) ShimOpt {
	return func(b *bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		return b.shimConfig(ns, c, ropts), client.WithLocal(exchange)
	}
}

// ShimConnect is a ShimOpt for connecting to an existing remote shim
func ShimConnect(c *Config, onClose func()) ShimOpt {
	return func(b *bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		return b.shimConfig(ns, c, ropts), client.WithConnect(b.decideShimAddress(ns), onClose)
	}
}

// NewShimClient connects to the shim managing the bundle and tasks creating it if needed
func (b *bundle) NewShimClient(ctx context.Context, namespace string, getClientOpts ShimOpt, runcOpts *runctypes.RuncOptions) (*client.Client, error) {
	cfg, opt := getClientOpts(b, namespace, runcOpts)
	return client.New(ctx, cfg, opt)
}

// Delete deletes the bundle from disk
func (b *bundle) Delete() error {
	address, _ := b.loadAddress()
	if address != "" {
		// we don't care about errors here
		client.RemoveSocket(address)
	}
	err := atomicDelete(b.path)
	if err == nil {
		return atomicDelete(b.workDir)
	}
	// error removing the bundle path; still attempt removing work dir
	err2 := atomicDelete(b.workDir)
	if err2 == nil {
		return err
	}
	return errors.Wrapf(err, "Failed to remove both bundle and workdir locations: %v", err2)
}

func (b *bundle) legacyShimAddress(namespace string) string {
	return filepath.Join(string(filepath.Separator), "containerd-shim", namespace, b.id, "shim.sock")
}

const socketRoot = "/run/containerd"

func (b *bundle) shimAddress(namespace, socketPath string) string {
	d := sha256.Sum256([]byte(filepath.Join(socketPath, namespace, b.id)))
	return fmt.Sprintf("unix://%s/%x", filepath.Join(socketRoot, "s"), d)
}

func (b *bundle) loadAddress() (string, error) {
	addressPath := filepath.Join(b.path, "address")
	data, err := ioutil.ReadFile(addressPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (b *bundle) decideShimAddress(namespace string) string {
	address, err := b.loadAddress()
	if err != nil {
		return b.legacyShimAddress(namespace)
	}
	return address
}

func (b *bundle) shimConfig(namespace string, c *Config, runcOptions *runctypes.RuncOptions) shim.Config {
	var (
		criuPath      string
		runtimeRoot   = c.RuntimeRoot
		systemdCgroup bool
	)
	if runcOptions != nil {
		criuPath = runcOptions.CriuPath
		systemdCgroup = runcOptions.SystemdCgroup
		if runcOptions.RuntimeRoot != "" {
			runtimeRoot = runcOptions.RuntimeRoot
		}
	}
	return shim.Config{
		Path:          b.path,
		WorkDir:       b.workDir,
		Namespace:     namespace,
		Criu:          criuPath,
		RuntimeRoot:   runtimeRoot,
		SystemdCgroup: systemdCgroup,
	}
}

// atomicDelete renames the path to a hidden file before removal
func atomicDelete(path string) error {
	// create a hidden dir for an atomic removal
	atomicPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s", filepath.Base(path)))
	if err := os.Rename(path, atomicPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(atomicPath)
}
