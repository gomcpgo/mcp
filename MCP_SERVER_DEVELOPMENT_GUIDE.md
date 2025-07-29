# MCP Server Development Guide

A comprehensive guide for building Model Context Protocol (MCP) servers using the Go MCP framework.

## Table of Contents

1. [Project Structure](#project-structure)
2. [Core Components](#core-components)
3. [Implementation Steps](#implementation-steps)
4. [Best Practices](#best-practices)
5. [Testing Strategies](#testing-strategies)
6. [Common Gotchas](#common-gotchas)
7. [Performance Considerations](#performance-considerations)
8. [Deployment & Distribution](#deployment--distribution)

## Project Structure

### Recommended Directory Layout

```
your-mcp-server/
├── cmd/
│   └── main.go              # MCP server entry point
├── pkg/
│   ├── types/               # API request/response types
│   │   ├── types.go
│   │   └── types_test.go
│   ├── config/              # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── client/              # External API client (if needed)
│   │   ├── client.go
│   │   └── client_test.go
│   └── handlers/            # Business logic
│       ├── handlers.go
│       └── handlers_test.go
├── test/                    # Integration tests
│   └── integration_test.go
├── go.mod
├── go.sum
├── run.sh                   # Build script
└── README.md
```

### Key Principles

- **Package separation**: Separate concerns into logical packages
- **Types first**: Define your data structures before implementation
- **Configuration**: Use environment variables for all configuration
- **Testing**: Unit tests for each package, integration tests for end-to-end

## Core Components

### 1. MCP Server Setup

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    
    "github.com/gomcpgo/mcp/pkg/handler"
    "github.com/gomcpgo/mcp/pkg/protocol"
    "github.com/gomcpgo/mcp/pkg/server"
)

type YourMCPServer struct {
    client *YourClient
    config *config.Config
}

func (s *YourMCPServer) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
    return &protocol.ListToolsResponse{
        Tools: []protocol.Tool{
            {
                Name:        "your_tool_name",
                Description: "Clear description with usage hints for the LLM",
                InputSchema: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "param1": {
                            "type": "string",
                            "description": "Detailed parameter description with examples"
                        }
                    },
                    "required": ["param1"]
                }`),
            },
        },
    }, nil
}

func (s *YourMCPServer) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
    switch req.Name {
    case "your_tool_name":
        result, err := s.handleYourTool(ctx, req.Arguments)
        if err != nil {
            return nil, err
        }
        return &protocol.CallToolResponse{
            Content: []protocol.ToolContent{
                {
                    Type: "text",
                    Text: result,
                },
            },
        }, nil
    default:
        return nil, fmt.Errorf("unknown tool: %s", req.Name)
    }
}
```

### 2. Configuration Management

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    APIKey    string
    Timeout   time.Duration
    MaxRetries int
    // Add your configuration fields
}

func LoadConfig() (*Config, error) {
    cfg := &Config{
        Timeout:    30 * time.Second, // Set defaults
        MaxRetries: 3,
    }

    // Required fields
    cfg.APIKey = os.Getenv("YOUR_API_KEY")
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("YOUR_API_KEY environment variable is required")
    }

    // Optional fields with validation
    if timeout := os.Getenv("YOUR_TIMEOUT"); timeout != "" {
        val, err := time.ParseDuration(timeout)
        if err != nil {
            return nil, fmt.Errorf("invalid YOUR_TIMEOUT: %w", err)
        }
        cfg.Timeout = val
    }

    return cfg, nil
}
```

### 3. Type Definitions

```go
package types

// External API types
type APIRequest struct {
    Query      string            `json:"query"`
    Parameters map[string]string `json:"parameters,omitempty"`
}

type APIResponse struct {
    Result string   `json:"result"`
    Sources []string `json:"sources,omitempty"`
    Error  *APIError `json:"error,omitempty"`
}

type APIError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Code    int    `json:"code,omitempty"`
}

// MCP parameter types (use pointers for optional fields)
type ToolParameters struct {
    Query     string  `json:"query"`
    MaxItems  *int    `json:"max_items,omitempty"`
    Filter    *bool   `json:"filter,omitempty"`
}
```

## Implementation Steps

### Step 1: Define Your Tools

Start by clearly defining what tools your MCP server will provide:

1. **Tool names**: Use descriptive, action-oriented names
2. **Parameters**: Define required vs optional parameters
3. **Response format**: Plan your output structure
4. **Error handling**: Define error scenarios

### Step 2: Create Type Definitions

Define all your types before implementing logic:

```go
// Constants for validation
const (
    MaxQueryLength = 1000
    DefaultTimeout = 30 * time.Second
    MaxRetries     = 5
)

// Validation functions
func (p *ToolParameters) Validate() error {
    if p.Query == "" {
        return fmt.Errorf("query parameter is required")
    }
    if len(p.Query) > MaxQueryLength {
        return fmt.Errorf("query too long (max %d characters)", MaxQueryLength)
    }
    return nil
}
```

### Step 3: Implement Configuration

Always use environment variables for configuration:

- API keys and secrets
- Timeout values
- Feature flags
- Default parameters

### Step 4: Build External API Client (if needed)

```go
package client

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
}

func NewClient(apiKey string, timeout time.Duration) *Client {
    return &Client{
        apiKey: apiKey,
        httpClient: &http.Client{Timeout: timeout},
        baseURL: "https://api.example.com",
    }
}

func (c *Client) MakeRequest(ctx context.Context, req *types.APIRequest) (*types.APIResponse, error) {
    // Implement with proper error handling
    // Include retry logic if needed
    // Handle rate limiting
    // Parse structured errors
}
```

### Step 5: Implement Tool Handlers

```go
func (s *YourMCPServer) handleYourTool(ctx context.Context, params map[string]interface{}) (string, error) {
    // 1. Parse and validate parameters
    query, ok := params["query"].(string)
    if !ok || query == "" {
        return "", fmt.Errorf("query parameter is required. Please provide a search query.")
    }

    // 2. Call external API
    result, err := s.client.MakeRequest(ctx, &types.APIRequest{
        Query: query,
    })
    if err != nil {
        return "", fmt.Errorf("API call failed: %w. Try simplifying your query or check your configuration.", err)
    }

    // 3. Format response
    return formatResponse(result), nil
}
```

## Best Practices

### Tool Descriptions

Write tool descriptions that help LLMs understand when and how to use your tools:

```go
Description: "Search for information about X. Best for: specific use cases. Returns: what it returns. Use 'param1' for Y, 'param2' for Z."
```

### Parameter Descriptions

Include examples and usage hints:

```go
"description": "Search query (e.g., 'latest news about AI', 'Python tutorial'). Be specific for better results."
```

### Error Messages

Provide actionable error messages with hints:

```go
return "", fmt.Errorf("rate limit exceeded: %s. Try reducing request frequency or using a different parameter", err)
```

### Response Formatting

Structure responses for easy parsing:

```markdown
## Main Content
[Your main response]

## Sources
1. https://example.com/source1
2. https://example.com/source2

## Related Information
- Additional context
- Follow-up suggestions
```

### Configuration Validation

Always validate configuration at startup:

```go
func (c *Config) Validate() error {
    if c.APIKey == "" {
        return fmt.Errorf("API key is required")
    }
    if c.Timeout <= 0 {
        return fmt.Errorf("timeout must be positive")
    }
    return nil
}
```

## Testing Strategies

### Unit Tests

Test each package independently:

```go
func TestConfigLoad(t *testing.T) {
    // Test with valid config
    // Test with missing required fields
    // Test with invalid values
}

func TestClientRequest(t *testing.T) {
    // Use httptest.NewServer for mocking
    // Test successful responses
    // Test error scenarios
    // Test retry logic
}
```

### Integration Tests

Test the full MCP server with a CLI test mode:

```go
// Add -test flag to your main.go
func main() {
    testMode := flag.Bool("test", false, "Run integration tests")
    flag.Parse()

    if *testMode {
        runIntegrationTests()
        os.Exit(0)
    }
    
    // Normal MCP server startup
}
```

### Mock External APIs

```go
func createTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(handler))
}
```

## Common Gotchas

### 1. Parameter Type Handling

MCP parameters come as `map[string]interface{}`. Handle type assertions carefully:

```go
// ❌ Wrong - will panic if type assertion fails
query := params["query"].(string)

// ✅ Correct - check for existence and type
query, ok := params["query"].(string)
if !ok || query == "" {
    return "", fmt.Errorf("query parameter is required and must be a string")
}

// For arrays
domains, ok := params["domains"].([]interface{})
if ok {
    stringDomains := make([]string, 0, len(domains))
    for _, d := range domains {
        if str, ok := d.(string); ok {
            stringDomains = append(stringDomains, str)
        }
    }
}
```

### 2. Numeric Parameter Handling

JSON numbers come as float64:

```go
// ❌ Wrong
maxItems := params["max_items"].(int)

// ✅ Correct
if maxItemsFloat, ok := params["max_items"].(float64); ok {
    maxItems := int(maxItemsFloat)
}
```

### 3. Environment Variable Loading

Always check required environment variables at startup:

```go
func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatal(err) // Fail fast if configuration is invalid
    }
    // ... rest of main
}
```

### 4. Error Context

Provide helpful error messages:

```go
// ❌ Generic error
return "", fmt.Errorf("API call failed")

