package async_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gomcpgo/mcp/pkg/async"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Example of using the async package in an MCP server
func Example() {
	// Create executor with custom configuration
	config := async.ExecutorConfig{
		DefaultTimeout:  15 * time.Second,
		MaxLifetime:     10 * time.Minute,
		RetentionPeriod: 5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	executor := async.NewExecutor(config)
	defer executor.Stop()

	// Example: Image generation handler
	handleGenerateImage := func(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
		// Extract parameters
		prompt := args["prompt"].(string)

		// Execute the long-running operation
		result, err := executor.Execute(ctx,
			func(opCtx context.Context) (interface{}, error) {
				// Simulate long-running image generation
				select {
				case <-time.After(20 * time.Second):
					return map[string]string{
						"image_path": "/tmp/generated.png",
						"prompt":     prompt,
					}, nil
				case <-opCtx.Done():
					return nil, opCtx.Err()
				}
			},
			async.ExecuteOptions{
				Type:    "generate_image",
				Timeout: 15 * time.Second, // Return "processing" after 15 seconds
			},
		)

		if err != nil {
			return nil, err
		}

		// Check if operation is still running
		if result.Status == async.StatusRunning {
			// Return processing response
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{
						"status": "processing",
						"operation_id": "%s",
						"message": "%s"
					}`, result.OperationID, result.Message),
				}},
			}, nil
		}

		// Operation completed immediately
		if result.Status == async.StatusFailed {
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{"error": "%s"}`, result.Error),
				}},
			}, nil
		}

		// Format success response
		imageData := result.Result.(map[string]string)
		return &protocol.CallToolResponse{
			Content: []protocol.ToolContent{{
				Type: "text",
				Text: fmt.Sprintf(`{
					"status": "completed",
					"image_path": "%s"
				}`, imageData["image_path"]),
			}},
		}, nil
	}

	// Example: Continue operation handler
	handleContinueOperation := func(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
		opID := args["operation_id"].(string)
		waitTime := 30 * time.Second

		// Check/wait for operation status
		result, err := executor.Continue(ctx, opID, waitTime)
		if err != nil {
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{"error": "%s"}`, err.Error()),
				}},
			}, nil
		}

		// Check status
		switch result.Status {
		case async.StatusRunning:
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{
						"status": "processing",
						"operation_id": "%s",
						"message": "%s"
					}`, result.OperationID, result.Message),
				}},
			}, nil

		case async.StatusFailed:
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{"error": "%s"}`, result.Error),
				}},
			}, nil

		case async.StatusCompleted:
			imageData := result.Result.(map[string]string)
			return &protocol.CallToolResponse{
				Content: []protocol.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf(`{
						"status": "completed",
						"image_path": "%s"
					}`, imageData["image_path"]),
				}},
			}, nil
		}

		return nil, fmt.Errorf("unexpected status: %s", result.Status)
	}

	// Register these handlers with your MCP server
	_ = handleGenerateImage
	_ = handleContinueOperation
	
	fmt.Println("Executor created and handlers defined")
	// Output: Executor created and handlers defined
}