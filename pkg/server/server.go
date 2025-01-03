package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
	"github.com/gomcpgo/mcp/pkg/transport"
)

// Server options for configuration
type Options struct {
	Name        string
	Version     string
	Registry    *handler.HandlerRegistry
	Transport   transport.Transport
}

// Server represents an MCP server instance
type Server struct {
	options  Options
	registry *handler.HandlerRegistry
	transport transport.Transport
}

// New creates a new MCP server instance
func New(options Options) *Server {
	if options.Registry == nil {
		options.Registry = handler.NewHandlerRegistry()
	}
	
	if options.Transport == nil {
		options.Transport = transport.NewStdioTransport()
	}

	return &Server{
		options:   options,
		registry:  options.Registry,
		transport: options.Transport,
	}
}

// Run starts the server and handles requests
func (s *Server) Run() error {
	ctx := context.Background()

	// Start transport
	if err := s.transport.Start(ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}
	defer s.transport.Stop(ctx)

	// Process requests
	for {
		select {
		case err := <-s.transport.Errors():
			log.Printf("Transport error: %v", err)
			continue

		case req := <-s.transport.Receive():
			if req == nil {
				log.Printf("Received nil request, shutting down")
				return nil
			}

			go s.handleRequest(ctx, req)
		}
	}
}

// handleRequest processes individual requests
func (s *Server) handleRequest(ctx context.Context, req *protocol.Request) {
	var result interface{}
	var err error

	switch req.Method {
	case protocol.MethodInitialize:
		result, err = s.handleInitialize(ctx, req.Params)

	case protocol.MethodToolsList:
		if s.registry.HasToolHandler() {
			result, err = s.registry.GetToolHandler().ListTools(ctx)
		} else {
			err = fmt.Errorf("tools not supported")
		}

	case protocol.MethodToolsCall:
		if s.registry.HasToolHandler() {
			var toolReq protocol.CallToolRequest
			if err := json.Unmarshal(req.Params, &toolReq); err != nil {
				s.sendError(req.ID, protocol.InvalidParams, "Invalid tool parameters")
				return
			}
			result, err = s.registry.GetToolHandler().CallTool(ctx, &toolReq)
		} else {
			err = fmt.Errorf("tools not supported")
		}

	case protocol.MethodResourcesList:
		if s.registry.HasResourceHandler() {
			result, err = s.registry.GetResourceHandler().ListResources(ctx)
		} else {
			err = fmt.Errorf("resources not supported")
		}

	case protocol.MethodResourcesRead:
		if s.registry.HasResourceHandler() {
			var resourceReq protocol.ReadResourceRequest
			if err := json.Unmarshal(req.Params, &resourceReq); err != nil {
				s.sendError(req.ID, protocol.InvalidParams, "Invalid resource parameters")
				return
			}
			result, err = s.registry.GetResourceHandler().ReadResource(ctx, &resourceReq)
		} else {
			err = fmt.Errorf("resources not supported")
		}

	case protocol.MethodPromptsList:
		if s.registry.HasPromptHandler() {
			result, err = s.registry.GetPromptHandler().ListPrompts(ctx)
		} else {
			err = fmt.Errorf("prompts not supported")
		}

	case protocol.MethodPromptsGet:
		if s.registry.HasPromptHandler() {
			var promptReq protocol.GetPromptRequest
			if err := json.Unmarshal(req.Params, &promptReq); err != nil {
				s.sendError(req.ID, protocol.InvalidParams, "Invalid prompt parameters")
				return
			}
			result, err = s.registry.GetPromptHandler().GetPrompt(ctx, &promptReq)
		} else {
			err = fmt.Errorf("prompts not supported")
		}

	case protocol.MethodInitialized:
		// Initialization notification, no response needed
		return

	default:
		err = fmt.Errorf("unknown method: %s", req.Method)
	}

	if err != nil {
		s.sendError(req.ID, protocol.InternalError, err.Error())
		return
	}

	s.sendResponse(req.ID, result)
}

// handleInitialize processes initialization requests
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (*protocol.InitializeResponse, error) {
	var initReq protocol.InitializeRequest
	if err := json.Unmarshal(params, &initReq); err != nil {
		return nil, fmt.Errorf("invalid initialization parameters: %w", err)
	}

	capabilities := protocol.Capabilities{}
	if s.registry.HasToolHandler() {
		capabilities.Tools = &protocol.ToolsInfo{}
	}
	if s.registry.HasResourceHandler() {
		capabilities.Resources = &protocol.ResourcesInfo{}
	}
	if s.registry.HasPromptHandler() {
		capabilities.Prompts = &protocol.PromptsInfo{}
	}

	return &protocol.InitializeResponse{
		ProtocolVersion: protocol.Version,
		ServerInfo: protocol.ServerInfo{
			Name:    s.options.Name,
			Version: s.options.Version,
		},
		Capabilities: capabilities,
	}, nil
}

// sendResponse sends a successful response
func (s *Server) sendResponse(id interface{}, result interface{}) {
	response := &protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	if err := s.transport.Send(response); err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

// sendError sends an error response
func (s *Server) sendError(id interface{}, code int, message string) {
	response := &protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &protocol.Error{
			Code:    code,
			Message: message,
		},
	}

	if err := s.transport.Send(response); err != nil {
		log.Printf("Error sending error response: %v", err)
	}
}