// ✅ Helpful error with context
return "", fmt.Errorf("search API call failed: %w. Check your API key and try again", err)
```

### 5. Response Formatting

Don't just return raw API responses. Format them for LLM consumption:

```go
func formatResponse(apiResp *APIResponse) string {
    var result strings.Builder
    
    result.WriteString(apiResp.Result)
    
    if len(apiResp.Sources) > 0 {
        result.WriteString("\n\n## Sources\n")
        for i, source := range apiResp.Sources {
            result.WriteString(fmt.Sprintf("%d. %s\n", i+1, source))
        }
    }
    
    return result.String()
}
```

### 6. Context Handling

Always respect context cancellation:

```go
func (c *Client) MakeRequest(ctx context.Context, req *APIRequest) (*APIResponse, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, reqBody)
    // ...
}
```

## Performance Considerations

### HTTP Client Reuse

```go
// ✅ Reuse HTTP client
type Client struct {
    httpClient *http.Client // Reused across requests
}

// ❌ Don't create new clients for each request
func makeRequest() {
    client := &http.Client{} // Creates new connection pool each time
}
```

### Connection Pooling

```go
func NewClient(apiKey string, timeout time.Duration) *Client {
    return &Client{
        apiKey: apiKey,
        httpClient: &http.Client{
            Timeout: timeout,
            Transport: &http.Transport{
                MaxIdleConns:       10,
                IdleConnTimeout:    90 * time.Second,
                DisableCompression: false,
            },
        },
    }
}
```

### Memory Management

- Use streaming for large responses
- Don't load entire responses into memory if not needed
- Close response bodies properly

## Deployment & Distribution

### Build Script

Create a `run.sh` script:

```bash
#!/bin/bash

case "$1" in
    "build")
        echo "Building MCP server..."
        go build -o bin/your-server cmd/main.go
        ;;
    "test")
        go test ./pkg/...
        ;;
    "integration-test")
        go run cmd/main.go -test
        ;;
    *)
        echo "Usage: $0 {build|test|integration-test}"
        exit 1
        ;;
esac
```

### Documentation

Always include:
- Clear README with setup instructions
- Environment variable documentation
- Usage examples
- Troubleshooting section

### Version Management

```go
package version

const Version = "1.0.0"

// Use in server initialization
srv := server.New(server.Options{
    Name:     "your-server",
    Version:  version.Version,
    Registry: registry,
})
```

This guide provides a solid foundation for building robust, maintainable MCP servers using the Go MCP framework. Remember to always prioritize clear documentation, thorough testing, and helpful error messages for the best developer and LLM experience.