package client

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ContentStoreSession struct {
	sessionId string
	cli       *Client
}

func (c ContentStoreSession) Close() error {
	query := url.Values{}
	query.Set("session", c.sessionId)
	resp, err := c.cli.delete(context.Background(), "/contentstore", query, nil)
	defer ensureReaderClosed(resp)

	return err
}

func (c ContentStoreSession) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	query := url.Values{}
	query.Set("session", c.sessionId)
	query.Set("digest", dgst.String())
	resp, err := c.cli.post(ctx, "/contentstore/info", query, nil, nil)
	if err != nil {
		return content.Info{}, err
	}
	defer ensureReaderClosed(resp)

	var info content.Info
	if err := json.NewDecoder(resp.body).Decode(&info); err != nil {
		return content.Info{}, err
	}
	return info, nil
}

type readerAt struct {
	descJson string
	size     int64
	sess     *ContentStoreSession
}

func (r *readerAt) Close() error {
	return nil
}

func (r *readerAt) Size() int64 {
	return r.size
}

func (r *readerAt) getSize() (int64, error) {
	query := url.Values{}
	query.Set("session", r.sess.sessionId)
	query.Set("descriptor", r.descJson)

	resp, err := r.sess.cli.get(context.Background(), "/contentstore/size", query, nil)
	if err != nil {
		return -1, err
	}
	defer ensureReaderClosed(resp)

	var out struct {
		Size int64 `json:"size"`
	}
	if err := json.NewDecoder(resp.body).Decode(&out); err != nil {
		return -1, err
	}
	return out.Size, nil
}

func (r *readerAt) ReadAt(p []byte, off int64) (n int, err error) {
	query := url.Values{}
	query.Set("session", r.sess.sessionId)
	query.Set("descriptor", r.descJson)

	headers := http.Header{}
	end := off + int64(len(p))
	if end > r.size {
		end = r.size
	}
	headers.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))

	resp, err := r.sess.cli.get(context.Background(), "/contentstore/read", query, headers)
	if err != nil {
		return -1, err
	}
	defer ensureReaderClosed(resp)

	return io.ReadFull(resp.body, p)
}

var _ content.ReaderAt = &readerAt{}

func (c ContentStoreSession) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	descJson, err := json.Marshal(desc)
	if err != nil {
		return nil, nil
	}

	ra := &readerAt{
		descJson: string(descJson),
		sess:     &c,
	}
	size, err := ra.getSize()
	if err != nil {
		return nil, err
	}
	ra.size = size
	return ra, nil
}

func (c ContentStoreSession) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	query := url.Values{}
	query.Set("session", c.sessionId)

	var writerOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&writerOpts); err != nil {
			return nil, err
		}
	}

	if writerOpts.Ref != "" {
		query.Set("ref", writerOpts.Ref)
	}

	if writerOpts.Desc.Digest != "" {
		descJson, err := json.Marshal(writerOpts.Desc)
		if err != nil {
			return nil, err
		}
		query.Set("descriptor", string(descJson))
	}

	resp, err := c.cli.post(ctx, "/contentstore/writer", query, nil, nil)
	if err != nil {
		return nil, err
	}
	defer ensureReaderClosed(resp)

	var response struct {
		Writer string `json:"writer"`
	}

	if err := json.NewDecoder(resp.body).Decode(&response); err != nil {
		return nil, err
	}

	return &writer{
		session: &c,
		writer:  response.Writer,
		h:       crypto.SHA256.New(),
	}, nil
}

type writer struct {
	session *ContentStoreSession
	writer  string
	h       hash.Hash
}

func (w *writer) Write(p []byte) (n int, err error) {
	ctx := context.Background()

	query := url.Values{}
	query.Set("session", w.session.sessionId)
	query.Set("writer", w.writer)

	rd := io.TeeReader(bytes.NewReader(p), w.h)

	headers := http.Header{}
	headers.Set("Content-Type", "application/octet-stream")
	headers.Set("Content-Length", strconv.Itoa(len(p)))

	resp, err := w.session.cli.sendRequest(ctx, http.MethodPost, "/contentstore/writer/write", query, rd, headers)
	if err != nil {
		return -1, err
	}
	defer ensureReaderClosed(resp)

	var out struct {
		N int `json:"n"`
	}

	if err := json.NewDecoder(resp.body).Decode(&out); err != nil {
		return -1, err
	}

	return out.N, nil
}

func (w *writer) Close() error {
	ctx := context.Background()

	query := url.Values{}
	query.Set("session", w.session.sessionId)
	query.Set("writer", w.writer)

	resp, err := w.session.cli.delete(ctx, "/contentstore/writer", query, nil)
	defer ensureReaderClosed(resp)

	return err
}

func (w *writer) Digest() digest.Digest {
	return digest.NewDigest("sha256", w.h)
}

func (w *writer) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	query := url.Values{}
	query.Set("session", w.session.sessionId)
	query.Set("writer", w.writer)
	query.Set("size", strconv.FormatInt(size, 10))
	query.Set("expected", expected.String())

	resp, err := w.session.cli.post(ctx, "/contentstore/writer/commit", query, nil, nil)
	if err != nil {
		return err
	}
	defer ensureReaderClosed(resp)

	_, err = io.Copy(io.Discard, resp.body)
	return err
}

func (w *writer) Status() (content.Status, error) {
	query := url.Values{}
	query.Set("session", w.session.sessionId)
	query.Set("writer", w.writer)

	resp, err := w.session.cli.get(context.Background(), "/contentstore/writer/status", query, nil)
	if err != nil {
		return content.Status{}, err
	}
	defer ensureReaderClosed(resp)

	var status content.Status
	if err := json.NewDecoder(resp.body).Decode(&status); err != nil {
		return content.Status{}, err
	}
	return status, nil
}

func (w *writer) Truncate(size int64) error {
	query := url.Values{}
	query.Set("session", w.session.sessionId)
	query.Set("writer", w.writer)
	query.Set("size", strconv.FormatInt(size, 10))

	resp, err := w.session.cli.post(context.Background(), "/contentstore/writer/truncate", query, nil, nil)
	if err != nil {
		return err
	}
	defer ensureReaderClosed(resp)

	_, err = io.Copy(io.Discard, resp.body)
	return err

}

var _ content.Provider = &ContentStoreSession{}
var _ content.InfoProvider = &ContentStoreSession{}
var _ content.Ingester = &ContentStoreSession{}

type ContentStoreSessionOpt func(*ContentStoreSession)

func (cli *Client) ContentStore(ctx context.Context, opts ...ContentStoreSessionOpt) (ContentStoreSession, error) {
	cs := ContentStoreSession{
		cli: cli,
	}

	resp, err := cli.post(ctx, "/contentstore", nil, nil, nil)
	if err != nil {
		return cs, err
	}
	defer ensureReaderClosed(resp)

	var response struct {
		Session string `json:"session"`
	}

	if err := json.NewDecoder(resp.body).Decode(&response); err != nil {
		return cs, err
	}
	cs.sessionId = response.Session

	return cs, err
}
