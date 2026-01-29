// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"mime"
	"net/http"

	"github.com/go-openapi/errors"
)

// ContentType parses a content type header
func ContentType(headers http.Header) (string, string, error) {
	ct := headers.Get(HeaderContentType)
	orig := ct
	if ct == "" {
		ct = DefaultMime
	}
	if ct == "" {
		return "", "", nil
	}

	mt, opts, err := mime.ParseMediaType(ct)
	if err != nil {
		return "", "", errors.NewParseError(HeaderContentType, "header", orig, err)
	}

	if cs, ok := opts[charsetKey]; ok {
		return mt, cs, nil
	}

	return mt, "", nil
}
