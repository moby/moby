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

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	remoteserrors "github.com/containerd/containerd/remotes/errors"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerPusher struct {
	*dockerBase
	object string

	// TODO: namespace tracker
	tracker StatusTracker
}

// Writer implements Ingester API of content store. This allows the client
// to receive ErrUnavailable when there is already an on-going upload.
// Note that the tracker MUST implement StatusTrackLocker interface to avoid
// race condition on StatusTracker.
func (p dockerPusher) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return nil, err
		}
	}
	if wOpts.Ref == "" {
		return nil, fmt.Errorf("ref must not be empty: %w", errdefs.ErrInvalidArgument)
	}
	return p.push(ctx, wOpts.Desc, wOpts.Ref, true)
}

func (p dockerPusher) Push(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	return p.push(ctx, desc, remotes.MakeRefKey(ctx, desc), false)
}

func (p dockerPusher) push(ctx context.Context, desc ocispec.Descriptor, ref string, unavailableOnFail bool) (content.Writer, error) {
	if l, ok := p.tracker.(StatusTrackLocker); ok {
		l.Lock(ref)
		defer l.Unlock(ref)
	}
	ctx, err := ContextWithRepositoryScope(ctx, p.refspec, true)
	if err != nil {
		return nil, err
	}
	status, err := p.tracker.GetStatus(ref)
	if err == nil {
		if status.Committed && status.Offset == status.Total {
			return nil, fmt.Errorf("ref %v: %w", ref, errdefs.ErrAlreadyExists)
		}
		if unavailableOnFail && status.ErrClosed == nil {
			// Another push of this ref is happening elsewhere. The rest of function
			// will continue only when `errdefs.IsNotFound(err) == true` (i.e. there
			// is no actively-tracked ref already).
			return nil, fmt.Errorf("push is on-going: %w", errdefs.ErrUnavailable)
		}
		// TODO: Handle incomplete status
	} else if !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	hosts := p.filterHosts(HostCapabilityPush)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no push hosts: %w", errdefs.ErrNotFound)
	}

	var (
		isManifest bool
		existCheck []string
		host       = hosts[0]
	)

	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, images.MediaTypeDockerSchema2ManifestList,
		ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex:
		isManifest = true
		existCheck = getManifestPath(p.object, desc.Digest)
	default:
		existCheck = []string{"blobs", desc.Digest.String()}
	}

	req := p.request(host, http.MethodHead, existCheck...)
	req.header.Set("Accept", strings.Join([]string{desc.MediaType, `*/*`}, ", "))

	log.G(ctx).WithField("url", req.String()).Debugf("checking and pushing to")

	resp, err := req.doWithRetries(ctx, nil)
	if err != nil {
		if !errors.Is(err, ErrInvalidAuthorization) {
			return nil, err
		}
		log.G(ctx).WithError(err).Debugf("Unable to check existence, continuing with push")
	} else {
		if resp.StatusCode == http.StatusOK {
			var exists bool
			if isManifest && existCheck[1] != desc.Digest.String() {
				dgstHeader := digest.Digest(resp.Header.Get("Docker-Content-Digest"))
				if dgstHeader == desc.Digest {
					exists = true
				}
			} else {
				exists = true
			}

			if exists {
				p.tracker.SetStatus(ref, Status{
					Committed: true,
					Status: content.Status{
						Ref:    ref,
						Total:  desc.Size,
						Offset: desc.Size,
						// TODO: Set updated time?
					},
				})
				resp.Body.Close()
				return nil, fmt.Errorf("content %v on remote: %w", desc.Digest, errdefs.ErrAlreadyExists)
			}
		} else if resp.StatusCode != http.StatusNotFound {
			err := remoteserrors.NewUnexpectedStatusErr(resp)
			log.G(ctx).WithField("resp", resp).WithField("body", string(err.(remoteserrors.ErrUnexpectedStatus).Body)).Debug("unexpected response")
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
	}

	if isManifest {
		putPath := getManifestPath(p.object, desc.Digest)
		req = p.request(host, http.MethodPut, putPath...)
		req.header.Add("Content-Type", desc.MediaType)
	} else {
		// Start upload request
		req = p.request(host, http.MethodPost, "blobs", "uploads/")

		var resp *http.Response
		if fromRepo := selectRepositoryMountCandidate(p.refspec, desc.Annotations); fromRepo != "" {
			preq := requestWithMountFrom(req, desc.Digest.String(), fromRepo)
			pctx := ContextWithAppendPullRepositoryScope(ctx, fromRepo)

			// NOTE: the fromRepo might be private repo and
			// auth service still can grant token without error.
			// but the post request will fail because of 401.
			//
			// for the private repo, we should remove mount-from
			// query and send the request again.
			resp, err = preq.doWithRetries(pctx, nil)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode == http.StatusUnauthorized {
				log.G(ctx).Debugf("failed to mount from repository %s", fromRepo)

				resp.Body.Close()
				resp = nil
			}
		}

		if resp == nil {
			resp, err = req.doWithRetries(ctx, nil)
			if err != nil {
				return nil, err
			}
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusAccepted, http.StatusNoContent:
		case http.StatusCreated:
			p.tracker.SetStatus(ref, Status{
				Committed: true,
				Status: content.Status{
					Ref:    ref,
					Total:  desc.Size,
					Offset: desc.Size,
				},
			})
			return nil, fmt.Errorf("content %v on remote: %w", desc.Digest, errdefs.ErrAlreadyExists)
		default:
			err := remoteserrors.NewUnexpectedStatusErr(resp)
			log.G(ctx).WithField("resp", resp).WithField("body", string(err.(remoteserrors.ErrUnexpectedStatus).Body)).Debug("unexpected response")
			return nil, err
		}

		var (
			location = resp.Header.Get("Location")
			lurl     *url.URL
			lhost    = host
		)
		// Support paths without host in location
		if strings.HasPrefix(location, "/") {
			lurl, err = url.Parse(lhost.Scheme + "://" + lhost.Host + location)
			if err != nil {
				return nil, fmt.Errorf("unable to parse location %v: %w", location, err)
			}
		} else {
			if !strings.Contains(location, "://") {
				location = lhost.Scheme + "://" + location
			}
			lurl, err = url.Parse(location)
			if err != nil {
				return nil, fmt.Errorf("unable to parse location %v: %w", location, err)
			}

			if lurl.Host != lhost.Host || lhost.Scheme != lurl.Scheme {

				lhost.Scheme = lurl.Scheme
				lhost.Host = lurl.Host
				log.G(ctx).WithField("host", lhost.Host).WithField("scheme", lhost.Scheme).Debug("upload changed destination")

				// Strip authorizer if change to host or scheme
				lhost.Authorizer = nil
			}
		}
		q := lurl.Query()
		q.Add("digest", desc.Digest.String())

		req = p.request(lhost, http.MethodPut)
		req.header.Set("Content-Type", "application/octet-stream")
		req.path = lurl.Path + "?" + q.Encode()
	}
	p.tracker.SetStatus(ref, Status{
		Status: content.Status{
			Ref:       ref,
			Total:     desc.Size,
			Expected:  desc.Digest,
			StartedAt: time.Now(),
		},
	})

	// TODO: Support chunked upload

	pushw := newPushWriter(p.dockerBase, ref, desc.Digest, p.tracker, isManifest)

	req.body = func() (io.ReadCloser, error) {
		pr, pw := io.Pipe()
		pushw.setPipe(pw)
		return io.NopCloser(pr), nil
	}
	req.size = desc.Size

	go func() {
		resp, err := req.doWithRetries(ctx, nil)
		if err != nil {
			pushw.setError(err)
			pushw.Close()
			return
		}

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		default:
			err := remoteserrors.NewUnexpectedStatusErr(resp)
			log.G(ctx).WithField("resp", resp).WithField("body", string(err.(remoteserrors.ErrUnexpectedStatus).Body)).Debug("unexpected response")
			pushw.setError(err)
			pushw.Close()
		}
		pushw.setResponse(resp)
	}()

	return pushw, nil
}

