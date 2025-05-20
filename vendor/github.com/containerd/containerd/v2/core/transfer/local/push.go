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
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (ts *localTransferService) push(ctx context.Context, ig transfer.ImageGetter, p transfer.ImagePusher, tops *transfer.Config) error {
	matcher := platforms.All
	if ipg, ok := ig.(transfer.ImagePlatformsGetter); ok {
		if ps := ipg.Platforms(); len(ps) > 0 {
			matcher = platforms.Any(ps...)
		}
	}

	img, err := ig.Get(ctx, ts.images)
	if err != nil {
		return err
	}

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: fmt.Sprintf("Pushing to %s", p),
		})
		tops.Progress(transfer.Progress{
			Event: "pushing content",
			Name:  img.Name,
			//Digest: img.Target.Digest.String(),
			Desc: &img.Target,
		})
	}

	var pusher remotes.Pusher
	pusher, err = p.Pusher(ctx, img.Target)
	if err != nil {
		return err
	}

	var wrapper func(images.Handler) images.Handler

	ctx, cancel := context.WithCancel(ctx)
	if tops.Progress != nil {
		progressTracker := NewProgressTracker(img.Name, "uploading") //Pass in first name as root

		p := newProgressPusher(pusher, progressTracker)
		go progressTracker.HandleProgress(ctx, tops.Progress, p)
		defer progressTracker.Wait()
		wrapper = p.WrapHandler
		pusher = p
	}
	defer cancel()

	// TODO: Add handler to track parents
	/*
		// TODO: Add handlers
		if len(pushCtx.BaseHandlers) > 0 {
			wrapper = func(h images.Handler) images.Handler {
				h = images.Handlers(append(pushCtx.BaseHandlers, h)...)
				if pushCtx.HandlerWrapper != nil {
					h = pushCtx.HandlerWrapper(h)
				}
				return h
			}
		} else if pushCtx.HandlerWrapper != nil {
			wrapper = pushCtx.HandlerWrapper
		}
	*/
	if err := remotes.PushContent(ctx, pusher, img.Target, ts.content, ts.limiterU, matcher, wrapper); err != nil {
		return err
	}
	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: "pushed content",
			Name:  img.Name,
			//Digest: img.Target.Digest.String(),
			Desc: &img.Target,
		})
		tops.Progress(transfer.Progress{
			Event: fmt.Sprintf("Completed push to %s", p),
			Desc:  &img.Target,
		})
	}

	return nil
}

type progressPusher struct {
	remotes.Pusher
	progress *ProgressTracker

	status *pushStatus
}

type pushStatus struct {
	l        sync.Mutex
	statuses map[string]content.Status
	complete map[digest.Digest]struct{}
}

func newProgressPusher(pusher remotes.Pusher, progress *ProgressTracker) *progressPusher {
	return &progressPusher{
		Pusher:   pusher,
		progress: progress,
		status: &pushStatus{
			statuses: map[string]content.Status{},
			complete: map[digest.Digest]struct{}{},
		},
	}

}

func (p *progressPusher) WrapHandler(h images.Handler) images.Handler {
	return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
		p.progress.Add(desc)
		subdescs, err = h.Handle(ctx, desc)
		p.progress.AddChildren(desc, subdescs)
		return
	})
}

func (p *progressPusher) Push(ctx context.Context, d ocispec.Descriptor) (content.Writer, error) {
	ref := remotes.MakeRefKey(ctx, d)
	p.status.add(ref, d)
	var cw content.Writer
	var err error
	if cs, ok := p.Pusher.(content.Ingester); ok {
		cw, err = content.OpenWriter(ctx, cs, content.WithRef(ref), content.WithDescriptor(d))
	} else {
		cw, err = p.Pusher.Push(ctx, d)
	}
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			p.progress.MarkExists(d)
			p.status.markComplete(ref, d)
		}
		return nil, err
	}

	return &progressWriter{
		Writer:   cw,
		ref:      ref,
		desc:     d,
		status:   p.status,
		progress: p.progress,
	}, nil
}

func (ps *pushStatus) update(ref string, delta int) {
	ps.l.Lock()
	status, ok := ps.statuses[ref]
	if ok {
		if delta > 0 {
			status.Offset += int64(delta)
		} else if delta < 0 {
			status.Offset = 0
		}
		ps.statuses[ref] = status
	}
	ps.l.Unlock()
}

func (ps *pushStatus) add(ref string, d ocispec.Descriptor) {
	status := content.Status{
		Ref:       ref,
		Offset:    0,
		Total:     d.Size,
		StartedAt: time.Now(),
	}
	ps.l.Lock()
	_, ok := ps.statuses[ref]
	_, complete := ps.complete[d.Digest]
	if !ok && !complete {
		ps.statuses[ref] = status
	}
	ps.l.Unlock()
}
func (ps *pushStatus) markComplete(ref string, d ocispec.Descriptor) {
	ps.l.Lock()
	_, ok := ps.statuses[ref]
	if ok {
		delete(ps.statuses, ref)
	}
	ps.complete[d.Digest] = struct{}{}
	ps.l.Unlock()

}

func (ps *pushStatus) Status(name string) (content.Status, bool) {
	ps.l.Lock()
	status, ok := ps.statuses[name]
	ps.l.Unlock()
	return status, ok
}

func (ps *pushStatus) Check(ctx context.Context, dgst digest.Digest) (bool, error) {
	ps.l.Lock()
	_, ok := ps.complete[dgst]
	ps.l.Unlock()
	return ok, nil
}

func (p *progressPusher) Active(ctx context.Context, _ ...string) (ActiveJobs, error) {
	return p.status, nil
}

func (p *progressPusher) Check(ctx context.Context, dgst digest.Digest) (bool, error) {
	return p.status.Check(ctx, dgst)
}

type progressWriter struct {
	content.Writer
	ref      string
	desc     ocispec.Descriptor
	status   *pushStatus
	progress *ProgressTracker
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.Writer.Write(p)
	if err != nil {
		// TODO: Handle reset error to reset progress
		return
	}
	pw.status.update(pw.ref, n)
	return
}
func (pw *progressWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	err := pw.Writer.Commit(ctx, size, expected, opts...)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			pw.progress.MarkExists(pw.desc)
		}
		// TODO: Handle reset error to reset progress
	}
	pw.status.markComplete(pw.ref, pw.desc)
	return err
}
