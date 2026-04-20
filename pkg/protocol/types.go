package protocol

import "encoding/json"

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

// MCP Protocol types
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools     *ToolsInfo     `json:"tools,omitempty"`
	Resources *ResourcesInfo `json:"resources,omitempty"`
	Prompts   *PromptsInfo   `json:"prompts,omitempty"`
}

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
	// Intentionally minimal — will expand when elicitation, sampling, etc. are added
}

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
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResponse struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
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

	// NotificationCancelled is the MCP 2025-11-25 notifications/cancelled
	// message a peer emits to tell the other side it has abandoned an
	// in-flight request and the recipient should stop processing it.
	NotificationCancelled = "notifications/cancelled"
)

// CancelledParams are the params carried by notifications/cancelled.
type CancelledParams struct {
	RequestID interface{} `json:"requestId"`
	Reason    string      `json:"reason,omitempty"`
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
