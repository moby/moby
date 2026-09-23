package storagekvv0

import (
	"context"

	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/moby/extensions"
	"github.com/moby/moby/v2/errdefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// KV accesses records in one namespace.
// It owns no resources and does not need to be closed.
type KV struct {
	provider  Storage
	namespace string
}

// GetKV resolves the storage provider and binds it to namespace.
// Call it during initialization after declaring Point.Dependency().
// It performs no storage I/O; namespace validation happens on each operation.
func GetKV(resolver extensions.Resolver, namespace string) (*KV, error) {
	provider, err := Point.Single(resolver)
	if err != nil {
		return nil, err
	}
	return &KV{provider: provider, namespace: namespace}, nil
}

// Get reads a record, or returns a not-found error if it is absent.
func (kv *KV) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := kv.provider.Get(ctx, &RecordRequest{Namespace: kv.namespace, Key: key})
	if err != nil {
		return nil, recordError(err)
	}
	return resp.Value, nil
}

// Create writes a new record, or returns a conflict if the key exists.
func (kv *KV) Create(ctx context.Context, key string, value []byte) error {
	return recordError(kv.provider.Create(ctx, &WriteRequest{Namespace: kv.namespace, Key: key, Value: value}))
}

// Update replaces a record, or returns a not-found error if it is absent.
func (kv *KV) Update(ctx context.Context, key string, value []byte) error {
	return recordError(kv.provider.Update(ctx, &WriteRequest{Namespace: kv.namespace, Key: key, Value: value}))
}

// Delete removes a record; deleting an absent record succeeds.
func (kv *KV) Delete(ctx context.Context, key string) error {
	return recordError(kv.provider.Delete(ctx, &RecordRequest{Namespace: kv.namespace, Key: key}))
}

// ListOptions selects a page of keys in a KV namespace.
type ListOptions struct {
	Prefix string
	Limit  uint32
	Cursor string
}

// List returns matching keys in ascending byte order, not a snapshot.
// Reuse NextCursor with the same prefix to continue listing.
func (kv *KV) List(ctx context.Context, opts ListOptions) (*ListResponse, error) {
	resp, err := kv.provider.List(ctx, &ListRequest{
		Namespace: kv.namespace,
		Prefix:    opts.Prefix,
		Limit:     opts.Limit,
		Cursor:    opts.Cursor,
	})
	return resp, recordError(err)
}

// Preserve native errors, including context cancellation and filesystem causes.
// RPC clients need their status errors translated back to Go error categories.
func recordError(err error) error {
	s, ok := status.FromError(err)
	if !ok || err == nil {
		return err
	}
	native := errgrpc.ToNative(err)
	if s.Code() == codes.AlreadyExists {
		return errdefs.Conflict(native)
	}
	return native
}
