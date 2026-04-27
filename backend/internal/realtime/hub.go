package realtime

import (
	"strings"
	"sync"
	"sync/atomic"
)

type Message struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Message string `json:"message,omitempty"`
	Source  string `json:"source,omitempty"`
	Event   string `json:"event,omitempty"`
	Typing  any    `json:"typing,omitempty"`
}

type senderFunc func(Message) error

type Hub struct {
	mu        sync.RWMutex
	sequence  atomic.Uint64
	listeners map[string]map[string]senderFunc
}

func NewHub() *Hub {
	return &Hub{
		listeners: make(map[string]map[string]senderFunc),
	}
}

func (h *Hub) Register(userID string, send senderFunc) func() {
	if h == nil || send == nil {
		return func() {}
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return func() {}
	}

	connectionID := h.nextConnectionID()

	h.mu.Lock()
	if h.listeners[userID] == nil {
		h.listeners[userID] = make(map[string]senderFunc)
	}
	h.listeners[userID][connectionID] = send
	h.mu.Unlock()

	return func() {
		h.remove(userID, connectionID)
	}
}

func (h *Hub) Broadcast(userID string, message Message) {
	if h == nil {
		return
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}

	h.mu.RLock()
	userListeners := h.listeners[userID]
	copies := make(map[string]senderFunc, len(userListeners))
	for id, send := range userListeners {
		copies[id] = send
	}
	h.mu.RUnlock()

	for id, send := range copies {
		if err := send(message); err != nil {
			h.remove(userID, id)
		}
	}
}

func (h *Hub) nextConnectionID() string {
	return stringID(h.sequence.Add(1))
}

func (h *Hub) remove(userID string, connectionID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	userListeners := h.listeners[userID]
	if userListeners == nil {
		return
	}

	delete(userListeners, connectionID)
	if len(userListeners) == 0 {
		delete(h.listeners, userID)
	}
}

func stringID(value uint64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}

	buffer := make([]byte, 0, 20)
	for value > 0 {
		buffer = append(buffer, digits[int(value%10)])
		value /= 10
	}

	for left, right := 0, len(buffer)-1; left < right; left, right = left+1, right-1 {
		buffer[left], buffer[right] = buffer[right], buffer[left]
	}

	return string(buffer)
}
