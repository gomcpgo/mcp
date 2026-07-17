package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
	"github.com/gomcpgo/mcp/pkg/transport"
)

// Server represents an MCP server instance
type Server struct {
	options   Options
	registry  *handler.HandlerRegistry
	transport transport.Transport
	tracker   *requestTracker

	// outbound correlates server-initiated requests (e.g. elicitation/create)
	// with the response the client sends back.
	outbound *outboundTracker

	// elicitMu serializes Server.Elicit so only one elicitation is on the
	// wire at a time per server instance. See docs/mcp-elicitation-plan.md.
	elicitMu sync.Mutex

	// clientCaps is the capabilities block the client sent during
	// initialize. Used by Server.Elicit to refuse calls when the client did
	// not advertise elicitation support.
	clientCapsMu sync.RWMutex
	clientCaps   *protocol.ClientCapabilities

	// logLevel is the minimum level at which LogMessage emits
	// notifications/message. Controlled by logging/setLevel. Defaults to
	// "info". Guarded by logMu.
	logMu    sync.RWMutex
	logLevel string
}

// New creates a new MCP server instance with the provided options
func New(options Options) *Server {
	// Start with default options
	defaultOpts := DefaultOptions()

	// Override with provided options
	if options.Name != "" {
		defaultOpts.Name = options.Name
	}
	if options.Title != "" {
		defaultOpts.Title = options.Title
	}
	if options.Version != "" {
		defaultOpts.Version = options.Version
	}
	if len(options.Icons) > 0 {
		defaultOpts.Icons = options.Icons
	}
	if options.WebsiteURL != "" {
		defaultOpts.WebsiteURL = options.WebsiteURL
	}
	if options.Registry != nil {
		defaultOpts.Registry = options.Registry
	}
	if options.Transport != nil {
		defaultOpts.Transport = options.Transport
	}

	return &Server{
		options:   defaultOpts,
		registry:  defaultOpts.Registry,
		transport: defaultOpts.Transport,
		tracker:   newRequestTracker(),
		outbound:  newOutboundTracker(),
		logLevel:  protocol.LogLevelInfo,
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

	// Process requests and client responses
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

		case resp := <-s.transport.Responses():
			if resp == nil {
				log.Printf("Received nil response, shutting down")
				return nil
			}
			// Route to the outbound tracker so whichever goroutine called
			// Server.Elicit (or any future server→client request) unblocks.
			s.outbound.resolve(resp.ID, resp)
		}
	}
}

// handleRequest processes individual requests
func (s *Server) handleRequest(parent context.Context, req *protocol.Request) {
	log.Printf("MCP server req received:\n%v\n", truncatedJSON(req))

	// Notifications (no id) do not receive a response.
	if req.ID == nil {
		s.handleNotification(req)
		return
	}

	// Give the handler a cancellable context so an inbound
	// notifications/cancelled for this ID can stop it mid-flight.
	ctx, cancel := s.tracker.register(parent, req.ID)
	defer func() {
		cancel()
		s.tracker.unregister(req.ID)
	}()

	// If the client attached `_meta.progressToken`, inject a reporter bound
	// to that token so ProgressReporterFromContext(ctx).Report(...) in the
	// handler becomes an outbound notifications/progress. No token → the
	// handler-package default no-op reporter is used.
	if token := extractProgressToken(req.Params); token != nil {
		reporter := &transportProgressReporter{
			sendNotification: s.SendNotification,
			token:            token,
		}
		ctx = handler.WithProgressReporter(ctx, reporter)
	}

	// Inject an Elicitor when the client declared elicitation support during
	// initialize. Otherwise leave ctx alone and handlers see the stub
	// returning ErrElicitationNotSupported.
	if s.clientSupportsElicitation() {
		ctx = handler.WithElicitor(ctx, serverElicitor{s: s})
	}

	result, err := s.dispatchRequest(ctx, req)

	// If the client cancelled mid-flight, the handler's result (or error) is
	// stale per MCP spec — suppress the response so we don't waste bytes or
	// confuse the client.
	if s.tracker.wasCancelled(req.ID) {
		log.Printf("Request %v was cancelled; suppressing response", req.ID)
		return
	}

	if err != nil {
		s.sendError(req.ID, protocol.InternalError, err.Error())
		return
	}

	s.sendResponse(req.ID, result)
}

