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
	encoder   *json.Encoder
	decoder   *json.Decoder
	requests  chan *protocol.Request
	errors    chan error
	done      chan struct{}
	closeOnce sync.Once
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		encoder:  json.NewEncoder(os.Stdout),
		decoder:  json.NewDecoder(os.Stdin),
		requests: make(chan *protocol.Request),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}
}

func (t *StdioTransport) Start(ctx context.Context) error {
	go t.readLoop(ctx)
	return nil
}

func (t *StdioTransport) Stop(ctx context.Context) error {
	t.closeOnce.Do(func() {
		close(t.done)
		close(t.requests)
		close(t.errors)
	})
	return nil
}

func (t *StdioTransport) Send(response *protocol.Response) error {
	return t.encoder.Encode(response)
}

func (t *StdioTransport) Receive() <-chan *protocol.Request {
	return t.requests
}

func (t *StdioTransport) Errors() <-chan error {
	return t.errors
}

func (t *StdioTransport) readLoop(ctx context.Context) {
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
				t.Stop(ctx)
				return
			}

			if err != nil {
				log.Printf("Error decoding request: %v", err)
				t.errors <- fmt.Errorf("decode error: %w", err)
				continue
			}

			// Validate JSON-RPC version
			if request.JSONRPC != "2.0" {
				t.errors <- fmt.Errorf("invalid JSON-RPC version: %s", request.JSONRPC)
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
