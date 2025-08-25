# Async Package

The `async` package provides a framework for handling long-running operations in MCP servers that may exceed the standard 60-second timeout.

## Problem

MCP (Model Context Protocol) has a default timeout of 60 seconds for tool calls. Many operations (like AI image generation, data processing, etc.) can take longer than this limit. This package solves this by:

1. Starting operations in background goroutines
2. Returning a "processing" status if the operation doesn't complete within a configurable timeout
3. Providing a continuation mechanism to check/wait for operation completion

## Features

- **Goroutine-based execution**: Operations run in separate goroutines with their own context
- **Configurable timeouts**: Set how long to wait before returning "processing" status
- **Operation tracking**: In-memory registry tracks all running and completed operations
- **Automatic cleanup**: Expired operations are automatically removed after a retention period
- **Cancellation support**: Running operations can be cancelled
- **Thread-safe**: Safe for concurrent use

## Usage

### 1. Create an Executor

```go
import "github.com/gomcpgo/mcp/pkg/async"

config := async.ExecutorConfig{
    DefaultTimeout:  15 * time.Second,  // Wait 15s before returning "processing"
    MaxLifetime:     10 * time.Minute,  // Maximum operation runtime
    RetentionPeriod: 5 * time.Minute,   // Keep completed ops for 5 minutes
    CleanupInterval: 1 * time.Minute,   // Cleanup check frequency
}

executor := async.NewExecutor(config)
defer executor.Stop()
```

### 2. Execute Long-Running Operations

```go
func handleGenerateImage(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
    prompt := args["prompt"].(string)
    
    result, err := executor.Execute(ctx,
        func(opCtx context.Context) (interface{}, error) {
            // Your long-running operation here
            return generateImage(opCtx, prompt)
        },
        async.ExecuteOptions{
            Type:    "generate_image",
            Timeout: 15 * time.Second,
        },
    )
    
    if err != nil {
        return nil, err
    }
    
    // Check if operation is still running
    if result.Status == async.StatusRunning {
        // Return processing response with operation ID
        return formatProcessingResponse(result.OperationID, result.Message)
    }
    
    // Operation completed - return result
    return formatCompletedResponse(result.Result)
}
```

### 3. Continue/Check Operation Status

```go
func handleContinueOperation(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
    opID := args["operation_id"].(string)
    waitTime := 30 * time.Second
    
    result, err := executor.Continue(ctx, opID, waitTime)
    if err != nil {
        return errorResponse(err)
    }
    
    switch result.Status {
    case async.StatusRunning:
        return formatProcessingResponse(result.OperationID, result.Message)
    case async.StatusCompleted:
        return formatCompletedResponse(result.Result)
    case async.StatusFailed:
        return errorResponse(result.Error)
    }
}
```

### 4. Register MCP Tools

Your MCP server should register a `continue_operation` tool:

```go
tools := []protocol.Tool{
    {
        Name: "generate_image",
        Description: "Generate an image from a text prompt",
        InputSchema: generateImageSchema,
    },
    {
        Name: "continue_operation",
        Description: "Check status of a long-running operation",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "operation_id": {
                    "type": "string",
                    "description": "The operation ID to check"
                },
                "wait_time": {
                    "type": "integer",
                    "description": "Seconds to wait for completion",
                    "default": 30
                }
            },
            "required": ["operation_id"]
        }`),
    },
}
```

## How It Works

1. **Execute**: When `Execute()` is called:
   - A unique operation ID is generated
   - The operation starts in a background goroutine with its own context
   - The method waits for the configured timeout
   - If completed within timeout: returns the result immediately
   - If still running: returns a "processing" status with the operation ID

2. **Continue**: When `Continue()` is called:
   - Looks up the operation by ID
   - If completed: returns the result immediately
   - If still running: waits for the specified duration
   - Returns current status after wait

3. **Cleanup**: A background goroutine periodically:
   - Removes operations that have been completed for longer than the retention period
   - Cancels operations that exceed the maximum lifetime

## Configuration

### ExecutorConfig Fields

- `DefaultTimeout`: How long to wait before returning "processing" status (default: 15s)
- `MaxLifetime`: Maximum time an operation can run before being cancelled (default: 10m)
- `RetentionPeriod`: How long to keep completed operations in memory (default: 5m)
- `CleanupInterval`: How often to run cleanup (default: 1m)

## Context Handling

The package uses a hybrid context approach:
- Operations run with a detached context (not affected by MCP timeout)
- Operations have a maximum lifetime to prevent resource leaks
- Operations can be explicitly cancelled via `Cancel(operationID)`

## Thread Safety

All methods are thread-safe and can be called concurrently. Multiple clients can wait for the same operation using `Continue()`.

## Testing

The package includes comprehensive unit tests covering:
- Immediate completion
- Timeout scenarios
- Error handling
- Cancellation
- Multiple simultaneous waiters
- Cleanup behavior

Run tests with:
```bash
go test ./pkg/async -v
```

## Example

See `example_test.go` for a complete example of integrating this package with an MCP server.

## Best Practices

1. **Set appropriate timeouts**: Balance between user experience and server load
2. **Handle all status types**: Always check for Running, Completed, and Failed statuses
3. **Clean shutdown**: Call `executor.Stop()` when shutting down your server
4. **Monitor operations**: Use `ListOperations()` for debugging/monitoring
5. **Result formatting**: The executor returns `interface{}` - cast and format appropriately