// Copyright 2018, Google Inc.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package gax

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"unicode"

	"github.com/googleapis/gax-go/v2/callctx"
	"google.golang.org/grpc/metadata"
)

var (
	// GoVersion is a header-safe representation of the current runtime
	// environment's Go version. This is for GAX consumers that need to
	// report the Go runtime version in API calls.
	GoVersion string
	// version is a package internal global variable for testing purposes.
	version = runtime.Version
)

// versionUnknown is only used when the runtime version cannot be determined.
const versionUnknown = "UNKNOWN"

func init() {
	GoVersion = goVersion()
}

// goVersion returns a Go runtime version derived from the runtime environment
// that is modified to be suitable for reporting in a header, meaning it has no
// whitespace. If it is unable to determine the Go runtime version, it returns
// versionUnknown.
func goVersion() string {
	const develPrefix = "devel +"

	s := version()
	if strings.HasPrefix(s, develPrefix) {
		s = s[len(develPrefix):]
		if p := strings.IndexFunc(s, unicode.IsSpace); p >= 0 {
			s = s[:p]
		}
		return s
	} else if p := strings.IndexFunc(s, unicode.IsSpace); p >= 0 {
		s = s[:p]
	}

	notSemverRune := func(r rune) bool {
		return !strings.ContainsRune("0123456789.", r)
	}

	if strings.HasPrefix(s, "go1") {
		s = s[2:]
		var prerelease string
		if p := strings.IndexFunc(s, notSemverRune); p >= 0 {
			s, prerelease = s[:p], s[p:]
		}
		if strings.HasSuffix(s, ".") {
			s += "0"
		} else if strings.Count(s, ".") < 2 {
			s += ".0"
		}
		if prerelease != "" {
			// Some release candidates already have a dash in them.
			if !strings.HasPrefix(prerelease, "-") {
				prerelease = "-" + prerelease
			}
			s += prerelease
		}
		return s
	}
	return "UNKNOWN"
}

// XGoogHeader is for use by the Google Cloud Libraries only.
//
// XGoogHeader formats key-value pairs.
// The resulting string is suitable for x-goog-api-client header.
func XGoogHeader(keyval ...string) string {
	if len(keyval) == 0 {
		return ""
	}
	if len(keyval)%2 != 0 {
		panic("gax.Header: odd argument count")
	}
	var buf bytes.Buffer
	for i := 0; i < len(keyval); i += 2 {
		buf.WriteByte(' ')
		buf.WriteString(keyval[i])
		buf.WriteByte('/')
		buf.WriteString(keyval[i+1])
	}
	return buf.String()[1:]
}

// InsertMetadataIntoOutgoingContext is for use by the Google Cloud Libraries
// only.
//
// InsertMetadataIntoOutgoingContext returns a new context that merges the
// provided keyvals metadata pairs with any existing metadata/headers in the
// provided context. keyvals should have a corresponding value for every key
// provided. If there is an odd number of keyvals this method will panic.
// Existing values for keys will not be overwritten, instead provided values
// will be appended to the list of existing values.
func InsertMetadataIntoOutgoingContext(ctx context.Context, keyvals ...string) context.Context {
	return metadata.NewOutgoingContext(ctx, insertMetadata(ctx, keyvals...))
}

// BuildHeaders is for use by the Google Cloud Libraries only.
//
// BuildHeaders returns a new http.Header that merges the provided
// keyvals header pairs with any existing metadata/headers in the provided
// context. keyvals should have a corresponding value for every key provided.
// If there is an odd number of keyvals this method will panic.
// Existing values for keys will not be overwritten, instead provided values
// will be appended to the list of existing values.
func BuildHeaders(ctx context.Context, keyvals ...string) http.Header {
	return http.Header(insertMetadata(ctx, keyvals...))
}

func insertMetadata(ctx context.Context, keyvals ...string) metadata.MD {
	if len(keyvals)%2 != 0 {
		panic(fmt.Sprintf("gax: an even number of key value pairs must be provided, got %d", len(keyvals)))
	}
	out, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		out = metadata.MD(make(map[string][]string))
	}
	headers := callctx.HeadersFromContext(ctx)
	for k, v := range headers {
		out[k] = append(out[k], v...)
	}
	for i := 0; i < len(keyvals); i = i + 2 {
		out[keyvals[i]] = append(out[keyvals[i]], keyvals[i+1])
	}
	return out
}
