// Copyright 2018, OpenCensus Authors
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

//go:build go1.11
// +build go1.11

package trace

import (
	"context"
	t "runtime/trace"
)

func startExecutionTracerTask(ctx context.Context, name string) (context.Context, func()) {
	if !t.IsEnabled() {
		// Avoid additional overhead if
		// runtime/trace is not enabled.
		return ctx, func() {}
	}
	nctx, task := t.NewTask(ctx, name)
	return nctx, task.End
}
