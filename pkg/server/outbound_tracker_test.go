package server

import (
	"sync"
	"testing"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

func TestOutboundTracker_NextIDMonotonic(t *testing.T) {
	tr := newOutboundTracker()
	a := tr.nextID()
	b := tr.nextID()
	c := tr.nextID()
	if !(a < b && b < c) {
		t.Errorf("IDs not monotonic: %v, %v, %v", a, b, c)
	}
}

func TestOutboundTracker_ResolveDelivers(t *testing.T) {
	tr := newOutboundTracker()
	id := tr.nextID()
	ch := tr.register(id)

	want := &protocol.Response{JSONRPC: "2.0", ID: id}
	go tr.resolve(id, want)

	select {
	case got := <-ch:
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestOutboundTracker_ResolveUnknownIDNoPanic(t *testing.T) {
	tr := newOutboundTracker()
	// Resolve before register — should not panic, just drop.
	tr.resolve(int64(99), &protocol.Response{})
}

func TestOutboundTracker_CancelUnblocksWaiter(t *testing.T) {
	tr := newOutboundTracker()
	id := tr.nextID()
	ch := tr.register(id)

	go tr.cancel(id)

	select {
	case got := <-ch:
		if got != nil {
			t.Errorf("expected nil response on cancel, got %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock waiter")
	}
}

func TestOutboundTracker_DoubleResolveIgnored(t *testing.T) {
	tr := newOutboundTracker()
	id := tr.nextID()
	ch := tr.register(id)

	tr.resolve(id, &protocol.Response{ID: id})
	// Second resolve would panic on a send to closed channel if unguarded.
	tr.resolve(id, &protocol.Response{ID: id})

	// Drain the first (and only) value to make the test deterministic.
	<-ch
}

func TestOutboundTracker_ConcurrentRegisterResolve(t *testing.T) {
	tr := newOutboundTracker()

	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := tr.nextID()
			ch := tr.register(id)
			go tr.resolve(id, &protocol.Response{ID: id})
			<-ch
		}()
	}
	wg.Wait()
}

// matchID coerces the tracker's stored key and the response's ID for equality
// — smoke test that the tracker accepts either a raw int64 (from nextID) or a
// JSON-decoded float64 coming back from the client.
func TestOutboundTracker_ResolveAcceptsFloatID(t *testing.T) {
	tr := newOutboundTracker()
	id := tr.nextID() // int64
	ch := tr.register(id)

	// JSON decoders typically produce float64 for numeric IDs.
	tr.resolve(float64(id), &protocol.Response{ID: float64(id)})

	select {
	case got := <-ch:
		if got == nil {
			t.Fatal("expected non-nil response")
		}
	case <-time.After(time.Second):
		t.Fatal("resolve with float64 id did not reach waiter")
	}
}
