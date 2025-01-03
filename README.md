# Go MCP Framework

A Go framework for building Model Context Protocol (MCP) servers that enable Large Language Models (LLMs) to securely access tools and data sources.

## Installation

```bash
go get github.com/gomcpgo/mcp
```

## Quick Start

Here's a minimal example of creating an MCP server:

```go
package main

import (
    "context"
    "log"

    "github.com/gomcpgo/mcp/pkg/handler"
    "github.com/gomcpgo/mcp/pkg/server"
    "github.com/gomcpgo/mcp/pkg/protocol"
)

func main() {
    // Create handler registry
    registry := handler.NewHandlerRegistry()

    // Register your tool handler
    registry.RegisterToolHandler(&MyToolHandler{})

    // Create and start server
    srv := server.New(server.Options{
        Name:     "my-server",
        Version:  "1.0.0",
        Registry: registry,
    })

    if err := srv.Run(); err != nil {
        log.Fatal(err)
    }
}

// MyToolHandler implements the ToolHandler interface
type MyToolHandler struct{}

func (h *MyToolHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
    return &protocol.ListToolsResponse{
        Tools: []protocol.Tool{
            {
                Name:        "my-tool",
                Description: "Description of my tool",
                InputSchema: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "param1": {
                            "type": "string",
                            "description": "Description of param1"
                        }
                    },
                    "required": ["param1"]
                }`),
            },
        },
    }, nil
}

func (h *MyToolHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
    // Handle tool execution
    result := "Tool execution result"
    
    return &protocol.CallToolResponse{
        Content: []protocol.ToolContent{
            {
                Type: "text",
                Text: result,
            },
        },
    }, nil
}
```

## Handler Types

The framework supports three types of handlers:

### 1. Tool Handler

For implementing tools that LLMs can execute:

```go
type ToolHandler interface {
    ListTools(ctx context.Context) (*ListToolsResponse, error)
    CallTool(ctx context.Context, req *CallToolRequest) (*CallToolResponse, error)
}
```

### 2. Resource Handler

For exposing data that LLMs can read:

```go
type ResourceHandler interface {
    ListResources(ctx context.Context) (*ListResourcesResponse, error)
    ReadResource(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResponse, error)
}
```

### 3. Prompt Handler

For providing prompt templates:

```go
type PromptHandler interface {
    ListPrompts(ctx context.Context) (*ListPromptsResponse, error)
    GetPrompt(ctx context.Context, req *GetPromptRequest) (*GetPromptResponse, error)
}
```

## Complete Example

Here's a more complete example showing all handler types:

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/gomcpgo/mcp/pkg/handler"
    "github.com/gomcpgo/mcp/pkg/server"
    "github.com/gomcpgo/mcp/pkg/protocol"
)

type MyServer struct {
    handler.ToolHandler
    handler.ResourceHandler
    handler.PromptHandler
}

func (s *MyServer) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
    return &protocol.ListToolsResponse{
        Tools: []protocol.Tool{
            {
                Name:        "my-tool",
                Description: "Description of my tool",
                InputSchema: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "param1": {"type": "string"}
                    }
                }`),
            },
        },
    }, nil
}

func (s *MyServer) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
    return &protocol.CallToolResponse{
        Content: []protocol.ToolContent{
            {
                Type: "text",
                Text: "Tool result",
            },
        },
    }, nil
}

func (s *MyServer) ListResources(ctx context.Context) (*protocol.ListResourcesResponse, error) {
    return &protocol.ListResourcesResponse{
        Resources: []protocol.Resource{
            {
                URI:         "file:///example.txt",
                Name:        "Example File",
                Description: "An example resource",
                MimeType:    "text/plain",
            },
        },
    }, nil
}

func (s *MyServer) ReadResource(ctx context.Context, req *protocol.ReadResourceRequest) (*protocol.ReadResourceResponse, error) {
    return &protocol.ReadResourceResponse{
        Contents: []protocol.ResourceContent{
            {
                URI:      req.URI,
                Text:     "Resource content",
                MimeType: "text/plain",
            },
        },
    }, nil
}

func (s *MyServer) ListPrompts(ctx context.Context) (*protocol.ListPromptsResponse, error) {
    return &protocol.ListPromptsResponse{
        Prompts: []protocol.Prompt{
            {
                Name:        "my-prompt",
                Description: "A prompt template",
                Arguments: []protocol.PromptArgument{
                    {
                        Name:        "arg1",
                        Description: "First argument",
                        Required:    true,
                    },
                },
            },
        },
    }, nil
}

func (s *MyServer) GetPrompt(ctx context.Context, req *protocol.GetPromptRequest) (*protocol.GetPromptResponse, error) {
    return &protocol.GetPromptResponse{
        Messages: []protocol.Message{
            {
                Role: "user",
                Content: protocol.MessageContent{
                    Type: "text",
                    Text: "Prompt content",
                },
            },
        },
    }, nil
}

func main() {
    myServer := &MyServer{}
    registry := handler.NewHandlerRegistry()
    
    registry.RegisterToolHandler(myServer)
    registry.RegisterResourceHandler(myServer)
    registry.RegisterPromptHandler(myServer)

    srv := server.New(server.Options{
        Name:     "complete-server",
        Version:  "1.0.0",
        Registry: registry,
    })

    if err := srv.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## Using with Claude Desktop

To use your MCP server with Claude Desktop, add it to `claude_desktop_config.json`:

```json
{
    "mcpServers": {
        "my-server": {
            "command": "path/to/your/server",
            "args": ["arg1", "arg2"]
        }
    }
}
```

Location of config file:
- MacOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

## Testing

The framework includes utilities for testing your MCP servers. Here's an example:

```go
func TestMyServer(t *testing.T) {
    myServer := &MyServer{}
    registry := handler.NewHandlerRegistry()
    registry.RegisterToolHandler(myServer)

    // Test tool listing
    tools, err := myServer.ListTools(context.Background())
    if err != nil {
        t.Fatalf("ListTools failed: %v", err)
    }

    // Test tool execution
    resp, err := myServer.CallTool(context.Background(), &protocol.CallToolRequest{
        Name: "my-tool",
        Arguments: map[string]interface{}{
            "param1": "test",
        },
    })
    if err != nil {
        t.Fatalf("CallTool failed: %v", err)
    }
}
```

## Best Practices

1. **Error Handling**
   - Use appropriate error types
   - Provide meaningful error messages
   - Handle all error cases

2. **Configuration**
   - Use environment variables for sensitive data
   - Make file paths configurable
   - Validate all configuration

3. **Security**
   - Validate all inputs
   - Limit file system access
   - Use proper permissions

4. **Testing**
   - Write unit tests
   - Test edge cases
   - Use the provided testing utilities

## Learn More

- [MCP Specification](https://modelcontextprotocol.io/)
- [Example Servers](https://github.com/gomcpgo/mcp/examples)
- [Contributing Guide](https://github.com/gomcpgo/mcp/CONTRIBUTING.md)