func getManifestPath(object string, dgst digest.Digest) []string {
	if i := strings.IndexByte(object, '@'); i >= 0 {
		if object[i+1:] != dgst.String() {
			// use digest, not tag
			object = ""
		} else {
			// strip @<digest> for registry path to make tag
			object = object[:i]
		}

	}

	if object == "" {
		return []string{"manifests", dgst.String()}
	}

	return []string{"manifests", object}
}

type pushWriter struct {
	base *dockerBase
	ref  string

	pipe *io.PipeWriter

	pipeC     chan *io.PipeWriter
	respC     chan *http.Response
	closeOnce sync.Once
	errC      chan error

	isManifest bool

	expected digest.Digest
	tracker  StatusTracker
}

func newPushWriter(db *dockerBase, ref string, expected digest.Digest, tracker StatusTracker, isManifest bool) *pushWriter {
	// Initialize and create response
	return &pushWriter{
		base:       db,
		ref:        ref,
		expected:   expected,
		tracker:    tracker,
		pipeC:      make(chan *io.PipeWriter, 1),
		respC:      make(chan *http.Response, 1),
		errC:       make(chan error, 1),
		isManifest: isManifest,
	}
}

func (pw *pushWriter) setPipe(p *io.PipeWriter) {
	pw.pipeC <- p
}

