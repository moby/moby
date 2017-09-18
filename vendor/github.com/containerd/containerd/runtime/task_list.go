package runtime

import (
	"context"
	"sync"

	"github.com/containerd/containerd/namespaces"
	"github.com/pkg/errors"
)

var (
	ErrTaskNotExists     = errors.New("task does not exist")
	ErrTaskAlreadyExists = errors.New("task already exists")
)

func NewTaskList() *TaskList {
	return &TaskList{
		tasks: make(map[string]map[string]Task),
	}
}

type TaskList struct {
	mu    sync.Mutex
	tasks map[string]map[string]Task
}

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

func (l *TaskList) GetAll(ctx context.Context) ([]Task, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	var o []Task
	tasks, ok := l.tasks[namespace]
	if !ok {
		return o, nil
	}
	for _, t := range tasks {
		o = append(o, t)
	}
	return o, nil
}

func (l *TaskList) Add(ctx context.Context, t Task) error {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	return l.AddWithNamespace(namespace, t)
}

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

func (l *TaskList) Delete(ctx context.Context, t Task) {
	l.mu.Lock()
	defer l.mu.Unlock()
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return
	}
	tasks, ok := l.tasks[namespace]
	if ok {
		delete(tasks, t.ID())
	}
}
