package leaseutil

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/pkg/errors"
)

func WithLease(ctx context.Context, ls leases.Manager, opts ...leases.Opt) (context.Context, func(context.Context) error, error) {
	_, ok := leases.FromContext(ctx)
	if ok {
		return ctx, func(context.Context) error {
			return nil
		}, nil
	}

	lr, ctx, err := NewLease(ctx, ls, opts...)
	if err != nil {
		return ctx, nil, err
	}

	return ctx, func(ctx context.Context) error {
		return ls.Delete(ctx, lr.l)
	}, nil
}

func NewLease(ctx context.Context, lm leases.Manager, opts ...leases.Opt) (*LeaseRef, context.Context, error) {
	l, err := lm.Create(ctx, append([]leases.Opt{leases.WithRandomID(), leases.WithExpiration(time.Hour)}, opts...)...)
	if err != nil {
		return nil, ctx, err
	}

	ctx = leases.WithLease(ctx, l.ID)
	return &LeaseRef{lm: lm, l: l}, ctx, nil
}

type LeaseRef struct {
	lm leases.Manager
	l  leases.Lease

	once      sync.Once
	resources []leases.Resource
	err       error
}

func (l *LeaseRef) Discard() error {
	return l.lm.Delete(context.Background(), l.l)
}

func (l *LeaseRef) Adopt(ctx context.Context) error {
	l.once.Do(func() {
		resources, err := l.lm.ListResources(ctx, l.l)
		if err != nil {
			l.err = err
			return
		}
		l.resources = resources
	})
	if l.err != nil {
		return l.err
	}
	currentID, ok := leases.FromContext(ctx)
	if !ok {
		return errors.Errorf("missing lease requirement for adopt")
	}
	for _, r := range l.resources {
		if err := l.lm.AddResource(ctx, leases.Lease{ID: currentID}, r); err != nil {
			return err
		}
	}
	if len(l.resources) == 0 {
		l.Discard()
		return nil
	}
	go l.Discard()
	return nil
}

func MakeTemporary(l *leases.Lease) error {
	if l.Labels == nil {
		l.Labels = map[string]string{}
	}
	l.Labels["buildkit/lease.temporary"] = time.Now().UTC().Format(time.RFC3339Nano)
	return nil
}

func WithNamespace(lm leases.Manager, ns string) *Manager {
	return &Manager{manager: lm, ns: ns}
}

type Manager struct {
	manager leases.Manager
	ns      string
}

func (l *Manager) Namespace() string {
	return l.ns
}

func (l *Manager) WithNamespace(ns string) *Manager {
	return WithNamespace(l.manager, ns)
}

func (l *Manager) Create(ctx context.Context, opts ...leases.Opt) (leases.Lease, error) {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.Create(ctx, opts...)
}

func (l *Manager) Delete(ctx context.Context, lease leases.Lease, opts ...leases.DeleteOpt) error {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.Delete(ctx, lease, opts...)
}

func (l *Manager) List(ctx context.Context, filters ...string) ([]leases.Lease, error) {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.List(ctx, filters...)
}

func (l *Manager) AddResource(ctx context.Context, lease leases.Lease, resource leases.Resource) error {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.AddResource(ctx, lease, resource)
}

func (l *Manager) DeleteResource(ctx context.Context, lease leases.Lease, resource leases.Resource) error {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.DeleteResource(ctx, lease, resource)
}

func (l *Manager) ListResources(ctx context.Context, lease leases.Lease) ([]leases.Resource, error) {
	ctx = namespaces.WithNamespace(ctx, l.ns)
	return l.manager.ListResources(ctx, lease)
}
