package store

import "sync"

// Task notifications carry no payload: the event log remains the source of truth.
// Each subscriber holds one wake-up, so a slow UI cannot block model reception.
type taskChanges struct {
	mu          sync.Mutex
	closed      bool
	subscribers map[string]map[chan struct{}]struct{}
}

func (s *Store) SubscribeTask(id string) (<-chan struct{}, func()) {
	c := &s.changes
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan struct{}, 1)
	if c.closed {
		close(ch)
		return ch, func() {}
	}
	if c.subscribers == nil {
		c.subscribers = map[string]map[chan struct{}]struct{}{}
	}
	if c.subscribers[id] == nil {
		c.subscribers[id] = map[chan struct{}]struct{}{}
	}
	c.subscribers[id][ch] = struct{}{}
	return ch, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, ok := c.subscribers[id][ch]; ok {
			delete(c.subscribers[id], ch)
			close(ch)
		}
		if len(c.subscribers[id]) == 0 {
			delete(c.subscribers, id)
		}
	}
}
func (c *taskChanges) notify(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ch := range c.subscribers[id] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
func (c *taskChanges) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for _, subscribers := range c.subscribers {
		for ch := range subscribers {
			close(ch)
		}
	}
	c.subscribers = nil
}
