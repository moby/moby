// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"encoding/json"
	"io"
)

// JSONConsumer creates a new JSON consumer
func JSONConsumer() Consumer {
	return ConsumerFunc(func(reader io.Reader, data any) error {
		dec := json.NewDecoder(reader)
		dec.UseNumber() // preserve number formats
		return dec.Decode(data)
	})
}

// JSONProducer creates a new JSON producer
func JSONProducer() Producer {
	return ProducerFunc(func(writer io.Writer, data any) error {
		enc := json.NewEncoder(writer)
		enc.SetEscapeHTML(false)
		return enc.Encode(data)
	})
}
