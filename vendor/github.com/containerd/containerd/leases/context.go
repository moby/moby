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

package leases

import "context"

type leaseKey struct{}

// WithLease sets a given lease on the context
func WithLease(ctx context.Context, lid string) context.Context {
	ctx = context.WithValue(ctx, leaseKey{}, lid)

	// also store on the grpc headers so it gets picked up by any clients that
	// are using this.
	return withGRPCLeaseHeader(ctx, lid)
}

// Lease returns the lease from the context.
func Lease(ctx context.Context) (string, bool) {
	lid, ok := ctx.Value(leaseKey{}).(string)
	if !ok {
		return fromGRPCHeader(ctx)
	}

	return lid, ok
}
