package buildid

import (
	"context"

	"google.golang.org/grpc/metadata"
)

var metadataKey = "buildkit-controlapi-buildid"

func AppendToOutgoingContext(ctx context.Context, id string) context.Context {
	if id != "" {
		return metadata.AppendToOutgoingContext(ctx, metadataKey, id)
	}
	return ctx
}

func FromIncomingContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	if ids := md.Get(metadataKey); len(ids) == 1 {
		return ids[0]
	}

	return ""
}
