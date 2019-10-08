package leaseutil

import (
	"context"
	"time"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
)

func WithLease(ctx context.Context, ls leases.Manager, opts ...leases.Opt) (context.Context, func(context.Context) error, error) {
	_, ok := leases.FromContext(ctx)
	if ok {
		return ctx, func(context.Context) error {
			return nil
		}, nil
	}

	l, err := ls.Create(ctx, append([]leases.Opt{leases.WithRandomID(), leases.WithExpiration(time.Hour)}, opts...)...)
	if err != nil {
		return nil, nil, err
	}

	ctx = leases.WithLease(ctx, l.ID)
	return ctx, func(ctx context.Context) error {
		return ls.Delete(ctx, l)
	}, nil
}

func WithNamespace(lm leases.Manager, ns string) leases.Manager {
	return &nsLM{Manager: lm, ns: ns}
}

type nsLM struct {
	leases.Manager
	ns string
}

func (l *nsLM) Create(ctx context.Context, opts ...leases.Opt) (leases.Lease, error) {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.Manager.Create(ctx, opts...)
}

func (l *nsLM) Delete(ctx context.Context, lease leases.Lease, opts ...leases.DeleteOpt) error {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.Manager.Delete(ctx, lease, opts...)
}

func (l *nsLM) List(ctx context.Context, filters ...string) ([]leases.Lease, error) {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.Manager.List(ctx, filters...)
}
