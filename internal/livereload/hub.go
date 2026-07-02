package livereload

import "sync"

// Hub fans out broadcast messages to all connected SSE clients.
type Hub struct {
	clients map[chan string]struct{}
	mu      sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan string]struct{}),
	}
}

func (h *Hub) addClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

func (h *Hub) removeClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
}

// Broadcast sends a message to all connected SSE clients.
func (h *Hub) Broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		select {
		case ch <- message:
		default:
			// client too slow; skip rather than block
		}
	}
}
