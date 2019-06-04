package leaseutil

import (
	"context"
	"time"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	bolt "go.etcd.io/bbolt"
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

func NewManager(mdb *metadata.DB) leases.Manager {
	return &local{db: mdb}
}

type local struct {
	db *metadata.DB
}

func (l *local) Create(ctx context.Context, opts ...leases.Opt) (leases.Lease, error) {
	var lease leases.Lease
	if err := l.db.Update(func(tx *bolt.Tx) error {
		var err error
		lease, err = metadata.NewLeaseManager(tx).Create(ctx, opts...)
		return err
	}); err != nil {
		return leases.Lease{}, err
	}
	return lease, nil
}

func (l *local) Delete(ctx context.Context, lease leases.Lease, opts ...leases.DeleteOpt) error {
	var do leases.DeleteOptions
	for _, opt := range opts {
		if err := opt(ctx, &do); err != nil {
			return err
		}
	}

	if err := l.db.Update(func(tx *bolt.Tx) error {
		return metadata.NewLeaseManager(tx).Delete(ctx, lease)
	}); err != nil {
		return err
	}

	return nil

}

func (l *local) List(ctx context.Context, filters ...string) ([]leases.Lease, error) {
	var ll []leases.Lease
	if err := l.db.View(func(tx *bolt.Tx) error {
		var err error
		ll, err = metadata.NewLeaseManager(tx).List(ctx, filters...)
		return err
	}); err != nil {
		return nil, err
	}
	return ll, nil
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
