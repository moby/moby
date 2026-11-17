// SPDX-FileCopyrightText: Copyright 2015-2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/go-openapi/errors"
)

// MultipartFormStreamOption configures [NewMultipartFormStream].
type MultipartFormStreamOption func(*multipartFormStreamConfig)

const defaultMultipartFormStreamMaxParts = 1000

type multipartFormStreamConfig struct {
	multipartFormLimits

	maxParts int
}

// MultipartFormStreamMaxBody caps the total number of request-body bytes read
// by a [MultipartFormStream].
//
// A value of 0 applies [DefaultMaxUploadBodySize]. A negative value disables
// the cap when the caller has already limited the request body upstream.
func MultipartFormStreamMaxBody(n int64) MultipartFormStreamOption {
	return func(c *multipartFormStreamConfig) { c.maxBody = n }
}

// MultipartFormStreamMaxFiles rejects a multipart stream after more than n
// file parts have been encountered. A value of 0 means no file-count cap.
func MultipartFormStreamMaxFiles(n int) MultipartFormStreamOption {
	return func(c *multipartFormStreamConfig) { c.maxFiles = n }
}

// MultipartFormStreamMaxParts rejects a multipart stream after more than n
// total parts have been encountered. The default is 1000, matching
// [multipart.Reader.ReadForm]. A value of 0 disables the limit.
func MultipartFormStreamMaxParts(n int) MultipartFormStreamOption {
	return func(c *multipartFormStreamConfig) { c.maxParts = n }
}

// MultipartFormStreamMaxFilenameLen rejects file parts whose filename exceeds
// n bytes. A value of 0 disables the limit. When this option is not supplied,
// [DefaultMaxUploadFilenameLength] is used.
func MultipartFormStreamMaxFilenameLen(n int) MultipartFormStreamOption {
	return func(c *multipartFormStreamConfig) { c.maxFilenameLen = n }
}

// StreamedFile exposes a file part directly from the multipart request body.
//
// Reads block until bytes arrive from the client. StreamedFile is not seekable
// and is not safe for concurrent use. Its form name, filename and MIME headers
// are available before the payload is consumed. The underlying [multipart.Part]
// remains private so callers cannot bypass Close and its error-preserving drain
// semantics; Header exposes the part metadata without exposing that lifecycle.
//
// Closing a StreamedFile drains only the unread remainder of that file part.
// Close may therefore block while the client is still uploading the current
// part. The owning [MultipartFormStream] may then advance to the next part.
type StreamedFile struct {
	FieldName string
	Filename  string
	Header    textproto.MIMEHeader
	part      *multipart.Part
	closeErr  error
}

// MultipartFileInfo describes a file part discovered by [MultipartFormStream].
//
// The payload reader is intentionally omitted. File parts remain sequential and
// are consumed through [MultipartFormStream.NextFile]. Header is a snapshot of
// the client-supplied MIME headers and must be treated as untrusted input.
type MultipartFileInfo struct {
	FieldName string
	Filename  string
	Header    textproto.MIMEHeader
}

// Read reads file payload bytes directly from the request body.
func (f *StreamedFile) Read(p []byte) (int, error) {
	if f == nil || f.part == nil {
		return 0, io.ErrClosedPipe
	}

	return f.part.Read(p)
}

// Close discards the unread remainder of this file part.
//
// Close does not close the underlying HTTP request body and does not consume
// subsequent multipart parts. After Close returns successfully, the parent
// MultipartFormStream may advance to the next part.
//
// Any error encountered while discarding the unread payload is returned.
func (f *StreamedFile) Close() error {
	if f == nil {
		return nil
	}
	if f.part == nil {
		return f.closeErr
	}

	part := f.part
	f.part = nil
	// multipart.Part.Close drains with io.Copy but intentionally discards the
	// resulting error, so drain explicitly to preserve error propagation.
	_, f.closeErr = io.Copy(io.Discard, part)

	return f.closeErr
}

