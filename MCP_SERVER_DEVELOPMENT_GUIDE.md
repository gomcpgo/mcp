# MCP Server Development Guide

A comprehensive guide for building Model Context Protocol (MCP) servers using the Go MCP framework.

## Table of Contents

1. [Architecture Patterns](#architecture-patterns)
2. [Project Structure](#project-structure)
3. [Core Components](#core-components)
4. [Implementation Steps](#implementation-steps)
5. [Best Practices](#best-practices)
6. [Testing Strategies](#testing-strategies)
7. [Common Gotchas](#common-gotchas)
8. [Performance Considerations](#performance-considerations)
9. [Deployment & Distribution](#deployment--distribution)

## Architecture Patterns

### Clean Architecture Principles

Based on real-world refactoring experience, here's the optimal architecture for MCP servers:

#### 1. Thin Main Entry Point
Keep `cmd/main.go` minimal (~300-400 lines max):
- Command-line flag parsing
- Terminal mode routing for testing
- MCP server initialization
- NO business logic implementation

```go
// cmd/main.go - GOOD: Thin entry point
func main() {
    // Parse flags
    var generateModel string
    flag.StringVar(&generateModel, "g", "", "Generate an image")
    flag.Parse()
    
    // Terminal mode for easy testing
    if generateModel != "" {
        h, err := handler.NewHandler(apiKey, rootFolder, true)
        runGeneration(ctx, h, generateModel, prompt)
        return
    }
    
    // MCP Server mode
    registry := handler.NewHandlerRegistry()
    registry.RegisterToolHandler(h)
    srv := server.New(server.Options{...})
    srv.Run()
}
```

#### 2. Handler Layer Pattern
Separate MCP protocol handling from business logic:

```go
// pkg/handler/handler.go - Handles MCP protocol
type Handler struct {
    generator *generation.Generator
    enhancer  *enhancement.Enhancer
    editor    *editing.Editor
}

func (h *Handler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
    switch req.Name {
    case "generate_image":
        return h.handleGenerateImage(ctx, req.Arguments)
    }
}

// pkg/handler/generate_handlers.go - Parameter extraction
func (h *Handler) handleGenerateImage(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
    // 1. Extract and validate parameters from map[string]interface{}
    params, err := extractGenerateParams(args)
    
    // 2. Call strongly-typed core function
    result, err := h.generator.GenerateImage(ctx, params)
    
    // 3. Build MCP response
    return buildGenerateResponse(result), nil
}
```

#### 3. Core Business Logic with Local Types
Each package owns its types - NO central types dumping ground:

```go
// pkg/generation/types.go - Types local to generation
package generation

type GenerateParams struct {
    Prompt string
    Model  string
    Width  int
    Height int
}

type ImageResult struct {
    URL      string
    Metadata map[string]interface{}
}

// pkg/generation/generate.go - Core logic
func (g *Generator) GenerateImage(ctx context.Context, params GenerateParams) (*ImageResult, error) {
    // Strongly-typed implementation
    // Easy to test
    // No MCP dependencies
}
```

#### 4. Terminal Mode for Testing
Built-in CLI testing makes development faster:

```go
// Terminal operations in main.go
if listModels {
    listAvailableModels()
    return
}

if generateModel != "" {
    // Direct testing without MCP protocol
    req := &protocol.CallToolRequest{
        Name: "generate_image",
        Arguments: map[string]interface{}{
            "prompt": prompt,
            "model":  generateModel,
        },
    }
    resp, err := h.CallTool(ctx, req)
    printResponse(resp)
    return
}
```

### Package Organization Strategy

#### Local Types Pattern
Instead of a central `types` package that becomes a dumping ground:

```
BAD Structure:                    GOOD Structure:
pkg/                             pkg/
├── types/                       ├── generation/
│   ├── generation.go           │   ├── types.go      # Generation-specific types
│   ├── enhancement.go          │   ├── generate.go
│   ├── editing.go              │   └── models.go
│   └── common.go               ├── enhancement/
└── handlers/                    │   ├── types.go      # Enhancement-specific types
                                │   ├── upscale.go
                                │   └── background.go
                                └── handler/
                                    ├── handler.go     # MCP layer
                                    └── tools.go
```

Benefits:
- Types are near their usage
- Packages are self-contained
- Easier to understand and maintain
- Prevents circular dependencies

#### Separation of Concerns

1. **MCP Layer** (`pkg/handler/`): Handles protocol, parameter extraction, response building
2. **Business Logic** (`pkg/generation/`, `pkg/enhancement/`, etc.): Core functionality with strongly-typed functions
3. **External APIs** (`pkg/client/`): API client if needed
4. **Configuration** (`pkg/config/`): Environment variables and settings
5. **Storage** (`pkg/storage/`): File operations, caching

### Refactoring Strategy

When refactoring an existing MCP server (e.g., from 3000+ lines to 400 lines):

1. **Identify Core Functions**: Extract business logic into packages
2. **Create Strongly-Typed Functions**: Replace map[string]interface{} with proper types
3. **Separate Handler Layer**: Move parameter extraction to handler
4. **Add Terminal Mode**: Enable CLI testing without MCP
5. **Remove Duplication**: Consolidate similar code patterns
6. **Delete Unused Code**: Be ruthless about removing "kept for future" code

Example transformation:
```go
// BEFORE: Everything in main.go (3000+ lines)
func CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
    switch req.Name {
    case "generate_image":
        // 200 lines of parameter extraction and logic
        prompt := req.Arguments["prompt"].(string)
        // ... lots of inline logic ...
    }
}

// AFTER: Clean separation (main.go: 400 lines)
// main.go - just routing
// pkg/handler/handler.go - parameter extraction
// pkg/generation/generate.go - core logic with types
```

## Project Structure

### Recommended Directory Layout

```
your-mcp-server/
├── cmd/
│   └── main.go              # Thin entry point (~400 lines max)
├── pkg/
│   ├── config/              # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── handler/             # MCP protocol layer
│   │   ├── handler.go       # Main handler with CallTool
│   │   ├── tools.go         # Tool definitions for ListTools
│   │   ├── [feature]_handlers.go  # Parameter extraction per feature
│   │   └── handler_test.go
│   ├── [feature1]/          # Core business logic (e.g., generation)
│   │   ├── types.go         # Local types for this feature
│   │   ├── [feature1].go    # Core implementation
│   │   └── [feature1]_test.go
│   ├── [feature2]/          # Another feature (e.g., enhancement)
│   │   ├── types.go         # Local types
│   │   ├── [feature2].go
│   │   └── [feature2]_test.go
│   ├── client/              # External API client (if needed)
│   │   ├── types.go         # API-specific types
│   │   ├── client.go
│   │   └── client_test.go
│   └── storage/             # File operations, caching
│       ├── storage.go
│       └── storage_test.go
├── test/                    # Integration tests
│   └── integration_test.go
├── go.mod
├── go.sum
├── run.sh                   # Build and test script
├── .env.example             # Example environment variables
└── README.md
```

### Key Principles

- **Thin main.go**: Keep entry point minimal, only routing and initialization
- **Handler separation**: Separate MCP protocol handling from business logic
- **Local types**: Each package owns its types, no central dumping ground
- **Terminal mode**: Built-in CLI testing for rapid development
- **Configuration**: Use environment variables for all configuration
- **Testing**: Easy-to-test core functions with strongly-typed parameters

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

### Terminal Mode Testing

Build terminal mode directly into your MCP server for rapid development and testing:

```go
// cmd/main.go - Terminal mode flags
func main() {
    var (
        generateModel string
        listModels    bool
        testEnhance   string
        inputImage    string
    )
    
    flag.StringVar(&generateModel, "g", "", "Generate an image using specified model")
    flag.BoolVar(&listModels, "list", false, "List all available models")
    flag.StringVar(&testEnhance, "enhance", "", "Test enhancement: remove-bg, upscale, face")
    flag.StringVar(&inputImage, "input", "", "Input image path")
    flag.Parse()
    
    // Terminal mode operations
    if listModels || generateModel != "" || testEnhance != "" {
        h, err := handler.NewHandler(apiKey, rootFolder, true) // true = debug mode
        if err != nil {
            log.Fatal(err)
        }
        
        ctx := context.Background()
        
        if listModels {
            listAvailableModels()
            return
        }
        
        if generateModel != "" {
            runGeneration(ctx, h, generateModel, prompt)
            return
        }
        
        if testEnhance != "" {
            runEnhancement(ctx, h, testEnhance, inputImage)
            return
        }
    }
    
    // MCP Server mode
    // ...
}
```

### Build Script with Test Commands

Create a comprehensive `run.sh` for easy testing:

```bash
#!/bin/bash

# Source .env file if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

case "$1" in
    "build")
        echo "Building MCP server..."
        go build -o bin/mcp-server ./cmd
        ;;
    
    "test")
        echo "Running unit tests..."
        go test ./pkg/...
        ;;
    
    "generate")
        # Test generation with specific model
        if [ -z "$2" ]; then
            echo "Usage: ./run.sh generate <model> [prompt]"
            exit 1
        fi
        go run ./cmd -g "$2" -p "${3:-default prompt}"
        ;;
    
    "enhance")
        # Test enhancement functions
        if [ -z "$2" ] || [ -z "$3" ]; then
            echo "Usage: ./run.sh enhance <tool> <image_path>"
            exit 1
        fi
        go run ./cmd -enhance "$2" -input "$3"
        ;;
    
    "list-models")
        go run ./cmd -list
        ;;
    
    "run")
        echo "Running MCP server..."
        go run ./cmd
        ;;
    
    *)
        echo "Usage: $0 {build|test|generate|enhance|list-models|run}"
        ;;
esac
```

### Unit Tests

Test core functions with strongly-typed parameters:

```go
// pkg/generation/generate_test.go
func TestGenerateImage(t *testing.T) {
    g := &Generator{
        client: mockClient,
    }
    
    params := GenerateParams{
        Prompt: "test prompt",
        Model:  "test-model",
        Width:  512,
        Height: 512,
    }
    
    result, err := g.GenerateImage(context.Background(), params)
    assert.NoError(t, err)
    assert.NotEmpty(t, result.URL)
}

// pkg/handler/handler_test.go
func TestParameterExtraction(t *testing.T) {
    args := map[string]interface{}{
        "prompt": "test",
        "model": "flux",
        "width": float64(512), // JSON numbers are float64
    }
    
    params, err := extractGenerateParams(args)
    assert.NoError(t, err)
    assert.Equal(t, "test", params.Prompt)
    assert.Equal(t, 512, params.Width)
}
```

### Integration Tests

Test the full MCP server with terminal mode:

```go
// test/integration_test.go
func TestEndToEndGeneration(t *testing.T) {
    // Use terminal mode for integration testing
    cmd := exec.Command("go", "run", "./cmd", "-g", "flux-schnell", "-p", "test image")
    cmd.Env = append(os.Environ(), "API_KEY=test-key")
    
    output, err := cmd.CombinedOutput()
    assert.NoError(t, err)
    assert.Contains(t, string(output), "Generated image")
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

## Key Lessons from Real-World Refactoring

Based on refactoring a 3000+ line MCP server to 400 lines with 87% code reduction:

### Do's
1. **Keep main.go thin** - Just routing and initialization (~400 lines max)
2. **Use terminal mode** - Build CLI testing directly into the server
3. **Create handler layer** - Separate MCP protocol from business logic
4. **Use local types** - Each package owns its types
5. **Write strongly-typed core functions** - Easy to test and understand
6. **Extract parameters early** - Convert map[string]interface{} to structs in handler
7. **Use run.sh for everything** - Consistent commands for build, test, and run
8. **Be ruthless with deletion** - Remove all "kept for future" code

### Don'ts
1. **Don't dump everything in main.go** - It becomes unmaintainable
2. **Don't create central types package** - It becomes a dumping ground
3. **Don't mix protocol handling with logic** - Keep them separate
4. **Don't skip terminal mode** - It makes development 10x faster
5. **Don't keep unused code** - If it's not used now, delete it
6. **Don't over-engineer** - Solve today's problem, not tomorrow's

### Example Transformation Results
- **Before**: 2 files (main.go: 2152 lines, enhancements.go: 988 lines)
- **After**: Clean package structure with main.go at 380 lines
- **Result**: 87% code reduction, much easier to test and maintain

### Quick Start Template

For new MCP servers, start with this structure:
1. Create thin main.go with terminal mode flags
2. Create pkg/handler/ for MCP protocol layer
3. Create pkg/[feature]/ for each core feature with local types
4. Use run.sh for all operations
5. Test with terminal mode first, then with actual MCP client

This guide provides a solid foundation for building robust, maintainable MCP servers using the Go MCP framework. Remember to always prioritize clear documentation, thorough testing, and helpful error messages for the best developer and LLM experience.