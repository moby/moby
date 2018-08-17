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
	"os"
	"path/filepath"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/pkg/errors"
)

// Install a binary image into the opt service
func (c *Client) Install(ctx context.Context, image Image, opts ...InstallOpts) error {
	resp, err := c.IntrospectionService().Plugins(ctx, &introspectionapi.PluginsRequest{
		Filters: []string{
			"id==opt",
		},
	})
	if err != nil {
		return err
	}
	if len(resp.Plugins) != 1 {
		return errors.New("opt service not enabled")
	}
	path := resp.Plugins[0].Exports["path"]
	if path == "" {
		return errors.New("opt path not exported")
	}
	var config InstallConfig
	for _, o := range opts {
		o(&config)
	}
	var (
		cs       = image.ContentStore()
		platform = platforms.Default()
	)
	manifest, err := images.Manifest(ctx, cs, image.Target(), platform)
	if err != nil {
		return err
	}
	for _, layer := range manifest.Layers {
		ra, err := cs.ReaderAt(ctx, layer)
		if err != nil {
			return err
		}
		cr := content.NewReader(ra)
		r, err := compression.DecompressStream(cr)
		if err != nil {
			return err
		}
		defer r.Close()
		if _, err := archive.Apply(ctx, path, r, archive.WithFilter(func(hdr *tar.Header) (bool, error) {
			d := filepath.Dir(hdr.Name)
			result := d == "bin"
			if config.Libs {
				result = result || d == "lib"
			}
			if result && !config.Replace {
				if _, err := os.Lstat(filepath.Join(path, hdr.Name)); err == nil {
					return false, errors.Errorf("cannot replace %s in %s", hdr.Name, path)
				}
			}
			return result, nil
		})); err != nil {
			return err
		}
	}
	return nil
}
