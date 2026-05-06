package widget

import (
	"sync"
)

type Update struct {
	WidgetID string
	HTML     string
}

type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan Update]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{clients: make(map[chan Update]struct{})}
}

func (b *Broadcaster) Subscribe() chan Update {
	ch := make(chan Update, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan Update) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) Publish(u Update) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- u:
		default:
			// drop slow client
		}
	}
}

func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
