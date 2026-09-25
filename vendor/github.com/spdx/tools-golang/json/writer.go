// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package json

import (
	"encoding/json"
	"io"

	"github.com/spdx/tools-golang/spdx/common"
)

type WriteOption func(*json.Encoder)

func Indent(indent string) WriteOption {
	return func(e *json.Encoder) {
		e.SetIndent("", indent)
	}
}

func EscapeHTML(escape bool) WriteOption {
	return func(e *json.Encoder) {
		e.SetEscapeHTML(escape)
	}
}

// Write takes an SPDX Document and an io.Writer, and writes the document to the writer in JSON format.
func Write(doc common.AnyDocument, w io.Writer, opts ...WriteOption) error {
	e := json.NewEncoder(w)
	for _, opt := range opts {
		opt(e)
	}
	return e.Encode(doc)
}
