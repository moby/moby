package logbroker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	events "github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/moby/swarmkit/v2/watch"
)

type subscription struct {
	id string

	follow bool

	mu sync.RWMutex
	wg sync.WaitGroup

	store   *store.MemoryStore
	message *api.SubscriptionMessage
	changed *watch.Queue

	ctx    context.Context
	cancel context.CancelFunc

	errors       []error
	nodes        map[string]struct{}
	pendingTasks map[string]struct{}
}

func newSubscription(store *store.MemoryStore, message *api.SubscriptionMessage, changed *watch.Queue) *subscription {
	return &subscription{
		id:           message.ID,
		follow:       message.Options != nil && message.Options.Follow,
		store:        store,
		message:      message,
		changed:      changed,
		nodes:        make(map[string]struct{}),
		pendingTasks: make(map[string]struct{}),
	}
}

func (s *subscription) ID() string {
	return s.id
}

func (s *subscription) Contains(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.nodes[nodeID]
	return ok
}

func (s *subscription) Nodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]string, 0, len(s.nodes))
	for node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *subscription) Run(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)

	if s.follow {
		wq := s.store.WatchQueue()
		ch, cancel := state.Watch(wq, api.EventCreateTask{}, api.EventUpdateTask{})
		go func() {
			defer cancel()
			_ = s.watch(ch)
		}()
	}

	s.match()
}

func (s *subscription) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *subscription) Wait(_ context.Context) <-chan struct{} {
	// Follow subscriptions never end
	if s.follow {
		return nil
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		s.wg.Wait()
	}()
	return ch
}

func (s *subscription) Done(nodeID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.errors = append(s.errors, err)
	}

	if s.follow {
		return
	}

	if _, ok := s.nodes[nodeID]; !ok {
		return
	}

	delete(s.nodes, nodeID)
	s.wg.Done()
}

func (s *subscription) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.errors) == 0 && len(s.pendingTasks) == 0 {
		return nil
	}

	messages := make([]string, 0, len(s.errors))
	for _, err := range s.errors {
		messages = append(messages, err.Error())
	}
	for t := range s.pendingTasks {
		messages = append(messages, fmt.Sprintf("task %s has not been scheduled", t))
	}

	return fmt.Errorf("warning: incomplete log stream. some logs could not be retrieved for the following reasons: %s", strings.Join(messages, ", "))
}

func (s *subscription) Close() {
	s.mu.Lock()
	s.message.Close = true
	s.mu.Unlock()
}

func (s *subscription) Closed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.message.Close
}

func (s *subscription) match() {
	s.mu.Lock()
	defer s.mu.Unlock()
	selector := s.message.Selector

	add := func(t *api.Task) {
		if t.NodeID == "" {
			s.pendingTasks[t.ID] = struct{}{}
			return
		}
		if _, ok := s.nodes[t.NodeID]; !ok {
			s.nodes[t.NodeID] = struct{}{}
			s.wg.Add(1)
		}
	}

	s.store.View(func(tx store.ReadTx) {
		for _, nid := range selector.NodeIDs {
			s.nodes[nid] = struct{}{}
		}

		for _, tid := range selector.TaskIDs {
			if task := store.GetTask(tx, tid); task != nil {
				add(task)
			}
		}

		for _, sid := range selector.ServiceIDs {
			tasks, err := store.FindTasks(tx, store.ByServiceID(sid))
			if err != nil {
				log.L.Warning(err)
				continue
			}
			for _, task := range tasks {
				// if we're not following, don't add tasks that aren't running yet
				if !s.follow && task.Status.State < api.TaskStateRunning {
					continue
				}
				add(task)
			}
		}
	})
}

func (s *subscription) watch(ch <-chan events.Event) error {
	selector := s.message.Selector

	matchTasks := map[string]struct{}{}
	for _, tid := range selector.TaskIDs {
		matchTasks[tid] = struct{}{}
	}

	matchServices := map[string]struct{}{}
	for _, sid := range selector.ServiceIDs {
		matchServices[sid] = struct{}{}
	}

	add := func(t *api.Task) {
		// this mutex does not have a deferred unlock, because there is work
		// we need to do after we release it.
		s.mu.Lock()

		// Un-allocated task.
		if t.NodeID == "" {
			s.pendingTasks[t.ID] = struct{}{}
			s.mu.Unlock()
			return
		}

		delete(s.pendingTasks, t.ID)
		if _, ok := s.nodes[t.NodeID]; !ok {
			s.nodes[t.NodeID] = struct{}{}

			s.mu.Unlock()

			// if we try to call Publish before we release the lock, we can end
			// up in a situation where the receiver is trying to acquire a read
			// lock on it. it's hard to explain.
			s.changed.Publish(s)
			return
		}

		s.mu.Unlock()
	}

	for {
		var t *api.Task
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case event := <-ch:
			switch v := event.(type) {
			case api.EventCreateTask:
				t = v.Task
			case api.EventUpdateTask:
				t = v.Task
			}
		}

		if t == nil {
			panic("received invalid task from the watch queue")
		}

		if _, ok := matchTasks[t.ID]; ok {
			add(t)
		}
		if _, ok := matchServices[t.ServiceID]; ok {
			add(t)
		}
	}
}

// Message returns a snapshot of the current subscription message.
//
// The underlying message is owned by the subscription and may be updated
// as part of its lifecycle. Callers should use this accessor rather than
// accessing the underlying message directly.
func (s *subscription) Message() *api.SubscriptionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a snapshot of the message to avoid races while it is being
	// marshaled. See https://github.com/moby/moby/issues/47322.
	msg := *s.message
	return &msg
}
