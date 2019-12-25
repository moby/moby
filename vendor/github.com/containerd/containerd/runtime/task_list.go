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
	"sync"

	"github.com/containerd/containerd/namespaces"
	"github.com/pkg/errors"
)

var (
	// ErrTaskNotExists is returned when a task does not exist
	ErrTaskNotExists = errors.New("task does not exist")
	// ErrTaskAlreadyExists is returned when a task already exists
	ErrTaskAlreadyExists = errors.New("task already exists")
)

// NewTaskList returns a new TaskList
func NewTaskList() *TaskList {
	return &TaskList{
		tasks: make(map[string]map[string]Task),
	}
}

// TaskList holds and provides locking around tasks
type TaskList struct {
	mu    sync.Mutex
	tasks map[string]map[string]Task
}

// Get a task
func (l *TaskList) Get(ctx context.Context, id string) (Task, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	tasks, ok := l.tasks[namespace]
	if !ok {
		return nil, ErrTaskNotExists
	}
	t, ok := tasks[id]
	if !ok {
		return nil, ErrTaskNotExists
	}
	return t, nil
}

// GetAll tasks under a namespace
func (l *TaskList) GetAll(ctx context.Context, noNS bool) ([]Task, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var o []Task
	if noNS {
		for ns := range l.tasks {
			for _, t := range l.tasks[ns] {
				o = append(o, t)
			}
		}
		return o, nil
	}
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	tasks, ok := l.tasks[namespace]
	if !ok {
		return o, nil
	}
	for _, t := range tasks {
		o = append(o, t)
	}
	return o, nil
}

// Add a task
func (l *TaskList) Add(ctx context.Context, t Task) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	return l.AddWithNamespace(namespace, t)
}

// AddWithNamespace adds a task with the provided namespace
func (l *TaskList) AddWithNamespace(namespace string, t Task) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	id := t.ID()
	if _, ok := l.tasks[namespace]; !ok {
		l.tasks[namespace] = make(map[string]Task)
	}
	if _, ok := l.tasks[namespace][id]; ok {
		return errors.Wrap(ErrTaskAlreadyExists, id)
	}
	l.tasks[namespace][id] = t
	return nil
}

// Delete a task
func (l *TaskList) Delete(ctx context.Context, id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return
	}
	tasks, ok := l.tasks[namespace]
	if ok {
		delete(tasks, id)
	}
}
