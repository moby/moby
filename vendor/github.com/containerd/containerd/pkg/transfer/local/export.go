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

package local

import (
	"context"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/pkg/transfer"
)

func (ts *localTransferService) exportStream(ctx context.Context, ig transfer.ImageGetter, is transfer.ImageExporter, tops *transfer.Config) error {
	ctx, done, err := ts.withLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: "Exporting",
		})
	}

	var imgs []images.Image
	if il, ok := ig.(transfer.ImageLookup); ok {
		imgs, err = il.Lookup(ctx, ts.images)
		if err != nil {
			return err
		}
	} else {
		img, err := ig.Get(ctx, ts.images)
		if err != nil {
			return err
		}
		imgs = append(imgs, img)
	}

	err = is.Export(ctx, ts.content, imgs)
	if err != nil {
		return err
	}

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: "Completed export",
		})
	}
	return nil
}
