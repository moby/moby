// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	stderrors "errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-openapi/errors"
)

// DefaultMaxUploadFilenameLength is the default cap applied to
// FileHeader.Filename for each declared file when [BindForm] is invoked
// without an explicit [BindFormMaxFilenameLen] option.
//
// Multipart headers are allocated per part; an attacker submitting
// multi-MB filenames inflates the parser's memory footprint. 1 KiB
// matches the IETF guidance for sane filename length and is enough
// for realistic uploads.
const DefaultMaxUploadFilenameLength = 1024

// DefaultMaxUploadBodySize limits the size of the body to upload forms to 32MB.
//
// Use an explicit [BindFormMaxBody] option to change this limit.
const DefaultMaxUploadBodySize = int64(32) << 20

// filenamePreviewLen caps the byte length of the FileHeader.Filename
// preview embedded as the ParseError.Value field when the helper
// rejects a too-long filename.
const filenamePreviewLen = 32

// ValidateFilenameLength enforces the FileHeader.Filename length cap
// that [BindForm] applies via [BindFormFile] declarations. Untyped
// binder paths that fetch the file via [http.Request.FormFile]
// directly (rather than declaring the file through [BindFormFile]) call
// this to opt into the same protection.
//
// Returns nil if filename length is within maxLen or maxLen <= 0.
// Otherwise returns a [*errors.ParseError] suitable for direct return
// from a parameter binder. The error embeds a truncated preview of
// the offending filename to keep the error message bounded.
func ValidateFilenameLength(paramName, paramIn, filename string, maxLen int) error {
	if maxLen <= 0 || len(filename) <= maxLen {
		return nil
	}
	preview := filename[:min(len(filename), filenamePreviewLen)]
	return errors.NewParseError(paramName, paramIn, preview,
		fmt.Errorf("filename length %d exceeds limit %d", len(filename), maxLen))
}

// FileBinder is the per-file callback invoked by [BindForm] when a
// declared file field is present.
//
// The callback is responsible for BOTH validating the file (size, MIME, etc.) AND assigning the bound
// file to its destination — typically using:
//
//	o.FieldName = &runtime.File{Data: file, Header: header}
//
// Returning a non-nil error surfaces the error in [BindForm]'s per-field
// accumulator. Errors from the binder flow through verbatim — the
// binder is expected to produce HTTP-aware errors (e.g.
// [errors.ExceedsMaximum] from go-openapi/validate).
type FileBinder func(file multipart.File, header *multipart.FileHeader) error

// BindOption configures [BindForm]. The variadic style keeps simple
// call sites simple and lets new knobs (security caps, additional
// behaviour) be added without breaking the signature.
type BindOption func(*bindConfig)

type bindConfig struct {
	maxParseMemory int64
	maxBody        int64
	maxFiles       int
	maxFilenameLen int
	files          []formFileSpec
}

type formFileSpec struct {
	name     string
	required bool
	bind     FileBinder
}

// BindFormMaxParseMemory caps the in-memory portion of a multipart
// body. Bytes beyond this are spilled to temporary files on disk by
// the stdlib parser. 0 (the default) defers to the stdlib's 32 MB.
//
// This option does NOT cap total body bytes — see [BindFormMaxBody]
// for that. The default body cap ([DefaultMaxUploadBodySize] = 32 MB)
// is applied even when this option is not supplied, so out of the box
// [BindForm] is bounded; callers with stricter or looser requirements
// adjust via [BindFormMaxBody].
func BindFormMaxParseMemory(n int64) BindOption {
	return func(c *bindConfig) { c.maxParseMemory = n }
}

// BindFormMaxBody caps the size of the body read from a http form before parsing.
//
// The limit is set to 32MB by default. This default limit is applied for any n=0.
//
// The limit is disabled for n<0, assuming the caller has already capped the body size upstream.
func BindFormMaxBody(n int64) BindOption {
	return func(c *bindConfig) { c.maxBody = n }
}

// BindFormMaxFiles rejects parses where the total number of file
// parts across all field names exceeds n. 0 (the default) means no
// cap. Exceeding the cap is a fatal error — [BindForm] returns
// fatal=true and no per-file binders run.
func BindFormMaxFiles(n int) BindOption {
	return func(c *bindConfig) { c.maxFiles = n }
}

