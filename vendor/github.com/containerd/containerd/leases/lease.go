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
	"time"
)

// Opt is used to set options on a lease
type Opt func(*Lease) error

// DeleteOpt allows configuring a delete operation
type DeleteOpt func(context.Context, *DeleteOptions) error

// Manager is used to create, list, and remove leases
type Manager interface {
	Create(context.Context, ...Opt) (Lease, error)
	Delete(context.Context, Lease, ...DeleteOpt) error
	List(context.Context, ...string) ([]Lease, error)
}

// Lease retains resources to prevent cleanup before
// the resources can be fully referenced.
type Lease struct {
	ID        string
	CreatedAt time.Time
	Labels    map[string]string
}

// DeleteOptions provide options on image delete
type DeleteOptions struct {
	Synchronous bool
}

// SynchronousDelete is used to indicate that a lease deletion and removal of
// any unreferenced resources should occur synchronously before returning the
// result.
func SynchronousDelete(ctx context.Context, o *DeleteOptions) error {
	o.Synchronous = true
	return nil
}

// WithLabels sets labels on a lease
func WithLabels(labels map[string]string) Opt {
	return func(l *Lease) error {
		l.Labels = labels
		return nil
	}
}

// WithExpiration sets an expiration on the lease
func WithExpiration(d time.Duration) Opt {
	return func(l *Lease) error {
		if l.Labels == nil {
			l.Labels = map[string]string{}
		}
		l.Labels["containerd.io/gc.expire"] = time.Now().Add(d).Format(time.RFC3339)

		return nil
	}
}
