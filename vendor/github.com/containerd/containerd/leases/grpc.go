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

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const (
	// GRPCHeader defines the header name for specifying a containerd lease.
	GRPCHeader = "containerd-lease"
)

func withGRPCLeaseHeader(ctx context.Context, lid string) context.Context {
	// also store on the grpc headers so it gets picked up by any clients
	// that are using this.
	txheader := metadata.Pairs(GRPCHeader, lid)
	md, ok := metadata.FromOutgoingContext(ctx) // merge with outgoing context.
	if !ok {
		md = txheader
	} else {
		// order ensures the latest is first in this list.
		md = metadata.Join(txheader, md)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

func fromGRPCHeader(ctx context.Context) (string, bool) {
	// try to extract for use in grpc servers.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}

	values := md[GRPCHeader]
	if len(values) == 0 {
		return "", false
	}

	return values[0], true
}
