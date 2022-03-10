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

package containerd

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
)

// Install a binary image into the opt service
func (c *Client) Install(ctx context.Context, image Image, opts ...InstallOpts) error {
	var config InstallConfig
	for _, o := range opts {
		o(&config)
	}
	path, err := c.getInstallPath(ctx, config)
	if err != nil {
		return err
	}
	var (
		cs       = image.ContentStore()
		platform = c.platform
	)
	manifest, err := images.Manifest(ctx, cs, image.Target(), platform)
	if err != nil {
		return err
	}

	var binDir, libDir string
	if runtime.GOOS == "windows" {
		binDir = "Files\\bin"
		libDir = "Files\\lib"
	} else {
		binDir = "bin"
		libDir = "lib"
	}
	for _, layer := range manifest.Layers {
		ra, err := cs.ReaderAt(ctx, layer)
		if err != nil {
			return err
		}
		cr := content.NewReader(ra)
		r, err := compression.DecompressStream(cr)
		if err != nil {
			ra.Close()
			return err
		}
		if _, err := archive.Apply(ctx, path, r, archive.WithFilter(func(hdr *tar.Header) (bool, error) {
			d := filepath.Dir(hdr.Name)
			result := d == binDir

			if config.Libs {
				result = result || d == libDir
			}

			if runtime.GOOS == "windows" {
				hdr.Name = strings.Replace(hdr.Name, "Files", "", 1)
			}
			if result && !config.Replace {
				if _, err := os.Lstat(filepath.Join(path, hdr.Name)); err == nil {
					return false, fmt.Errorf("cannot replace %s in %s", hdr.Name, path)
				}
			}
			return result, nil
		})); err != nil {
			r.Close()
			ra.Close()
			return err
		}
		r.Close()
		ra.Close()
	}
	return nil
}

func (c *Client) getInstallPath(ctx context.Context, config InstallConfig) (string, error) {
	if config.Path != "" {
		return config.Path, nil
	}
	filters := []string{"id==opt"}
	resp, err := c.IntrospectionService().Plugins(ctx, filters)
	if err != nil {
		return "", err
	}
	if len(resp.Plugins) != 1 {
		return "", errors.New("opt service not enabled")
	}
	path := resp.Plugins[0].Exports["path"]
	if path == "" {
		return "", errors.New("opt path not exported")
	}
	return path, nil
}
