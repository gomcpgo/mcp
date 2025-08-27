package async

import (
	"context"
	"fmt"
	"log"
	"time"
)

// OperationExecutor manages async operation execution
type OperationExecutor struct {
	registry *OperationRegistry
	config   ExecutorConfig
}

// NewExecutor creates a new operation executor
func NewExecutor(config ExecutorConfig) *OperationExecutor {
	// Apply defaults for zero values
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 15 * time.Second
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 10 * time.Minute
	}
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 5 * time.Minute
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Minute
	}
	
	return &OperationExecutor{
		registry: NewRegistry(config),
		config:   config,
	}
}

// Execute runs an operation with timeout management
func (e *OperationExecutor) Execute(ctx context.Context, operation OperationFunc, opts ExecuteOptions) (*ExecuteResult, error) {
	// Generate operation ID
	opID := generateID()
	log.Printf("[ASYNC] Execute called for operation type: %s, generated ID: %s", opts.Type, opID)
	
	// Use default timeout if not specified
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}
	
	// Create operation context with max lifetime
	opCtx, opCancel := context.WithTimeout(context.Background(), e.config.MaxLifetime)
	
	// Create operation record
	op := &Operation{
		ID:         opID,
		Type:       opts.Type,
		Status:     StatusRunning,
		StartTime:  timeNow().Now(),
		CompleteCh: make(chan struct{}),
		cancelFunc: opCancel,
	}
	
	// Register the operation
	e.registry.Add(op)
	log.Printf("[ASYNC] Operation registered with ID: %s, type: %s", opID, opts.Type)
	
	// Start operation in goroutine
	go func() {
		defer close(op.CompleteCh)
		defer opCancel()
		
		// Run the operation
		result, err := operation(opCtx)
		
		// Update operation status
		op.EndTime = timeNow().Now()
		if err != nil {
			op.Status = StatusFailed
			op.Error = err
		} else {
			op.Status = StatusCompleted
			op.Result = result
		}
	}()
	
	// Wait for completion or timeout
	select {
	case <-op.CompleteCh:
		// Operation completed
		if op.Error != nil {
			return &ExecuteResult{
				Status: StatusFailed,
				Error:  op.Error.Error(),
			}, nil
		}
		return &ExecuteResult{
			Status: StatusCompleted,
			Result: op.Result,
		}, nil
		
	case <-timeNow().After(timeout):
		// Timeout - return processing status
		log.Printf("[ASYNC] Operation %s timed out after %v, returning processing status", opID, timeout)
		return &ExecuteResult{
			Status:        StatusRunning,
			OperationID:   opID,
			OperationType: opts.Type,
			Message:       fmt.Sprintf("Operation in progress. Use continue_operation with operation_id='%s' to check status.", opID),
		}, nil
		
	case <-ctx.Done():
		// MCP context cancelled - but don't cancel the operation
		// The operation continues in the background
		return &ExecuteResult{
			Status:        StatusRunning,
			OperationID:   opID,
			OperationType: opts.Type,
			Message:       "Request cancelled, but operation continues. Use continue_operation to check status.",
		}, nil
	}
}

// Continue checks or waits for operation completion
func (e *OperationExecutor) Continue(ctx context.Context, operationID string, waitTime time.Duration) (*ContinueResult, error) {
	log.Printf("[ASYNC] Continue called for operation ID: %s, waitTime: %v", operationID, waitTime)
	
	// Get the operation
	op, err := e.registry.Get(operationID)
	if err != nil {
		log.Printf("[ASYNC] Operation %s not found in registry: %v", operationID, err)
		return nil, err
	}
	
	log.Printf("[ASYNC] Found operation %s with status: %s, type: %s", operationID, op.Status, op.Type)
	
	// Check current status
	if op.Status != StatusRunning {
		// Operation already completed
		if op.Error != nil {
			return &ContinueResult{
				Status:        StatusFailed,
				OperationID:   operationID,
				OperationType: op.Type,
				Error:         op.Error.Error(),
			}, nil
		}
		return &ContinueResult{
			Status:        StatusCompleted,
			OperationID:   operationID,
			OperationType: op.Type,
			Result:        op.Result,
		}, nil
	}
	
	// Wait for completion or timeout
	select {
	case <-op.CompleteCh:
		// Operation completed
		if op.Error != nil {
			return &ContinueResult{
				Status:        StatusFailed,
				OperationID:   operationID,
				OperationType: op.Type,
				Error:         op.Error.Error(),
			}, nil
		}
		return &ContinueResult{
			Status:        StatusCompleted,
			OperationID:   operationID,
			OperationType: op.Type,
			Result:        op.Result,
		}, nil
		
	case <-timeNow().After(waitTime):
		// Still running
		elapsed := timeNow().Now().Sub(op.StartTime)
		return &ContinueResult{
			Status:        StatusRunning,
			OperationID:   operationID,
			OperationType: op.Type,
			Message:       fmt.Sprintf("Operation still in progress (elapsed: %v). Continue checking.", elapsed.Round(time.Second)),
		}, nil
		
	case <-ctx.Done():
		// Context cancelled
		return nil, ctx.Err()
	}
}

// Cancel cancels a running operation
func (e *OperationExecutor) Cancel(operationID string) error {
	op, err := e.registry.Get(operationID)
	if err != nil {
		return err
	}
	
	if op.Status != StatusRunning {
		return fmt.Errorf("operation %s is not running (status: %s)", operationID, op.Status)
	}
	
	// Cancel the operation
	if op.cancelFunc != nil {
		op.cancelFunc()
		op.Status = StatusFailed
		op.Error = fmt.Errorf("operation cancelled")
		op.EndTime = timeNow().Now()
	}
	
	return nil
}

// Cleanup manually triggers cleanup of expired operations
func (e *OperationExecutor) Cleanup() {
	e.registry.cleanupExpired()
}

// Stop stops the executor and cleans up resources
func (e *OperationExecutor) Stop() {
	e.registry.Stop()
}

// ListOperations returns all operation IDs (mainly for debugging/testing)
func (e *OperationExecutor) ListOperations() []string {
	return e.registry.List()
}