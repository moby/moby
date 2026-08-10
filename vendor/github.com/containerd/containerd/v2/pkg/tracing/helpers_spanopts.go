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

package tracing

import (
	"context"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"go.opentelemetry.io/otel/trace"
)

// WithNamespace adds containerd namespace attribute to spans when available.
// It is best-effort: if namespace is not present in the context, it does nothing.
func WithNamespace(ctx context.Context) SpanOpt {
	return func(config *StartConfig) {
		ns, err := namespaces.NamespaceRequired(ctx)
		if err != nil {
			return
		}
		config.spanOpts = append(config.spanOpts,
			trace.WithAttributes(Attribute("namespace", ns)),
		)
	}
}
