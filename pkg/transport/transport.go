package transport

import (
	"context"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Transport defines the interface for MCP server transport
type Transport interface {
	// Start initializes and starts the transport
	Start(ctx context.Context) error

	// Stop gracefully shuts down the transport
	Stop(ctx context.Context) error

	// Send sends a response to a client request
	Send(response *protocol.Response) error

	// SendNotification sends a server-initiated notification to the client
	SendNotification(notification *protocol.Notification) error

	// SendRequest sends a server-initiated request (with id) to the client.
	// Used for server→client methods like elicitation/create. The caller is
	// responsible for correlating the response arriving on Responses() with
	// the request's ID.
	SendRequest(request *protocol.Request) error

	// Receive returns a channel that provides incoming requests and
	// notifications from the client.
	Receive() <-chan *protocol.Request

	// Responses returns a channel that provides incoming responses to
	// server-initiated requests previously sent via SendRequest.
	Responses() <-chan *protocol.Response

	// Errors returns a channel that provides transport errors
	Errors() <-chan error
}

// Options holds configuration for transports
type Options struct {
	// Add common transport options here
}

// TransportType identifies different transport implementations
type TransportType string

const (
	TypeStdio TransportType = "stdio"
	TypeSSE   TransportType = "sse"
)
