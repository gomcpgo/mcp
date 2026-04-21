package server

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// outboundTracker correlates a server-initiated request's ID with the
// response channel the caller is blocked on. Used by Server.Elicit and any
// future server→client request methods. Keys are normalised via
// fmt.Sprintf("%v", id) so a tracker registered with int64 matches a
// response whose ID arrived as float64 after JSON round-trip.
type outboundTracker struct {
	counter int64

	mu      sync.Mutex
	pending map[string]chan *protocol.Response
}

func newOutboundTracker() *outboundTracker {
	return &outboundTracker{
		pending: make(map[string]chan *protocol.Response),
	}
}

// nextID mints the next monotonic request ID. Safe for concurrent use.
func (t *outboundTracker) nextID() int64 {
	return atomic.AddInt64(&t.counter, 1)
}

// register allocates a 1-buffered channel keyed by id and returns it. The
// caller blocks on the channel; resolve or cancel unblocks it.
func (t *outboundTracker) register(id interface{}) <-chan *protocol.Response {
	ch := make(chan *protocol.Response, 1)
	key := trackerKey(id)
	t.mu.Lock()
	t.pending[key] = ch
	t.mu.Unlock()
	return ch
}

// resolve delivers resp to the channel registered under id, then removes the
// entry. A second resolve for the same id is silently dropped so a late /
// duplicate response cannot panic on a send to a closed channel. Unknown IDs
// are likewise dropped.
func (t *outboundTracker) resolve(id interface{}, resp *protocol.Response) {
	key := trackerKey(id)
	t.mu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.mu.Unlock()

	if !ok {
		return
	}
	// Channel is 1-buffered and we just removed the entry, so this send
	// cannot block and no second resolver can reach it.
	ch <- resp
	close(ch)
}

// cancel unblocks the waiter on id by delivering nil, then removes the
// entry. Used when the caller's ctx is cancelled or the elicitation times
// out.
func (t *outboundTracker) cancel(id interface{}) {
	key := trackerKey(id)
	t.mu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.mu.Unlock()

	if !ok {
		return
	}
	ch <- nil
	close(ch)
}

func trackerKey(id interface{}) string {
	return fmt.Sprintf("%v", id)
}
