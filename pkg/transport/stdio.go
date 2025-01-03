package transport

import (
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
	encoder    *json.Encoder
	decoder    *json.Decoder
	requests   chan *protocol.Request
	errors     chan error
	done       chan struct{}
	mu         sync.RWMutex
	isClosed   bool
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		encoder:   json.NewEncoder(os.Stdout),
		decoder:   json.NewDecoder(os.Stdin),
		requests:  make(chan *protocol.Request),
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

func (t *StdioTransport) Receive() <-chan *protocol.Request {
	return t.requests
}

func (t *StdioTransport) Errors() <-chan error {
	return t.errors
}

func (t *StdioTransport) readLoop(ctx context.Context) {
	defer t.Stop(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		default:
			var request protocol.Request
			err := t.decoder.Decode(&request)

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
				select {
				case t.errors <- fmt.Errorf("decode error: %w", err):
				case <-ctx.Done():
					return
				case <-t.done:
					return
				default:
					log.Printf("Error decoding request: %v", err)
				}
				continue
			}

			if request.JSONRPC != "2.0" {
				select {
				case t.errors <- fmt.Errorf("invalid JSON-RPC version: %s", request.JSONRPC):
				case <-ctx.Done():
					return
				case <-t.done:
					return
				default:
					log.Printf("Invalid JSON-RPC version: %s", request.JSONRPC)
				}
				continue
			}

			select {
			case t.requests <- &request:
			case <-ctx.Done():
				return
			case <-t.done:
				return
			}
		}
	}
}
