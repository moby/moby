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

package proxy

import (
	"context"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	digest "github.com/opencontainers/go-digest"
)

type remoteReaderAt struct {
	ctx    context.Context
	digest digest.Digest
	size   int64
	client contentapi.TTRPCContentClient
}

func (ra *remoteReaderAt) Size() int64 {
	return ra.size
}

func (ra *remoteReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	rr := &contentapi.ReadContentRequest{
		Digest: ra.digest.String(),
		Offset: off,
		Size:   int64(len(p)),
	}
	// we need a child context with cancel, or the eventually called
	// grpc.NewStream will leak the goroutine until the whole thing is cleared.
	// See comment at https://godoc.org/google.golang.org/grpc#ClientConn.NewStream
	childCtx, cancel := context.WithCancel(ra.ctx)
	// we MUST cancel the child context; see comment above
	defer cancel()
	rc, err := ra.client.Read(childCtx, rr)
	if err != nil {
		return 0, err
	}

	for len(p) > 0 {
		var resp *contentapi.ReadContentResponse
		// fill our buffer up until we can fill p.
		resp, err = rc.Recv()
		if err != nil {
			return n, err
		}

		copied := copy(p, resp.Data)
		n += copied
		p = p[copied:]
	}
	return n, nil
}

func (ra *remoteReaderAt) Close() error {
	return nil
}