// BindFormMaxFilenameLen rejects per-file headers whose Filename
// length exceeds n. 0 means no cap; the default applied when this
// option is not supplied is [DefaultMaxUploadFilenameLength]. The
// cap is a per-field bind error (non-fatal); other declared files
// still run.
func BindFormMaxFilenameLen(n int) BindOption {
	return func(c *bindConfig) { c.maxFilenameLen = n }
}

// BindFormFile declares a file field to bind under the given form
// name. If required is true and the field is absent, [BindForm]
// produces the per-field error.
//
//	errors.NewParseError(name, "formData", "", http.ErrMissingFile)
//
// If required is false, absence is silent (no error, no bind).
//
// The bind callback runs only when the field is present. It is the
// site where both validation and assignment happen — see [FileBinder].
//
// FileHeader.Filename is attacker-controlled text; the binder MUST
// NOT use it directly as a filesystem path. The helper does not
// touch the filesystem.
func BindFormFile(name string, required bool, bind FileBinder) BindOption {
	return func(c *bindConfig) {
		c.files = append(c.files, formFileSpec{
			name:     name,
			required: required,
			bind:     bind,
		})
	}
}

// BindForm parses r as multipart/form-data, falling back to
// application/x-www-form-urlencoded when the request is not
// multipart. On success, r.MultipartForm and r.PostForm are populated;
// the caller can read non-file form values via [Values](r.Form) after
// the call returns.
//
// All errors produced by BindForm itself (parse failure, missing
// required field, cap exceeded) are [*errors.ParseError] values built
// via [errors.NewParseError], matching the untyped
// middleware/parameter.go path. Errors returned by per-file binders
// flow through verbatim — binders own their HTTP-aware error shape.
//
// Per-file binders declared via [BindFormFile] run in declaration
// order after a successful parse. Their errors are accumulated and
// returned wrapped in [errors.CompositeValidationError]; the caller
// typically appends the returned err to its own []error and continues
// with non-file parameter binding.
//
// Return semantics:
//
//   - fatal=true, err!=nil: parse failure or a hard cap (e.g.
//     [BindFormMaxFiles]) was exceeded. No per-file binders ran; the
//     caller MUST return err immediately.
//   - fatal=false, err!=nil: one or more per-file binders produced
//     errors. The form parsed successfully; r.Form is populated. The
//     caller appends err to its accumulator and continues.
//   - fatal=false, err==nil: full success.
//
// fatal==true implies err!=nil.
//
// Defaults applied out of the box:
//
//   - Total body bytes capped at [DefaultMaxUploadBodySize] (32 MB)
//     via [http.MaxBytesReader]. Adjust with [BindFormMaxBody]
//     (negative n disables, when the caller has already capped the
//     body upstream).
//   - FileHeader.Filename length capped at
//     [DefaultMaxUploadFilenameLength]. Adjust with
//     [BindFormMaxFilenameLen].
//
// Caller responsibilities the helper does NOT cover:
//
//   - Set [http.Server.ReadTimeout] / [http.Server.IdleTimeout] to defend
//     against slow-read attacks.
//   - Decompress Content-Encoding: gzip request bodies upstream if
//     the API accepts them, using a size-limited reader.
//   - Treat FileHeader.Filename as untrusted user input; never use
//     it directly as a filesystem path.
func BindForm(r *http.Request, opts ...BindOption) (fatal bool, err error) {
	cfg := bindConfig{
		maxFilenameLen: DefaultMaxUploadFilenameLength,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if perr := parseFormBody(r, cfg.maxParseMemory, cfg.maxBody); perr != nil {
		// Body-cap hit gets the 413 status; everything else maps to a
		// 400 ParseError. parseFormBody returns the raw stdlib error
		// in both cases — the HTTP-aware wrapping happens here.
		var maxBytesErr *http.MaxBytesError
		if stderrors.As(perr, &maxBytesErr) {
			return true, errors.New(http.StatusRequestEntityTooLarge, "formData: %v", perr)
		}
		return true, errors.NewParseError("body", "formData", "", perr)
	}

	if cfg.maxFiles > 0 {
		if got := countFileParts(r); got > cfg.maxFiles {
			return true, errors.NewParseError("body", "formData", "",
				fmt.Errorf("multipart form contains %d file parts, exceeds limit %d", got, cfg.maxFiles))
		}
	}

	var bindErrs []error
	for _, spec := range cfg.files {
		if e := bindFormFile(r, spec, cfg.maxFilenameLen); e != nil {
			bindErrs = append(bindErrs, e)
		}
	}
	if len(bindErrs) > 0 {
		return false, errors.CompositeValidationError(bindErrs...)
	}
	return false, nil
}

// parseFormBody parses the request body. Content-Type drives the
// parser: multipart/form-data → r.ParseMultipartForm, everything else
// → r.ParseForm (stdlib's parsePostForm only actually reads the body
// when Content-Type is application/x-www-form-urlencoded, so calling
// ParseForm is safe for unrecognised types).
//
// Caveat: ParseMultipartForm calls ParseForm internally and discards its error
// when the body turns out not to be multipart, returning ErrNotMultipart instead
// — the subsequent retry then short-circuits because r.PostForm is already
// set. Content-type-based routing avoids the lossy detour.
//
// Returns the raw stdlib error on failure; the caller (BindForm)
// handles HTTP-aware wrapping (413 for MaxBytesError, 400 ParseError
// otherwise).
//
// maxMemory == 0 falls through to the stdlib default (32 MB).
// maxBody == 0 defaults to DefaultMaxUploadBodySize; maxBody < 0
// disables the body cap (caller has capped upstream).
func parseFormBody(r *http.Request, maxMemory, maxBody int64) error {
	if r.Body != nil && maxBody >= 0 {
		if maxBody == 0 {
			maxBody = DefaultMaxUploadBodySize
		}
		r.Body = http.MaxBytesReader(nil, r.Body, maxBody)
	}

	mt, _, _ := ContentType(r.Header)
	if mt == MultipartFormMime {
		//nolint:gosec // G120: false positive -- see below
		// gosec doesn't track the Body.
		// See https://github.com/securego/gosec/blob/de65614d10a6b84029e3e1215567b8ce7e490f23/testutils/g120_samples.go#L57
		return r.ParseMultipartForm(maxMemory)
	}
	return r.ParseForm()
}

func countFileParts(r *http.Request) int {
	if r.MultipartForm == nil {
		return 0
	}
	var n int
	for _, fhs := range r.MultipartForm.File {
		n += len(fhs)
	}

	return n
}

// FormFile resolves a file field from a parsed form body, transparently
// handling both content types accepted for `type: file` parameters by
// the OpenAPI 2.0 spec:
//
//   - multipart/form-data — delegates to [http.Request.FormFile].
//   - application/x-www-form-urlencoded — looks up the field in
//     r.PostForm and synthesizes a [multipart.File] backed by the
//     value bytes plus a [multipart.FileHeader] with Filename equal
//     to the field name and Size set to the byte length.
//
// Returns [http.ErrMissingFile] when the field is absent under either
// content type. Callers must have parsed the body upstream (e.g. via
// [BindForm] or [http.Request.ParseForm]) before reading from the
// urlencoded path — [http.Request.FormFile] takes care of parsing on
// the multipart path.
//
// Presence is the only criterion for binding a urlencoded file: an
// empty value (e.g. `file=`) is bound as a zero-byte file.
func FormFile(r *http.Request, name string) (multipart.File, *multipart.FileHeader, error) {
	file, header, err := r.FormFile(name)
	if err == nil {
		return file, header, nil
	}
	if !stderrors.Is(err, http.ErrNotMultipart) {
		return nil, nil, err
	}

	values, present := r.PostForm[name]
	if !present {
		return nil, nil, http.ErrMissingFile
	}
	value := values[0]
	return urlencodedFile{Reader: strings.NewReader(value)},
		&multipart.FileHeader{Filename: name, Size: int64(len(value))},
		nil
}

// urlencodedFile adapts a urlencoded form value (already buffered in
// memory by [http.Request.ParseForm]) to the [multipart.File]
// interface. The embedded [strings.Reader] supplies Read/ReadAt/Seek;
// Close is a no-op since there is no resource to release.
type urlencodedFile struct {
	*strings.Reader
}

func (urlencodedFile) Close() error { return nil }

func bindFormFile(r *http.Request, spec formFileSpec, maxFilenameLen int) error {
	file, header, err := FormFile(r, spec.name)
	if err != nil {
		if stderrors.Is(err, http.ErrMissingFile) {
			if spec.required {
				return errors.New(http.StatusBadRequest, "formData: %v", http.ErrMissingFile)
			}

			return nil
		}

		return errors.NewParseError(spec.name, "formData", "", err)
	}

	if err := ValidateFilenameLength(spec.name, "formData", header.Filename, maxFilenameLen); err != nil {
		return err
	}

	if spec.bind == nil {
		return nil
	}

	return spec.bind(file, header)
}
