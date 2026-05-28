//
// Copyright 2021 The Sigstore Authors.
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

// Package options defines options for KMS clients
package options

import (
	"context"
)

// RequestContext implements the functional option pattern for including a context during RPC
type RequestContext struct {
	NoOpOptionImpl
	ctx context.Context
}

// ApplyContext sets the specified context as the functional option
func (r RequestContext) ApplyContext(ctx *context.Context) {
	*ctx = r.ctx
}

// WithContext specifies that the given context should be used in RPC to external services
func WithContext(ctx context.Context) RequestContext {
	return RequestContext{ctx: ctx}
}
