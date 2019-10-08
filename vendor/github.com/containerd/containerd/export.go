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
	"context"
	"io"

	"github.com/containerd/containerd/images/archive"
)

// Export exports images to a Tar stream.
// The tar archive is in OCI format with a Docker compatible manifest
// when a single target platform is given.
func (c *Client) Export(ctx context.Context, w io.Writer, opts ...archive.ExportOpt) error {
	return archive.Export(ctx, c.ContentStore(), w, opts...)
}
