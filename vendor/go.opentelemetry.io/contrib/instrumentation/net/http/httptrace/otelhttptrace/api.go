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

package otelhttptrace

import (
	"context"
	"net/http"
	"net/http/httptrace"
)

// W3C client.
func W3C(ctx context.Context, req *http.Request) (context.Context, *http.Request) {
	ctx = httptrace.WithClientTrace(ctx, NewClientTrace(ctx))
	req = req.WithContext(ctx)
	return ctx, req
}