// MultipartFormStream reads multipart/form-data sequentially without parsing
// the complete request body before exposing file payloads.
//
// [MultipartFormStream.NextFile] consumes ordinary form fields until it reaches
// the next file part. Fields are appended to request.PostForm and request.Form
// as they are encountered. Consequently, fields after a file become visible
// only after the caller consumes or closes that file and advances the stream.
//
// A stream and its returned files are not safe for concurrent use. There is at
// most one active file part. Calling [MultipartFormStream.NextFile] closes and
// drains an unread active file before advancing, and may therefore block until
// the current part finishes arriving. No background goroutines are started.
//
// The caller owns the stream. Call [MultipartFormStream.Drain] to consume the
// remaining body, collect trailing fields and allow HTTP connection reuse when
// possible. Call [MultipartFormStream.Close] to stop multipart processing
// without explicitly draining the remaining parts.
//
// [MultipartFormStream.Fields] and [MultipartFormStream.Files] expose snapshots
// of the multipart fields and file metadata discovered so far. They never read
// ahead: fields or files after the active file become visible only after the
// stream advances.
type MultipartFormStream struct {
	multipartFormStreamConfig

	request *http.Request
	reader  *multipart.Reader
	current *StreamedFile

	fields    url.Values
	fileInfos []MultipartFileInfo
	parts     int
	closed    bool
	done      bool
}

// NewMultipartFormStream creates a sequential multipart/form-data stream over
// r.Body.
//
// For POST, PUT and PATCH requests, the constructor accepts only
// multipart/form-data, validates that a non-empty boundary is present, but does
// not consume multipart parts. For other methods, it returns an empty stream
// whose NextFile method reports io.EOF without reading the request body.
//
// The constructor initializes request.Form and request.PostForm in the same way
// as [http.Request.ParseForm], then populates multipart values incrementally as
// [MultipartFormStream.NextFile] advances.
//
// For POST, PUT and PATCH requests, NewMultipartFormStream marks the request
// as handled by MultipartReader. Callers must not subsequently call
// [http.Request.ParseMultipartForm] or [BindForm] for the same request.
//
// File payloads are exposed directly from the request body and are not buffered
// in memory or temporary files. Ordinary form values are read into memory as
// they are encountered. Parts are processed in wire order.
//
// At most one StreamedFile may be active at a time. Calling
// [MultipartFormStream.NextFile] automatically closes and drains an unread
// current file before advancing. No background goroutines are started.
//
// MultipartFormStream is not safe for concurrent use.
//
// File names and MIME headers are supplied by the client and remain untrusted.
func NewMultipartFormStream(r *http.Request, opts ...MultipartFormStreamOption) (*MultipartFormStream, error) {
	cfg := multipartFormStreamConfig{
		multipartFormLimits: multipartFormLimits{
			maxFilenameLen: DefaultMaxUploadFilenameLength,
		},
		maxParts: defaultMultipartFormStreamMaxParts,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if r == nil {
		return nil, errors.NewParseError("body", "formData", "", stderrors.New("nil request"))
	}

	if !supportsMultipartFormStream(r.Method) {
		if err := r.ParseForm(); err != nil {
			return nil, errors.NewParseError("body", "formData", "", err)
		}

		return &MultipartFormStream{
			request:                   r,
			multipartFormStreamConfig: cfg,
			fields:                    make(url.Values),
			done:                      true,
		}, nil
	}

	if r.Body == nil {
		return nil, errors.NewParseError("body", "formData", "", stderrors.New("nil request body"))
	}

	contentType := r.Header.Get(HeaderContentType)
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, errors.NewParseError(HeaderContentType, "header", contentType, err)
	}
	if mediaType != MultipartFormMime {
		return nil, errors.NewParseError(HeaderContentType, "header", mediaType, http.ErrNotMultipart)
	}
	if params["boundary"] == "" {
		return nil, errors.NewParseError(HeaderContentType, "header", contentType, http.ErrMissingBoundary)
	}
	if err = r.ParseForm(); err != nil {
		return nil, errors.NewParseError("body", "formData", "", err)
	}

	body := r.Body
	if cfg.maxBody >= 0 {
		maxBody := cfg.maxBody
		if maxBody == 0 {
			maxBody = DefaultMaxUploadBodySize
		}
		body = http.MaxBytesReader(nil, body, maxBody)
	}
	body = &contextReadCloser{ctx: r.Context(), ReadCloser: body}
	r.Body = body

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, errors.NewParseError("body", "formData", "", err)
	}

	return &MultipartFormStream{
		request:                   r,
		reader:                    reader,
		multipartFormStreamConfig: cfg,
		fields:                    make(url.Values),
	}, nil
}

