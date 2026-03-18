// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlpc

import (
	"io"

	"github.com/go-openapi/runtime"
	yaml "go.yaml.in/yaml/v3"
)

// YAMLConsumer creates a consumer for yaml data
func YAMLConsumer() runtime.Consumer {
	return runtime.ConsumerFunc(func(r io.Reader, v any) error {
		dec := yaml.NewDecoder(r)
		return dec.Decode(v)
	})
}

// YAMLProducer creates a producer for yaml data
func YAMLProducer() runtime.Producer {
	return runtime.ProducerFunc(func(w io.Writer, v any) error {
		enc := yaml.NewEncoder(w)
		defer enc.Close()
		return enc.Encode(v)
	})
}
