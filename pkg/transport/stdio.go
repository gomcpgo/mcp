package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

type StdioTransport struct {
	encoder   *json.Encoder
	reader    *bufio.Reader
	requests  chan *protocol.Request
	responses chan *protocol.Response
	errors    chan error
	done      chan struct{}
	mu        sync.RWMutex
	isClosed  bool
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		encoder:   json.NewEncoder(os.Stdout),
		reader:    bufio.NewReader(os.Stdin),
		requests:  make(chan *protocol.Request),
		responses: make(chan *protocol.Response),
		errors:    make(chan error),
		done:      make(chan struct{}),
	}
}

func (t *StdioTransport) Start(ctx context.Context) error {
	go t.readLoop(ctx)
	return nil
}

func (t *StdioTransport) Stop(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isClosed {
		t.isClosed = true
		close(t.done)
		close(t.requests)
		close(t.responses)
		close(t.errors)
	}
	return nil
}

func (t *StdioTransport) Send(response *protocol.Response) error {
	t.mu.RLock()
	if t.isClosed {
		t.mu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	t.mu.RUnlock()

	return t.encoder.Encode(response)
}

func (t *StdioTransport) SendNotification(notification *protocol.Notification) error {
	t.mu.RLock()
	if t.isClosed {
		t.mu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	t.mu.RUnlock()

	return t.encoder.Encode(notification)
}

func (t *StdioTransport) SendRequest(request *protocol.Request) error {
	t.mu.RLock()
	if t.isClosed {
		t.mu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	t.mu.RUnlock()

	return t.encoder.Encode(request)
}

func (t *StdioTransport) Receive() <-chan *protocol.Request {
	return t.requests
}

func (t *StdioTransport) Responses() <-chan *protocol.Response {
	return t.responses
}

func (t *StdioTransport) Errors() <-chan error {
	return t.errors
}

// readLoop reads JSON-encoded messages off stdin one at a time. Each message
// is routed by shape — presence of a `method` key marks it as a request or
// notification bound for the requests channel; presence of `result`/`error`
// marks it as a response bound for the responses channel. Routing by shape
// rather than structural heuristics keeps us spec-faithful: a well-formed
// response never carries a method, and a well-formed request/notification
// always does.
func (t *StdioTransport) readLoop(ctx context.Context) {
	defer t.Stop(ctx)

	dec := json.NewDecoder(t.reader)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		default:
		}

		var raw json.RawMessage
		err := dec.Decode(&raw)

		if err == io.EOF {
			return
		}

		t.mu.RLock()
		isClosed := t.isClosed
		t.mu.RUnlock()
		if isClosed {
			return
		}

		if err != nil {
			t.sendError(ctx, fmt.Errorf("decode error: %w", err))
			continue
		}

		// Peek at the keys to decide routing. Using a small envelope
		// avoids a full decode-and-reflect.
		var peek struct {
			JSONRPC string          `json:"jsonrpc"`
			Method  *string         `json:"method,omitempty"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   json.RawMessage `json:"error,omitempty"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			t.sendError(ctx, fmt.Errorf("decode envelope: %w", err))
			continue
		}

		if peek.JSONRPC != "2.0" {
			t.sendError(ctx, fmt.Errorf("invalid JSON-RPC version: %s", peek.JSONRPC))
			continue
		}

		if peek.Method != nil {
			var request protocol.Request
			if err := json.Unmarshal(raw, &request); err != nil {
				t.sendError(ctx, fmt.Errorf("decode request: %w", err))
				continue
			}
			select {
			case t.requests <- &request:
			case <-ctx.Done():
				return
			case <-t.done:
				return
			}
			continue
		}

		// No method → response (must have result or error).
		if len(peek.Result) == 0 && len(peek.Error) == 0 {
			t.sendError(ctx, fmt.Errorf("message has no method, result, or error"))
			continue
		}

		var response protocol.Response
		if err := json.Unmarshal(raw, &response); err != nil {
			t.sendError(ctx, fmt.Errorf("decode response: %w", err))
			continue
		}
		select {
		case t.responses <- &response:
		case <-ctx.Done():
			return
		case <-t.done:
			return
		}
	}
}

// sendError pushes err onto the errors channel if a receiver is ready,
// otherwise logs it. Mirrors the prior readLoop's behaviour.
func (t *StdioTransport) sendError(ctx context.Context, err error) {
	select {
	case t.errors <- err:
	case <-ctx.Done():
	case <-t.done:
	default:
		log.Printf("transport: %v", err)
	}
}