// Fields returns a snapshot of ordinary multipart form fields discovered so far.
//
// URL query values are not included. Repeated multipart fields preserve their
// encounter order. Fields after the active file are not visible until that file
// is consumed or closed and the stream advances. Mutating the returned values
// does not affect the stream or the request.
func (s *MultipartFormStream) Fields() url.Values {
	if s == nil {
		return nil
	}

	return cloneMultipartValues(s.fields)
}

// Files returns snapshots of file metadata discovered so far in wire order.
//
// The currently active file is included as soon as NextFile returns it. Payload
// readers are not retained in the index. Mutating the returned slice or MIME
// headers does not affect the stream.
func (s *MultipartFormStream) Files() []MultipartFileInfo {
	if s == nil {
		return nil
	}

	files := make([]MultipartFileInfo, len(s.fileInfos))
	for i, file := range s.fileInfos {
		files[i] = MultipartFileInfo{
			FieldName: file.FieldName,
			Filename:  file.Filename,
			Header:    cloneMultipartMIMEHeader(file.Header),
		}
	}

	return files
}

// NextFile advances through the multipart body and returns the next file part.
// Ordinary form fields encountered before that file are added to request.Form
// and request.PostForm.
//
// If the previously returned file is still open, NextFile closes and drains it
// before advancing. This may block while the client is still uploading that
// part. Any drain error is returned and the stream is aborted.
//
// NextFile returns io.EOF when no file parts remain. At that point all trailing
// ordinary fields have been collected.
func (s *MultipartFormStream) NextFile() (*StreamedFile, error) {
	if err := s.prepareNextFile(); err != nil {
		return nil, err
	}

	return s.readNextFile()
}

// Drain consumes the rest of the multipart body.
//
// Unread file payloads are discarded. Non-file form fields encountered while
// draining are collected in the request form values.
//
// Drain closes the underlying request body after reaching EOF. Subsequent calls
// to NextFile return io.EOF. Drain returns any multipart parsing, payload drain
// or request-body close error.
func (s *MultipartFormStream) Drain() error {
	if s == nil || s.closed {
		return nil
	}

	for {
		file, err := s.NextFile()
		if stderrors.Is(err, io.EOF) {
			return s.Close()
		}
		if err != nil {
			return stderrors.Join(err, s.Close())
		}
		if err = file.Close(); err != nil {
			return stderrors.Join(err, s.Close())
		}
	}
}

// Close stops multipart processing and closes the underlying HTTP request body
// without explicitly draining the remaining multipart parts.
//
// The concrete request body may perform its own work during Close. In
// particular, a net/http server request body may discard a limited amount of
// unread data to allow connection reuse, so Close is not guaranteed to return
// immediately. Call Drain when trailing form fields must be collected.
//
// Close returns the error reported by the request body, except io.EOF: a
// net/http server body reports io.EOF once it discards that unread remainder,
// which tells us the body ended, not that closing it failed.
func (s *MultipartFormStream) Close() error {
	if s == nil || s.closed {
		return nil
	}

	s.closed = true
	if s.current != nil {
		s.current.part = nil
		s.current = nil
	}

	if s.request == nil || s.request.Body == nil {
		return nil
	}

	// Since go1.27.0, net/http (*body).Close returns io.EOF when it discards
	// the unread remainder of the request itself. Reaching the end of the body
	// is not a close failure.
	if err := s.request.Body.Close(); err != nil && !stderrors.Is(err, io.EOF) {
		return err
	}

	return nil
}

