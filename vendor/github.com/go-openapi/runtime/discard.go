// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package runtime

import "io"

// DiscardConsumer does absolutely nothing, it's a black hole.
var DiscardConsumer = ConsumerFunc(func(_ io.Reader, _ any) error { return nil })

// DiscardProducer does absolutely nothing, it's a black hole.
var DiscardProducer = ProducerFunc(func(_ io.Writer, _ any) error { return nil })
