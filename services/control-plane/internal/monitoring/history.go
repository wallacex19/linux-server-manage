package monitoring

import (
	"sync"
)

type HistoryBuffer struct {
	mu       sync.RWMutex
	capacity int
	items    []*ServerMetricsSnapshot
	latest   *ServerMetricsSnapshot
}

func NewHistoryBuffer(capacity int) *HistoryBuffer {
	if capacity <= 0 {
		capacity = 60
	}
	return &HistoryBuffer{
		capacity: capacity,
		items:    make([]*ServerMetricsSnapshot, 0, capacity),
	}
}

func (h *HistoryBuffer) Push(snapshot *ServerMetricsSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.latest = snapshot
	if len(h.items) >= h.capacity {
		// Remove o elemento mais antigo
		h.items = h.items[1:]
	}
	h.items = append(h.items, snapshot)
}

func (h *HistoryBuffer) Latest() *ServerMetricsSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.latest
}

func (h *HistoryBuffer) All() []*ServerMetricsSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ServerMetricsSnapshot, len(h.items))
	copy(result, h.items)
	return result
}
