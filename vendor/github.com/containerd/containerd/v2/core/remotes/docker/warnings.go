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
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/v2/pkg/reference"
)

// WarningSource contains the information about the request that caused the warning.
type WarningSource struct {
	// Ref is the reference specification of the content that the warning is for.
	Ref reference.Spec

	// Desc is the descriptor of the content that the warning is for.
	// Can be nil if the warning is not for a specific content.
	Desc *ocispec.Descriptor

	// Digest is the digest of the content that the warning is for.
	// Can be nil if the warning is not for a specific content.
	Digest *digest.Digest
}

// WarningHandler is a function that receives warnings from the registry.
// Warnings are extracted from HTTP Warning headers as defined in RFC 7234.
// The src parameter contains information about the request that the warning is
// for. If the warning is for specific content, its descriptor is available via
// src.Desc. src.Desc is nil if the warning is not for a specific content.
type WarningHandler interface {
	Warn(ctx context.Context, src WarningSource, warning string)
}

type warningHandlerFunc func(ctx context.Context, src WarningSource, warning string)

func (f warningHandlerFunc) Warn(ctx context.Context, src WarningSource, warning string) {
	f(ctx, src, warning)
}

type warningSourceKey struct{}

// reportWarnings extracts and reports warnings from HTTP Warning headers.
// Per RFC 7234 and OCI distribution spec, warnings use warn-code 299,
// warn-agent "-", and the format: Warning: 299 - "message"
func reportWarnings(ctx context.Context, header http.Header, handler WarningHandler) {
	if handler == nil {
		return
	}

	var warnSrc WarningSource
	if v, ok := ctx.Value(warningSourceKey{}).(WarningSource); ok {
		warnSrc = v
	}

	reportWarningsWithSource(ctx, header, handler, warnSrc)
}

func reportWarningsWithSource(ctx context.Context, header http.Header, handler WarningHandler, warnSrc WarningSource) {
	if handler == nil {
		return
	}

	for _, warning := range header.Values("Warning") {
		// Parse RFC 7234 warning format: warn-code warn-agent "warn-text"
		// Expected format for OCI: 299 - "message"
		if text := parseWarningText(warning); text != "" {
			handler.Warn(ctx, warnSrc, text)
		}
	}
}

// parseWarningText extracts the warn-text from an RFC 7234 Warning header value.
// Only the OCI distribution spec recommended format is supported:
// 299 - "message"
// Other warn-agent values (tokens other than "-") and optional warn-date
// suffixes defined in RFC 7234 are not supported and will cause the
// warning to be silently ignored.
// Characters are validated per RFC 7230 section 3.2.6 quoted-string rules:
//   - qdtext: HTAB / SP / %x21 / %x23-5B / %x5D-7E / obs-text
//   - quoted-pair: "\" ( HTAB / SP / VCHAR / obs-text )
//   - percent-encoding: %xx where xx is a hex value; decoded byte must be valid qdtext
func parseWarningText(warning string) string {
	text, ok := strings.CutPrefix(warning, "299 - ")
	if !ok {
		return ""
	}

	text = strings.TrimSpace(text)

	ln := len(text)
	if ln == 0 || text[0] != '"' || text[ln-1] != '"' {
		return ""
	}

	out := strings.Builder{}
	idx := 1 // skip opening quote
	end := ln - 1

	for idx < end {
		c := text[idx]

		if c == '\\' {
			if idx+1 >= end {
				return ""
			}
			nextC := text[idx+1]
			if !isQuotedPairChar(nextC) {
				return ""
			}
			out.WriteByte(nextC)
			idx += 2
			continue
		}

		// Handle percent-encoding (%xx)
		if c == '%' && idx+2 < end {
			decoded, err := hex.DecodeString(text[idx+1 : idx+3])
			if err == nil && len(decoded) == 1 {
				if !isQdtext(decoded[0]) {
					return ""
				}
				out.WriteByte(decoded[0])
				idx += 3
				continue
			}
			// Invalid percent-encoding, treat as literal
		}

		if !isQdtext(c) {
			return ""
		}
		out.WriteByte(c)
		idx++
	}

	return out.String()
}

func updateWarningSource(ctx context.Context, ref reference.Spec) context.Context {
	var warnSrc WarningSource
	if v, ok := ctx.Value(warningSourceKey{}).(WarningSource); ok {
		warnSrc = v
	}
	warnSrc.Ref = ref
	ctx = context.WithValue(ctx, warningSourceKey{}, warnSrc)
	return ctx
}

// isQdtext returns whether c is valid unescaped inside a quoted-string per RFC 7230 §3.2.6.
//
//	qdtext = HTAB / SP / %x21 / %x23-5B / %x5D-7E / %x80-FF
func isQdtext(c byte) bool {
	if c == 0x09 || c == 0x20 { // HTAB, SP
		return true
	}
	if c == 0x21 { // '!'
		return true
	}
	if c >= 0x23 && c <= 0x5B { // '#' to '['
		return true
	}
	if c >= 0x5D && c <= 0x7E { // ']' to '~'
		return true
	}
	if c >= 0x80 { // obs-text
		return true
	}
	return false
}

// isQuotedPairChar returns whether c is valid after a backslash per RFC 7230 §3.2.6.
//
//	quoted-pair = "\" ( HTAB / SP / VCHAR / %x80-FF )
//	VCHAR = %x21-7E
func isQuotedPairChar(c byte) bool {
	if c == 0x09 || c == 0x20 { // HTAB, SP
		return true
	}
	if c >= 0x21 && c <= 0x7E { // VCHAR
		return true
	}
	if c >= 0x80 { // obs-text
		return true
	}
	return false
}

// LogWarningHandler is a WarningHandler that logs warnings using the
// containerd log package.
type LogWarningHandler struct{}

// Warn logs the warning message with source information.
func (LogWarningHandler) Warn(ctx context.Context, src WarningSource, warning string) {
	fields := log.Fields{}
	if ref := src.Ref.String(); ref != "" {
		fields["ref"] = ref
	}
	if src.Digest != nil {
		fields["digest"] = src.Digest.String()
	}
	log.G(ctx).WithFields(fields).Warn("registry warning: " + warning)
}
