// +build windows

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

package oci

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// WithImageConfig configures the spec to from the configuration of an Image
func WithImageConfig(image Image) SpecOpts {
	return func(ctx context.Context, client Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		ic, err := image.Config(ctx)
		if err != nil {
			return err
		}
		var (
			ociimage v1.Image
			config   v1.ImageConfig
		)
		switch ic.MediaType {
		case v1.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
			p, err := content.ReadBlob(ctx, image.ContentStore(), ic.Digest)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(p, &ociimage); err != nil {
				return err
			}
			config = ociimage.Config
		default:
			return fmt.Errorf("unknown image config media type %s", ic.MediaType)
		}
		s.Process.Env = config.Env
		s.Process.Args = append(config.Entrypoint, config.Cmd...)
		s.Process.User = specs.User{
			Username: config.User,
		}
		return nil
	}
}

// WithTTY sets the information on the spec as well as the environment variables for
// using a TTY
func WithTTY(width, height int) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.Terminal = true
		if s.Process.ConsoleSize == nil {
			s.Process.ConsoleSize = &specs.Box{}
		}
		s.Process.ConsoleSize.Width = uint(width)
		s.Process.ConsoleSize.Height = uint(height)
		return nil
	}
}

// WithUsername sets the username on the process
func WithUsername(username string) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.User.Username = username
		return nil
	}
}
