package libcontainerd

// pauseMonitor is helper to get notifications from pause state changes.
type pauseMonitor struct {
	waiters map[string][]chan struct{}
}

func (m *pauseMonitor) handle(t string) {
	if m.waiters == nil {
		return
	}
	q, ok := m.waiters[t]
	if !ok {
		return
	}
	if len(q) > 0 {
		close(q[0])
		m.waiters[t] = q[1:]
	}
}

func (m *pauseMonitor) append(t string, waiter chan struct{}) {
	if m.waiters == nil {
		m.waiters = make(map[string][]chan struct{})
	}
	_, ok := m.waiters[t]
	if !ok {
		m.waiters[t] = make([]chan struct{}, 0)
	}
	m.waiters[t] = append(m.waiters[t], waiter)
}
