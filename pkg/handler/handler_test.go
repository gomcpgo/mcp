package handler

import (
	"context"
	"testing"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Mock handlers for testing
type mockToolHandler struct {
	listToolsCalled bool
	callToolCalled  bool
}

func (h *mockToolHandler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	h.listToolsCalled = true
	return &protocol.ListToolsResponse{}, nil
}

func (h *mockToolHandler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	h.callToolCalled = true
	return &protocol.CallToolResponse{}, nil
}

type mockResourceHandler struct {
	listResourcesCalled bool
	readResourceCalled  bool
}

func (h *mockResourceHandler) ListResources(ctx context.Context) (*protocol.ListResourcesResponse, error) {
	h.listResourcesCalled = true
	return &protocol.ListResourcesResponse{}, nil
}

func (h *mockResourceHandler) ReadResource(ctx context.Context, req *protocol.ReadResourceRequest) (*protocol.ReadResourceResponse, error) {
	h.readResourceCalled = true
	return &protocol.ReadResourceResponse{}, nil
}

type mockPromptHandler struct {
	listPromptsCalled bool
	getPromptCalled   bool
}

func (h *mockPromptHandler) ListPrompts(ctx context.Context) (*protocol.ListPromptsResponse, error) {
	h.listPromptsCalled = true
	return &protocol.ListPromptsResponse{}, nil
}

func (h *mockPromptHandler) GetPrompt(ctx context.Context, req *protocol.GetPromptRequest) (*protocol.GetPromptResponse, error) {
	h.getPromptCalled = true
	return &protocol.GetPromptResponse{}, nil
}

func TestHandlerRegistry(t *testing.T) {
	// Create registry
	registry := NewHandlerRegistry()

	// Test initial state
	if registry.HasToolHandler() {
		t.Error("expected no tool handler initially")
	}
	if registry.HasResourceHandler() {
		t.Error("expected no resource handler initially")
	}
	if registry.HasPromptHandler() {
		t.Error("expected no prompt handler initially")
	}

	// Create mock handlers
	toolHandler := &mockToolHandler{}
	resourceHandler := &mockResourceHandler{}
	promptHandler := &mockPromptHandler{}

	// Register handlers
	registry.RegisterToolHandler(toolHandler)
	registry.RegisterResourceHandler(resourceHandler)
	registry.RegisterPromptHandler(promptHandler)

	// Test handler registration
	if !registry.HasToolHandler() {
		t.Error("expected tool handler to be registered")
	}
	if !registry.HasResourceHandler() {
		t.Error("expected resource handler to be registered")
	}
	if !registry.HasPromptHandler() {
		t.Error("expected prompt handler to be registered")
	}

	// Test handler retrieval
	if got := registry.GetToolHandler(); got != toolHandler {
		t.Error("GetToolHandler() returned wrong handler")
	}
	if got := registry.GetResourceHandler(); got != resourceHandler {
		t.Error("GetResourceHandler() returned wrong handler")
	}
	if got := registry.GetPromptHandler(); got != promptHandler {
		t.Error("GetPromptHandler() returned wrong handler")
	}

	// Test handler functionality
	ctx := context.Background()

	// Test tool handler
	if _, err := registry.GetToolHandler().ListTools(ctx); err != nil {
		t.Errorf("ListTools() error = %v", err)
	}
	if !toolHandler.listToolsCalled {
		t.Error("ListTools() was not called on tool handler")
	}

	if _, err := registry.GetToolHandler().CallTool(ctx, &protocol.CallToolRequest{}); err != nil {
		t.Errorf("CallTool() error = %v", err)
	}
	if !toolHandler.callToolCalled {
		t.Error("CallTool() was not called on tool handler")
	}

	// Test resource handler
	if _, err := registry.GetResourceHandler().ListResources(ctx); err != nil {
		t.Errorf("ListResources() error = %v", err)
	}
	if !resourceHandler.listResourcesCalled {
		t.Error("ListResources() was not called on resource handler")
	}

	if _, err := registry.GetResourceHandler().ReadResource(ctx, &protocol.ReadResourceRequest{}); err != nil {
		t.Errorf("ReadResource() error = %v", err)
	}
	if !resourceHandler.readResourceCalled {
		t.Error("ReadResource() was not called on resource handler")
	}

	// Test prompt handler
	if _, err := registry.GetPromptHandler().ListPrompts(ctx); err != nil {
		t.Errorf("ListPrompts() error = %v", err)
	}
	if !promptHandler.listPromptsCalled {
		t.Error("ListPrompts() was not called on prompt handler")
	}

	if _, err := registry.GetPromptHandler().GetPrompt(ctx, &protocol.GetPromptRequest{}); err != nil {
		t.Errorf("GetPrompt() error = %v", err)
	}
	if !promptHandler.getPromptCalled {
		t.Error("GetPrompt() was not called on prompt handler")
	}
}
