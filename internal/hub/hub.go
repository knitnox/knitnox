// Package hub is an in-memory publish/subscribe bus keyed by thread ID.
// Workers publish; SSE connections subscribe. Slow subscribers drop events
// and resynchronise from the database (the client detects gaps).
package hub

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan []byte]struct{}
}

// The feed uses the same bus. Thread ids are positive, so these keys never
// collide with a thread: one channel per user for their own posts, and one for
// the posts shared with everyone.
const FeedPublic int64 = -1 << 62

// FeedUser is the channel for new posts in one user's own feed.
func FeedUser(userID int64) int64 { return -userID }

func New() *Hub { return &Hub{subs: map[int64]map[chan []byte]struct{}{}} }

// Subscribe returns a channel of JSON-encoded events and an unsubscribe func.
func (h *Hub) Subscribe(threadID int64) (<-chan []byte, func()) {
	ch := make(chan []byte, 256)
	h.mu.Lock()
	if h.subs[threadID] == nil {
		h.subs[threadID] = map[chan []byte]struct{}{}
	}
	h.subs[threadID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[threadID], ch)
		if len(h.subs[threadID]) == 0 {
			delete(h.subs, threadID)
		}
		h.mu.Unlock()
	}
}

// Publish sends an event to every subscriber of the thread without blocking.
func (h *Hub) Publish(threadID int64, typ string, data any) {
	b, err := json.Marshal(Event{Type: typ, Data: data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[threadID] {
		select {
		case ch <- b:
		default: // subscriber too slow; it will resync
		}
	}
}

// Encode is used by the SSE handler for one-off events (snapshots).
func Encode(typ string, data any) []byte {
	b, _ := json.Marshal(Event{Type: typ, Data: data})
	return b
}
