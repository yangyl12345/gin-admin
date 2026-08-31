package biz

import (
	"sync"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
)

type EventHub struct {
	mu   sync.RWMutex
	next uint64
	subs map[string]map[uint64]chan *schema.RunEvent
}

func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[string]map[uint64]chan *schema.RunEvent)}
}

func (a *EventHub) Subscribe(runID string) (<-chan *schema.RunEvent, func()) {
	a.mu.Lock()
	a.next++
	id := a.next
	ch := make(chan *schema.RunEvent, 32)
	if a.subs[runID] == nil {
		a.subs[runID] = make(map[uint64]chan *schema.RunEvent)
	}
	a.subs[runID][id] = ch
	a.mu.Unlock()
	return ch, func() {
		a.mu.Lock()
		if group := a.subs[runID]; group != nil {
			delete(group, id)
			if len(group) == 0 {
				delete(a.subs, runID)
			}
		}
		a.mu.Unlock()
	}
}

func (a *EventHub) Publish(event *schema.RunEvent) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, ch := range a.subs[event.RunID] {
		select {
		case ch <- event:
		default:
		}
	}
}
