// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sdkapi // import "go.opentelemetry.io/otel/metric/sdkapi"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/number"
)

type noopInstrument struct {
	descriptor Descriptor
}
type noopSyncInstrument struct{ noopInstrument }
type noopAsyncInstrument struct{ noopInstrument }

var _ SyncImpl = noopSyncInstrument{}
var _ AsyncImpl = noopAsyncInstrument{}

// NewNoopSyncInstrument returns a No-op implementation of the
// synchronous instrument interface.
func NewNoopSyncInstrument() SyncImpl {
	return noopSyncInstrument{
		noopInstrument{
			descriptor: Descriptor{
				instrumentKind: CounterInstrumentKind,
			},
		},
	}
}

// NewNoopAsyncInstrument returns a No-op implementation of the
// asynchronous instrument interface.
func NewNoopAsyncInstrument() AsyncImpl {
	return noopAsyncInstrument{
		noopInstrument{
			descriptor: Descriptor{
				instrumentKind: CounterObserverInstrumentKind,
			},
		},
	}
}

func (noopInstrument) Implementation() interface{} {
	return nil
}

func (n noopInstrument) Descriptor() Descriptor {
	return n.descriptor
}

func (noopSyncInstrument) RecordOne(context.Context, number.Number, []attribute.KeyValue) {
}