func supportsMultipartFormStream(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func (s *MultipartFormStream) abort(err error) error {
	return stderrors.Join(err, s.Close())
}

func (s *MultipartFormStream) closeCurrent() error {
	if s.current == nil {
		return nil
	}

	err := s.current.Close()
	s.current = nil

	return err
}

func (s *MultipartFormStream) bindValue(part *multipart.Part, name string) error {
	value, err := io.ReadAll(part)
	if err != nil {
		return err
	}

	s.fields.Add(name, string(value))
	s.request.PostForm.Add(name, string(value))
	// Match net/http.ParseMultipartForm: query values already present in Form
	// keep precedence over multipart body values, which are appended.
	s.request.Form.Add(name, string(value))

	return nil
}

func discardPart(part *multipart.Part) error {
	_, err := io.Copy(io.Discard, part)

	return err
}

func cloneMultipartValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for name, entries := range values {
		cloned[name] = append([]string(nil), entries...)
	}

	return cloned
}

func cloneMultipartMIMEHeader(header textproto.MIMEHeader) textproto.MIMEHeader {
	cloned := make(textproto.MIMEHeader, len(header))
	for name, entries := range header {
		cloned[name] = append([]string(nil), entries...)
	}

	return cloned
}

type contextReadCloser struct {
	io.ReadCloser

	ctx context.Context //nolint:containedctx // Read has no context parameter, so the wrapper must retain it
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.ReadCloser.Read(p)
}

func (s *MultipartFormStream) prepareNextFile() error {
	if s == nil {
		return io.ErrClosedPipe
	}
	if s.done {
		return io.EOF
	}
	if s.closed {
		return io.ErrClosedPipe
	}
	if err := s.closeCurrent(); err != nil {
		return s.abort(err)
	}

	return nil
}

func (s *MultipartFormStream) readNextFile() (*StreamedFile, error) {
	for {
		part, err := s.reader.NextPart()
		if err != nil {
			return nil, s.handleNextPartError(err)
		}

		s.parts++
		if s.maxParts > 0 && s.parts > s.maxParts {
			err := errors.NewParseError(
				"body",
				"formData",
				"",
				fmt.Errorf(
					"multipart form contains %d parts, exceeds limit %d",
					s.parts,
					s.maxParts,
				),
			)

			return nil, s.abort(err)
		}

		fieldName := part.FormName()
		if fieldName == "" {
			if err := discardPart(part); err != nil {
				return nil, s.abort(err)
			}

			continue
		}

		filename := part.FileName()
		if filename == "" {
			if err := s.bindValue(part, fieldName); err != nil {
				return nil, s.abort(err)
			}

			continue
		}

		return s.openFile(part, fieldName, filename)
	}
}

func (s *MultipartFormStream) handleNextPartError(err error) error {
	if stderrors.Is(err, io.EOF) {
		s.done = true

		return io.EOF
	}

	return s.abort(err)
}

func (s *MultipartFormStream) openFile(
	part *multipart.Part,
	fieldName string,
	filename string,
) (*StreamedFile, error) {
	fileCount := len(s.fileInfos) + 1
	if s.maxFiles > 0 && fileCount > s.maxFiles {
		err := errors.NewParseError(
			"body",
			"formData",
			"",
			fmt.Errorf(
				"multipart form contains %d file parts, exceeds limit %d",
				fileCount,
				s.maxFiles,
			),
		)

		return nil, s.abort(err)
	}

	if err := ValidateFilenameLength(
		fieldName,
		"formData",
		filename,
		s.maxFilenameLen,
	); err != nil {
		return nil, s.abort(err)
	}

	file := &StreamedFile{
		FieldName: fieldName,
		Filename:  filename,
		Header:    part.Header,
		part:      part,
	}
	s.fileInfos = append(s.fileInfos, MultipartFileInfo{
		FieldName: fieldName,
		Filename:  filename,
		Header:    cloneMultipartMIMEHeader(part.Header),
	})
	s.current = file

	return file, nil
}
