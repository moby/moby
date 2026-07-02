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

package server

import (
	"context"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"google.golang.org/grpc"
)

func unaryNamespaceInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if ns, ok := namespaces.Namespace(ctx); ok {
		// The above call checks the *incoming* metadata, this makes sure the outgoing metadata is also set
		ctx = namespaces.WithNamespace(ctx, ns)
	}
	return handler(ctx, req)
}

func streamNamespaceInterceptor(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	if ns, ok := namespaces.Namespace(ctx); ok {
		// The above call checks the *incoming* metadata, this makes sure the outgoing metadata is also set
		ctx = namespaces.WithNamespace(ctx, ns)
		ss = &wrappedSSWithContext{ctx: ctx, ServerStream: ss}
	}

	return handler(srv, ss)
}

type wrappedSSWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedSSWithContext) Context() context.Context {
	return w.ctx
}
