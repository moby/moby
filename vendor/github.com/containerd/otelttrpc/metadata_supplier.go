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

/*
   Copyright The OpenTelemetry Authors

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

package otelttrpc

import (
	"context"

	"github.com/containerd/ttrpc"
	"go.opentelemetry.io/otel/propagation"
)

type metadataSupplier struct {
	metadata *ttrpc.MD
}

// assert that metadataSupplier implements the TextMapCarrier interface.
var _ propagation.TextMapCarrier = &metadataSupplier{}

func (s *metadataSupplier) Get(key string) string {
	values, _ := s.metadata.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (s *metadataSupplier) Set(key string, value string) {
	s.metadata.Set(key, value)
}

func (s *metadataSupplier) Keys() []string {
	out := make([]string, 0, len(*s.metadata))
	for key := range *s.metadata {
		out = append(out, key)
	}
	return out
}

func inject(ctx context.Context, propagators propagation.TextMapPropagator, req *ttrpc.Request) context.Context {
	md, ok := ttrpc.GetMetadata(ctx)
	if !ok {
		md = make(ttrpc.MD)
	} else {
		// make a copy to avoid concurrent read/write panic
		md = md.Clone()
	}

	propagators.Inject(ctx, &metadataSupplier{
		metadata: &md,
	})

	// keep non-conflicting metadata from req, update others from context
	newMD := make([]*ttrpc.KeyValue, 0)
	for _, kv := range req.Metadata {
		if _, found := md.Get(kv.Key); !found {
			newMD = append(newMD, kv)
		}
	}
	for k, values := range md {
		for _, v := range values {
			newMD = append(newMD, &ttrpc.KeyValue{
				Key:   k,
				Value: v,
			})
		}
	}
	req.Metadata = newMD

	return ttrpc.WithMetadata(ctx, md)
}

func extract(ctx context.Context, propagators propagation.TextMapPropagator) context.Context {
	md, ok := ttrpc.GetMetadata(ctx)
	if !ok {
		md = make(ttrpc.MD)
	}

	return propagators.Extract(ctx, &metadataSupplier{
		metadata: &md,
	})
}
