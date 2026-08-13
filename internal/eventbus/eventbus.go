package eventbus

import "sync"

// Bus allows publish-subscribe communication.
type Bus struct {
	subscribers map[string][]chan string
	mu          sync.RWMutex
}

// New creates a new Bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[string][]chan string),
	}
}

// Subscribe subscribes to a topic.
// It returns a channel on which events can be received.
func (b *Bus) Subscribe(topic string) <-chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan string, 1)
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}

// Publish publishes a message to a topic.
func (b *Bus) Publish(topic string, msg string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, found := b.subscribers[topic]; found {
		// Non-blocking send to all subscribers
		for _, ch := range subs {
			select {
			case ch <- msg:
			default:
				// Drop if the channel is full to prevent blocking
			}
		}
	}
}
