package protocol

import (
	"encoding/base64"
	"encoding/json"
)

// JSON-RPC 2.0 message types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification: method + optional params, no id.
// Used for messages that do not expect a response (e.g. notifications/initialized,
// notifications/tools/list_changed, notifications/progress).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewNotification constructs a Notification with JSONRPC "2.0" and marshals
// the given params. Pass nil for params to omit them.
func NewNotification(method string, params interface{}) (*Notification, error) {
	n := &Notification{
		JSONRPC: "2.0",
		Method:  method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		n.Params = raw
	}
	return n, nil
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Protocol types. Title, Icons, and WebsiteURL are MCP 2025-11-25
// additions to the Implementation type; older servers omit them.
type ServerInfo struct {
	Name       string `json:"name"`
	Title      string `json:"title,omitempty"`
	Version    string `json:"version"`
	Icons      []Icon `json:"icons,omitempty"`
	WebsiteURL string `json:"websiteUrl,omitempty"`
}

type Capabilities struct {
	Tools     *ToolsInfo     `json:"tools,omitempty"`
	Resources *ResourcesInfo `json:"resources,omitempty"`
	Prompts   *PromptsInfo   `json:"prompts,omitempty"`
	Logging   *LoggingInfo   `json:"logging,omitempty"`
}

// LoggingInfo is the capability marker the server sends when it supports
// logging/setLevel and notifications/message. Empty per spec — presence alone
// advertises support.
type LoggingInfo struct{}

type ToolsInfo struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesInfo struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsInfo struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ClientInfo identifies the MCP client making the connection
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities are the capabilities declared by the client
type ClientCapabilities struct {
	// Elicitation, when non-nil, tells the server it may send
	// elicitation/create requests to prompt the user for structured input
	// mid-tool-call. Per spec, presence alone signals support; the struct
	// itself is empty today.
	Elicitation *ElicitationClientCapabilities `json:"elicitation,omitempty"`
}

// ElicitationClientCapabilities is the empty marker struct the client sends
// under capabilities.elicitation when it supports elicitation/create.
type ElicitationClientCapabilities struct{}

// Initialize types
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
	Capabilities    ClientCapabilities `json:"capabilities"`
}

type InitializeResponse struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

// Tool types
type Tool struct {
	Name         string                 `json:"name"`
	Title        string                 `json:"title,omitempty"`
	Description  string                 `json:"description"`
	InputSchema  json.RawMessage        `json:"inputSchema"`
	OutputSchema json.RawMessage        `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations       `json:"annotations,omitempty"`
	Icons        []Icon                 `json:"icons,omitempty"`
	Meta         map[string]interface{} `json:"_meta,omitempty"`
}

// ToolAnnotations carry optional hints about a tool's behaviour. Pointer
// fields distinguish "unset" from an explicit boolean value, which matters
// because the MCP spec defines permissive defaults (e.g. destructiveHint
// defaults to true when absent); Savant only acts on explicit true values
// and treats unset as "no hint".
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// Icon describes an optional visual representation of a tool or server.
// Clients may choose among multiple Icons based on the Sizes field. Per the
// MCP spec, sizes is an array of WebApp-manifest-style size strings (e.g.
// ["48x48", "96x96"] or ["any"] for scalable SVG).
type Icon struct {
	Src      string   `json:"src"`
	MimeType string   `json:"mimeType,omitempty"`
	Sizes    []string `json:"sizes,omitempty"`
}

// IconFromSVG wraps an in-memory SVG byte slice as a one-entry []Icon with a
// base64 data URI. Each MCP server embeds its own asset and calls this to
// populate its server.Options.Icons — the framework does not ship or look up
// icons on a server's behalf.
func IconFromSVG(svg []byte) []Icon {
	return []Icon{{
		Src:      "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg),
		MimeType: "image/svg+xml",
		Sizes:    []string{"any"},
	}}
}

type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResponse struct {
	Content           []ToolContent          `json:"content"`
	StructuredContent map[string]interface{} `json:"structuredContent,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
	Meta              map[string]interface{} `json:"_meta,omitempty"`
}

type ToolContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// Resource types
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ListResourcesResponse struct {
	Resources []Resource `json:"resources"`
}