func (pw *pushWriter) setError(err error) {
	pw.errC <- err
}
func (pw *pushWriter) setResponse(resp *http.Response) {
	pw.respC <- resp
}

func (pw *pushWriter) Write(p []byte) (n int, err error) {
	status, err := pw.tracker.GetStatus(pw.ref)
	if err != nil {
		return n, err
	}

	if pw.pipe == nil {
		p, ok := <-pw.pipeC
		if !ok {
			return 0, io.ErrClosedPipe
		}
		pw.pipe = p
	} else {
		select {
		case p, ok := <-pw.pipeC:
			if !ok {
				return 0, io.ErrClosedPipe
			}
			pw.pipe.CloseWithError(content.ErrReset)
			pw.pipe = p

			// If content has already been written, the bytes
			// cannot be written and the caller must reset
			if status.Offset > 0 {
				status.Offset = 0
				status.UpdatedAt = time.Now()
				pw.tracker.SetStatus(pw.ref, status)
				return 0, content.ErrReset
			}
		default:
		}
	}

	n, err = pw.pipe.Write(p)
	status.Offset += int64(n)
	status.UpdatedAt = time.Now()
	pw.tracker.SetStatus(pw.ref, status)
	return
}

func (pw *pushWriter) Close() error {
	// Ensure pipeC is closed but handle `Close()` being
	// called multiple times without panicking
	pw.closeOnce.Do(func() {
		close(pw.pipeC)
	})
	if pw.pipe != nil {
		status, err := pw.tracker.GetStatus(pw.ref)
		if err == nil && !status.Committed {
			// Closing an incomplete writer. Record this as an error so that following write can retry it.
			status.ErrClosed = errors.New("closed incomplete writer")
			pw.tracker.SetStatus(pw.ref, status)
		}
		return pw.pipe.Close()
	}
	return nil
}

func (pw *pushWriter) Status() (content.Status, error) {
	status, err := pw.tracker.GetStatus(pw.ref)
	if err != nil {
		return content.Status{}, err
	}
	return status.Status, nil

}

func (pw *pushWriter) Digest() digest.Digest {
	// TODO: Get rid of this function?
	return pw.expected
}

func (pw *pushWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	// Check whether read has already thrown an error
	if _, err := pw.pipe.Write([]byte{}); err != nil && err != io.ErrClosedPipe {
		return fmt.Errorf("pipe error before commit: %w", err)
	}

	if err := pw.pipe.Close(); err != nil {
		return err
	}
	// TODO: timeout waiting for response
	var resp *http.Response
	select {
	case err := <-pw.errC:
		if err != nil {
			return err
		}
	case resp = <-pw.respC:
		defer resp.Body.Close()
	case p, ok := <-pw.pipeC:
		// check whether the pipe has changed in the commit, because sometimes Write
		// can complete successfully, but the pipe may have changed. In that case, the
		// content needs to be reset.
		if !ok {
			return io.ErrClosedPipe
		}
		pw.pipe.CloseWithError(content.ErrReset)
		pw.pipe = p
		status, err := pw.tracker.GetStatus(pw.ref)
		if err != nil {
			return err
		}
		// If content has already been written, the bytes
		// cannot be written again and the caller must reset
		if status.Offset > 0 {
			status.Offset = 0
			status.UpdatedAt = time.Now()
			pw.tracker.SetStatus(pw.ref, status)
			return content.ErrReset
		}
	}

	// 201 is specified return status, some registries return
	// 200, 202 or 204.
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusAccepted:
	default:
		return remoteserrors.NewUnexpectedStatusErr(resp)
	}

	status, err := pw.tracker.GetStatus(pw.ref)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if size > 0 && size != status.Offset {
		return fmt.Errorf("unexpected size %d, expected %d", status.Offset, size)
	}

	if expected == "" {
		expected = status.Expected
	}

	actual, err := digest.Parse(resp.Header.Get("Docker-Content-Digest"))
	if err != nil {
		return fmt.Errorf("invalid content digest in response: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("got digest %s, expected %s", actual, expected)
	}

	status.Committed = true
	status.UpdatedAt = time.Now()
	pw.tracker.SetStatus(pw.ref, status)

	return nil
}

func (pw *pushWriter) Truncate(size int64) error {
	// TODO: if blob close request and start new request at offset
	// TODO: always error on manifest
	return errors.New("cannot truncate remote upload")
}

func requestWithMountFrom(req *request, mount, from string) *request {
	creq := *req

	sep := "?"
	if strings.Contains(creq.path, sep) {
		sep = "&"
	}

	creq.path = creq.path + sep + "mount=" + mount + "&from=" + from

	return &creq
}
