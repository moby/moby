package logbroker

import (
	"context"
	"sync"

	events "github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/watch"
)

type subscription struct {
	mu sync.RWMutex

	store   *store.MemoryStore
	message *api.SubscriptionMessage
	changed *watch.Queue

	ctx    context.Context
	cancel context.CancelFunc

	nodes map[string]struct{}
}

func newSubscription(store *store.MemoryStore, message *api.SubscriptionMessage, changed *watch.Queue) *subscription {
	return &subscription{
		store:   store,
		message: message,
		changed: changed,
		nodes:   make(map[string]struct{}),
	}
}

func (s *subscription) Contains(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.nodes[nodeID]
	return ok
}

func (s *subscription) Run(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)

	wq := s.store.WatchQueue()
	ch, cancel := state.Watch(wq, state.EventCreateTask{}, state.EventUpdateTask{})
	go func() {
		defer cancel()
		s.watch(ch)
	}()

	s.match()
}

func (s *subscription) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *subscription) match() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store.View(func(tx store.ReadTx) {
		for _, nid := range s.message.Selector.NodeIDs {
			s.nodes[nid] = struct{}{}
		}

		for _, tid := range s.message.Selector.TaskIDs {
			if task := store.GetTask(tx, tid); task != nil {
				s.nodes[task.NodeID] = struct{}{}
			}
		}

		for _, sid := range s.message.Selector.ServiceIDs {
			tasks, err := store.FindTasks(tx, store.ByServiceID(sid))
			if err != nil {
				log.L.Warning(err)
				continue
			}
			for _, task := range tasks {
				s.nodes[task.NodeID] = struct{}{}
			}
		}
	})
}

func (s *subscription) watch(ch <-chan events.Event) error {
	matchTasks := map[string]struct{}{}
	for _, tid := range s.message.Selector.TaskIDs {
		matchTasks[tid] = struct{}{}
	}

	matchServices := map[string]struct{}{}
	for _, sid := range s.message.Selector.ServiceIDs {
		matchServices[sid] = struct{}{}
	}

	add := func(nodeID string) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if _, ok := s.nodes[nodeID]; !ok {
			s.nodes[nodeID] = struct{}{}
			s.changed.Publish(s)
		}
	}

	for {
		var t *api.Task
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case event := <-ch:
			switch v := event.(type) {
			case state.EventCreateTask:
				t = v.Task
			case state.EventUpdateTask:
				t = v.Task
			}
		}

		if t == nil {
			panic("received invalid task from the watch queue")
		}

		if _, ok := matchTasks[t.ID]; ok {
			add(t.NodeID)
		}
		if _, ok := matchServices[t.ServiceID]; ok {
			add(t.NodeID)
		}
	}
}
