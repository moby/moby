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

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
)

type object interface {
	ID() string
}

// NSMap extends Map type with a notion of namespaces passed via Context.
type NSMap[T object] struct {
	mu      sync.RWMutex
	objects map[string]map[string]T
}

// NewNSMap returns a new NSMap
func NewNSMap[T object]() *NSMap[T] {
	return &NSMap[T]{
		objects: make(map[string]map[string]T),
	}
}

// Get a task
func (m *NSMap[T]) Get(ctx context.Context, id string) (T, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	var t T
	if err != nil {
		return t, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks, ok := m.objects[namespace]
	if !ok {
		return t, errdefs.ErrNotFound
	}
	t, ok = tasks[id]
	if !ok {
		return t, errdefs.ErrNotFound
	}
	return t, nil
}

// GetAll objects under a namespace
func (m *NSMap[T]) GetAll(ctx context.Context, noNS bool) ([]T, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var o []T
	if noNS {
		for ns := range m.objects {
			for _, t := range m.objects[ns] {
				o = append(o, t)
			}
		}
		return o, nil
	}
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	tasks, ok := m.objects[namespace]
	if !ok {
		return o, nil
	}
	for _, t := range tasks {
		o = append(o, t)
	}
	return o, nil
}

// Add a task
func (m *NSMap[T]) Add(ctx context.Context, t T) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	return m.AddWithNamespace(namespace, t)
}

// AddWithNamespace adds a task with the provided namespace
func (m *NSMap[T]) AddWithNamespace(namespace string, t T) error {
	id := t.ID()

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[namespace]; !ok {
		m.objects[namespace] = make(map[string]T)
	}
	if _, ok := m.objects[namespace][id]; ok {
		return fmt.Errorf("%s: %w", id, errdefs.ErrAlreadyExists)
	}
	m.objects[namespace][id] = t
	return nil
}

// Delete a task
func (m *NSMap[T]) Delete(ctx context.Context, id string) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	tasks, ok := m.objects[namespace]
	if ok {
		delete(tasks, id)
	}
}

func (m *NSMap[T]) IsEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for ns := range m.objects {
		if len(m.objects[ns]) > 0 {
			return false
		}
	}

	return true
}