// dispatchRequest routes a request to the appropriate handler based on method.
func (s *Server) dispatchRequest(ctx context.Context, req *protocol.Request) (interface{}, error) {
	switch req.Method {
	case protocol.MethodInitialize:
		return s.handleInitialize(ctx, req.Params)

	case protocol.MethodPing:
		return struct{}{}, nil

	case protocol.MethodLoggingSetLevel:
		var setReq protocol.SetLevelParams
		if err := json.Unmarshal(req.Params, &setReq); err != nil {
			return nil, fmt.Errorf("invalid logging/setLevel parameters: %w", err)
		}
		if protocol.LogLevelRank(setReq.Level) < 0 {
			return nil, fmt.Errorf("unknown log level %q", setReq.Level)
		}
		s.logMu.Lock()
		s.logLevel = setReq.Level
		s.logMu.Unlock()
		return struct{}{}, nil

	case protocol.MethodToolsList:
		if s.registry.HasToolHandler() {
			return s.registry.GetToolHandler().ListTools(ctx)
		}
		return &protocol.ListToolsResponse{Tools: []protocol.Tool{}}, nil

	case protocol.MethodToolsCall:
		if !s.registry.HasToolHandler() {
			return nil, fmt.Errorf("tools not supported")
		}
		var toolReq protocol.CallToolRequest
		if err := json.Unmarshal(req.Params, &toolReq); err != nil {
			// Caller expects sendError on invalid params — but returning an
			// error here keeps the flow uniform; sendError will fire with
			// InternalError. That's a minor downgrade from InvalidParams in
			// exchange for a simpler control flow; if strict invalid-params
			// signalling matters, we can add per-case overrides later.
			return nil, fmt.Errorf("invalid tool parameters: %w", err)
		}
		return s.registry.GetToolHandler().CallTool(ctx, &toolReq)

	case protocol.MethodResourcesList:
		if s.registry.HasResourceHandler() {
			return s.registry.GetResourceHandler().ListResources(ctx)
		}
		return &protocol.ListResourcesResponse{Resources: []protocol.Resource{}}, nil

	case protocol.MethodResourcesRead:
		if !s.registry.HasResourceHandler() {
			return nil, fmt.Errorf("resources not supported")
		}
		var resourceReq protocol.ReadResourceRequest
		if err := json.Unmarshal(req.Params, &resourceReq); err != nil {
			return nil, fmt.Errorf("invalid resource parameters: %w", err)
		}
		return s.registry.GetResourceHandler().ReadResource(ctx, &resourceReq)

	case protocol.MethodPromptsList:
		if s.registry.HasPromptHandler() {
			return s.registry.GetPromptHandler().ListPrompts(ctx)
		}
		return &protocol.ListPromptsResponse{Prompts: []protocol.Prompt{}}, nil

	case protocol.MethodPromptsGet:
		if !s.registry.HasPromptHandler() {
			return nil, fmt.Errorf("prompts not supported")
		}
		var promptReq protocol.GetPromptRequest
		if err := json.Unmarshal(req.Params, &promptReq); err != nil {
			return nil, fmt.Errorf("invalid prompt parameters: %w", err)
		}
		return s.registry.GetPromptHandler().GetPrompt(ctx, &promptReq)

	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}

// handleNotification dispatches server-directed notifications. Notifications
// never receive a response per JSON-RPC semantics.
func (s *Server) handleNotification(req *protocol.Request) {
	switch req.Method {
	case protocol.MethodInitialized, protocol.NotificationInitialized:
		log.Printf("Server initialized successfully")

	case protocol.NotificationCancelled:
		var params protocol.CancelledParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Printf("Ignoring malformed notifications/cancelled: %v", err)
			return
		}
		if s.tracker.cancel(params.RequestID) {
			log.Printf("Request %v cancelled by client (reason: %q)", params.RequestID, params.Reason)
		} else {
			// No matching in-flight request; either it already completed or
			// the client sent a stale/unknown ID. Either is benign per spec.
			log.Printf("notifications/cancelled for unknown request id %v", params.RequestID)
		}

	default:
		log.Printf("Ignoring unknown notification: %s", req.Method)
	}
}

// handleInitialize processes initialization requests
func (s *Server) handleInitialize(_ context.Context, params json.RawMessage) (*protocol.InitializeResponse, error) {
	var initReq protocol.InitializeRequest
	if err := json.Unmarshal(params, &initReq); err != nil {
		return nil, fmt.Errorf("invalid initialization parameters: %w", err)
	}

	// Remember the client's capabilities so Server.Elicit (and any future
	// server→client calls) can refuse politely when the client didn't
	// advertise the matching capability.
	s.clientCapsMu.Lock()
	caps := initReq.Capabilities
	s.clientCaps = &caps
	s.clientCapsMu.Unlock()

	capabilities := protocol.Capabilities{
		// Logging is always advertised: the server may or may not emit
		// notifications/message, but supporting logging/setLevel costs
		// nothing, so every server exposes the capability.
		Logging: &protocol.LoggingInfo{},
	}
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
		ProtocolVersion: protocol.NegotiateVersion(initReq.ProtocolVersion),
		ServerInfo: protocol.ServerInfo{
			Name:       s.options.Name,
			Title:      s.options.Title,
			Version:    s.options.Version,
			Icons:      s.options.Icons,
			WebsiteURL: s.options.WebsiteURL,
		},
		Capabilities: capabilities,
	}, nil
}

// SendNotification sends a server-initiated notification to the client.
// Used for events like notifications/tools/list_changed, notifications/progress,
// notifications/message (logging), etc.
func (s *Server) SendNotification(method string, params interface{}) error {
	notification, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to build notification: %w", err)
	}
	return s.transport.SendNotification(notification)
}

// LogMessage emits a notifications/message if level is at or above the
// server's current threshold (controlled by logging/setLevel, default
// "info"). Unknown levels are silently dropped. loggerName is optional.
func (s *Server) LogMessage(level, loggerName string, data interface{}) error {
	msgRank := protocol.LogLevelRank(level)
	if msgRank < 0 {
		return nil
	}
	s.logMu.RLock()
	threshold := s.logLevel
	s.logMu.RUnlock()
	if msgRank < protocol.LogLevelRank(threshold) {
		return nil
	}
	return s.SendNotification(protocol.NotificationMessage, protocol.LogMessageParams{
		Level:  level,
		Logger: loggerName,
		Data:   data,
	})
}

// sendResponse sends a successful response
func (s *Server) sendResponse(id interface{}, result interface{}) {
	response := &protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	log.Printf("MCP server response:\n%v\n", truncatedJSON(response))
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

	log.Printf("MCP server error response:\n%v\n", truncatedJSON(response))
	if err := s.transport.Send(response); err != nil {
		log.Printf("Error sending error response: %v", err)
	}
}
