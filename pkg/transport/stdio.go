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

func (t *StdioTransport) Stop(ctx context.Context) error {
	t.mu.Lock()
	if !t.isClosed {
		t.isClosed = true
		close(t.done)
	}
	t.mu.Unlock()
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
	defer func() {
		t.mu.Lock()
		if !t.isClosed {
			t.isClosed = true
			close(t.requests)
			close(t.errors)
		}
		t.mu.Unlock()
	}()

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

			if err != nil {
				t.mu.RLock()
				if !t.isClosed {
					select {
					case t.errors <- fmt.Errorf("decode error: %w", err):
					default:
						log.Printf("Error decoding request: %v", err)
					}
				}
				t.mu.RUnlock()
				continue
			}

			if request.JSONRPC != "2.0" {
				t.mu.RLock()
				if !t.isClosed {
					select {
					case t.errors <- fmt.Errorf("invalid JSON-RPC version: %s", request.JSONRPC):
					default:
						log.Printf("Invalid JSON-RPC version: %s", request.JSONRPC)
					}
				}
				t.mu.RUnlock()
				continue
			}

			t.mu.RLock()
			if !t.isClosed {
				select {
				case t.requests <- &request:
				case <-ctx.Done():
					t.mu.RUnlock()
					return
				case <-t.done:
					t.mu.RUnlock()
					return
				default:
					log.Printf("Request channel full, dropping request")
				}
			}
			t.mu.RUnlock()
		}
	}
}