type ReadResourceRequest struct {
	URI string `json:"uri"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type ReadResourceResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// Prompt types
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type ListPromptsResponse struct {
	Prompts []Prompt `json:"prompts"`
}

type GetPromptRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type GetPromptResponse struct {
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string         `json:"role"`
	Content MessageContent `json:"content"`
}

type MessageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
}

// Constants
const (
	Version                 = "2025-11-25"
	MethodInitialize        = "initialize"
	NotificationInitialized = "notifications/initialized"
	MethodInitialized       = "initialized"
	MethodToolsList         = "tools/list"
	MethodToolsCall         = "tools/call"
	MethodResourcesList     = "resources/list"
	MethodResourcesRead     = "resources/read"
	MethodPromptsList       = "prompts/list"
	MethodPromptsGet        = "prompts/get"

	// MethodElicitationCreate is the MCP 2025-11-25 server→client request
	// sent when a tool handler needs to collect structured input from the
	// user mid-execution. Only sent if the client advertised the
	// `elicitation` capability during initialize.
	MethodElicitationCreate = "elicitation/create"

	// NotificationCancelled is the MCP 2025-11-25 notifications/cancelled
	// message a peer emits to tell the other side it has abandoned an
	// in-flight request and the recipient should stop processing it.
	NotificationCancelled = "notifications/cancelled"

	// NotificationProgress is the MCP 2025-11-25 notifications/progress
	// message the server emits to report progress for a long-running
	// request that was invoked with a `_meta.progressToken`.
	NotificationProgress = "notifications/progress"

	// MethodPing is the MCP 2025-11-25 ping request. Either peer MAY send
	// it with an id; the receiver MUST reply with an empty result.
	MethodPing = "ping"

	// MethodLoggingSetLevel is the MCP 2025-11-25 client→server request used
	// to change the minimum level the server should emit via
	// notifications/message.
	MethodLoggingSetLevel = "logging/setLevel"

	// NotificationMessage is the MCP 2025-11-25 server→client notification
	// carrying a structured log line (level, logger, data).
	NotificationMessage = "notifications/message"
)

// CancelledParams are the params carried by notifications/cancelled.
type CancelledParams struct {
	RequestID interface{} `json:"requestId"`
	Reason    string      `json:"reason,omitempty"`
}

// ElicitationAction values carried by an ElicitationResult. Per MCP spec,
// `content` is present only when Action == ElicitationActionAccept.
const (
	ElicitationActionAccept  = "accept"
	ElicitationActionDecline = "decline"
	ElicitationActionCancel  = "cancel"
)

// ElicitationRequestParams is the payload of an elicitation/create request.
// RequestedSchema is kept as raw JSON — the spec restricts it to a flat
// object with primitive properties, but that's the server author's contract;
// the framework forwards it verbatim.
type ElicitationRequestParams struct {
	Message         string          `json:"message"`
	RequestedSchema json.RawMessage `json:"requestedSchema"`
}

// ElicitationResult is the client's reply to elicitation/create. Content is
// populated only when Action == ElicitationActionAccept.
type ElicitationResult struct {
	Action  string                 `json:"action"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// ProgressParams are the params carried by notifications/progress. The spec
// lets progressToken be a string or number; total is optional — omit it for
// indeterminate progress. Progress SHOULD increase monotonically but the
// framework does not enforce this.
type ProgressParams struct {
	ProgressToken interface{} `json:"progressToken"`
	Progress      float64     `json:"progress"`
	Total         *float64    `json:"total,omitempty"`
	Message       string      `json:"message,omitempty"`
}

// SupportedVersions lists protocol versions this server framework can handle.
// Ordered latest-first; used for version negotiation during initialize.
var SupportedVersions = []string{
	"2025-11-25",
	"2024-11-05",
}

// NegotiateVersion returns the version to use in the initialize response.
// If the client's requested version is supported, return it; otherwise return
// the server's latest supported version and let the client decide to proceed.
func NegotiateVersion(clientVersion string) string {
	for _, v := range SupportedVersions {
		if v == clientVersion {
			return v
		}
	}
	return SupportedVersions[0]
}

// Error codes as per JSON-RPC 2.0
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)
