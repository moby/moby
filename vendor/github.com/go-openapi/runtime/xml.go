// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"encoding/xml"
	"io"
)

// XMLConsumer creates a new XML consumer
func XMLConsumer() Consumer {
	return ConsumerFunc(func(reader io.Reader, data any) error {
		dec := xml.NewDecoder(reader)
		return dec.Decode(data)
	})
}

// XMLProducer creates a new XML producer
func XMLProducer() Producer {
	return ProducerFunc(func(writer io.Writer, data any) error {
		enc := xml.NewEncoder(writer)
		return enc.Encode(data)
	})
}
