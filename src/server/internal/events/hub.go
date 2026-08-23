package events

import (
	"sync"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
)

const replayLimit = 1000

type Subscription struct {
	C      <-chan model.EventEnvelope
	c      chan model.EventEnvelope
	cancel func()
}

func (s *Subscription) Close() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

type Hub struct {
	mu      sync.Mutex
	nextID  uint64
	replay  []model.EventEnvelope
	nextSub uint64
	subs    map[uint64]*subscriber
}

type subscriber struct {
	id uint64
	c  chan model.EventEnvelope
}

func NewHub() *Hub { return &Hub{subs: make(map[uint64]*subscriber)} }

func (h *Hub) Publish(eventType string, data any) model.EventEnvelope {
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.newEventLocked(eventType, data)
	for id, sub := range h.subs {
		if !trySend(sub.c, e) {
			h.dropSlowLocked(id)
		}
	}
	return e
}

func (h *Hub) Subscribe(lastID uint64) *Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextSub++
	sub := &subscriber{id: h.nextSub, c: make(chan model.EventEnvelope, 64)}
	h.subs[sub.id] = sub
	if lastID > 0 {
		if len(h.replay) == 0 || lastID < h.replay[0].Sequence-1 {
			h.sendReplayOnlyLocked(sub, "replay_gap")
		} else {
			for _, event := range h.replay {
				if event.Sequence > lastID {
					if !trySend(sub.c, event) {
						h.dropSlowLocked(sub.id)
						break
					}
				}
			}
		}
	} else {
		e := h.newEventLocked("hello", map[string]any{})
		if !trySend(sub.c, e) {
			h.dropSlowLocked(sub.id)
		}
	}
	return &Subscription{C: sub.c, c: sub.c, cancel: func() { h.unsubscribe(sub.id) }}
}

func (h *Hub) unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sub, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(sub.c)
	}
}

func (h *Hub) newEventLocked(eventType string, data any) model.EventEnvelope {
	h.nextID++
	e := model.EventEnvelope{Sequence: h.nextID, GeneratedAt: time.Now().UTC(), Type: eventType, Data: data}
	h.replay = append(h.replay, e)
	if len(h.replay) > replayLimit {
		h.replay = h.replay[len(h.replay)-replayLimit:]
	}
	return e
}

func (h *Hub) sendReplayOnlyLocked(sub *subscriber, reason string) {
	e := h.newEventLocked("resync", model.ResyncEventData{Reason: reason})
	_ = trySend(sub.c, e)
}

func (h *Hub) dropSlowLocked(id uint64) {
	sub, ok := h.subs[id]
	if !ok {
		return
	}
	delete(h.subs, id)
	for len(sub.c) > 0 {
		<-sub.c
	}
	h.nextID++
	e := model.EventEnvelope{Sequence: h.nextID, GeneratedAt: time.Now().UTC(), Type: "resync", Data: model.ResyncEventData{Reason: "slow_consumer"}}
	h.replay = append(h.replay, e)
	if len(h.replay) > replayLimit {
		h.replay = h.replay[len(h.replay)-replayLimit:]
	}
	_ = trySend(sub.c, e)
	close(sub.c)
}

func trySend(c chan model.EventEnvelope, e model.EventEnvelope) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	select {
	case c <- e:
		return true
	default:
		return false
	}
}

func (h *Hub) ReplaySize() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.replay) }
