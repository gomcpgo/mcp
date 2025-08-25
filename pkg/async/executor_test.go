package async

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockTime implements timeInterface for testing
type mockTime struct {
	current time.Time
	mu      sync.Mutex
}

func (m *mockTime) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *mockTime) Unix() int64 {
	return m.Now().Unix()
}

func (m *mockTime) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- m.Now().Add(d)
	return ch
}

func (m *mockTime) advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = m.current.Add(d)
}

// Test helper to create test executor
func createTestExecutor() *OperationExecutor {
	config := ExecutorConfig{
		DefaultTimeout:  100 * time.Millisecond,
		MaxLifetime:     5 * time.Second,
		RetentionPeriod: 1 * time.Second,
		CleanupInterval: 100 * time.Millisecond,
	}
	return NewExecutor(config)
}

// Test immediate completion (operation completes before timeout)
func TestExecute_ImmediateCompletion(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Fast operation that completes immediately
	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "test_op",
		Timeout: 1 * time.Second,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
	
	if result.Result != "success" {
		t.Errorf("expected result 'success', got %v", result.Result)
	}
	
	if result.OperationID != "" {
		t.Errorf("expected no operation ID for completed operation, got %s", result.OperationID)
	}
}

// Test operation timeout (operation takes longer than timeout)
func TestExecute_Timeout(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Slow operation that exceeds timeout
	operation := func(ctx context.Context) (interface{}, error) {
		select {
		case <-time.After(2 * time.Second):
			return "success", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "slow_op",
		Timeout: 100 * time.Millisecond,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, result.Status)
	}
	
	if result.OperationID == "" {
		t.Errorf("expected operation ID for running operation")
	}
	
	if result.OperationType != "slow_op" {
		t.Errorf("expected operation type 'slow_op', got %s", result.OperationType)
	}
	
	// Verify operation is registered
	ops := executor.ListOperations()
	if len(ops) != 1 {
		t.Errorf("expected 1 registered operation, got %d", len(ops))
	}
}

// Test operation with error
func TestExecute_WithError(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	expectedErr := fmt.Errorf("operation failed")
	
	operation := func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "error_op",
		Timeout: 1 * time.Second,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	
	if result.Error != expectedErr.Error() {
		t.Errorf("expected error '%s', got '%s'", expectedErr.Error(), result.Error)
	}
}

// Test Continue with completed operation
func TestContinue_Completed(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Start a slow operation
	var opID string
	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "completed", nil
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "slow_op",
		Timeout: 50 * time.Millisecond,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Status != StatusRunning {
		t.Fatalf("expected running status, got %s", result.Status)
	}
	
	opID = result.OperationID
	
	// Wait for operation to complete
	time.Sleep(300 * time.Millisecond)
	
	// Continue should return completed result
	continueResult, err := executor.Continue(context.Background(), opID, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if continueResult.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, continueResult.Status)
	}
	
	if continueResult.Result != "completed" {
		t.Errorf("expected result 'completed', got %v", continueResult.Result)
	}
}

// Test Continue with still running operation
func TestContinue_StillRunning(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Start a very slow operation
	operation := func(ctx context.Context) (interface{}, error) {
		select {
		case <-time.After(5 * time.Second):
			return "completed", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "very_slow_op",
		Timeout: 50 * time.Millisecond,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	opID := result.OperationID
	
	// Continue with short wait should return still running
	continueResult, err := executor.Continue(context.Background(), opID, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if continueResult.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, continueResult.Status)
	}
	
	if continueResult.Message == "" {
		t.Errorf("expected progress message for running operation")
	}
}

// Test Continue with non-existent operation
func TestContinue_NotFound(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	_, err := executor.Continue(context.Background(), "nonexistent", 100*time.Millisecond)
	if err == nil {
		t.Errorf("expected error for non-existent operation")
	}
}

// Test Cancel operation
func TestCancel(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Start a slow operation
	operation := func(ctx context.Context) (interface{}, error) {
		select {
		case <-time.After(5 * time.Second):
			return "should not complete", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "cancellable_op",
		Timeout: 50 * time.Millisecond,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	opID := result.OperationID
	
	// Cancel the operation
	err = executor.Cancel(opID)
	if err != nil {
		t.Fatalf("unexpected error cancelling: %v", err)
	}
	
	// Check status after cancellation
	time.Sleep(100 * time.Millisecond)
	continueResult, err := executor.Continue(context.Background(), opID, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if continueResult.Status != StatusFailed {
		t.Errorf("expected failed status after cancel, got %s", continueResult.Status)
	}
	
	if continueResult.Error == "" {
		t.Errorf("expected error message after cancel")
	}
}

// Test multiple simultaneous waiters
func TestContinue_MultipleWaiters(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Start a slow operation
	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(300 * time.Millisecond)
		return "done", nil
	}
	
	result, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "multi_wait_op",
		Timeout: 50 * time.Millisecond,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	opID := result.OperationID
	
	// Start multiple waiters
	var wg sync.WaitGroup
	results := make([]*ContinueResult, 3)
	errors := make([]error, 3)
	
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = executor.Continue(context.Background(), opID, 1*time.Second)
		}(i)
	}
	
	wg.Wait()
	
	// All waiters should get the same result
	for i := 0; i < 3; i++ {
		if errors[i] != nil {
			t.Errorf("waiter %d got error: %v", i, errors[i])
		}
		if results[i].Status != StatusCompleted {
			t.Errorf("waiter %d expected status %s, got %s", i, StatusCompleted, results[i].Status)
		}
		if results[i].Result != "done" {
			t.Errorf("waiter %d expected result 'done', got %v", i, results[i].Result)
		}
	}
}

// Test cleanup of expired operations
func TestCleanup(t *testing.T) {
	config := ExecutorConfig{
		DefaultTimeout:  50 * time.Millisecond,
		MaxLifetime:     1 * time.Second,
		RetentionPeriod: 200 * time.Millisecond,
		CleanupInterval: 100 * time.Millisecond,
	}
	executor := NewExecutor(config)
	defer executor.Stop()
	
	// Create a completed operation
	operation := func(ctx context.Context) (interface{}, error) {
		return "done", nil
	}
	
	_, err := executor.Execute(context.Background(), operation, ExecuteOptions{
		Type:    "cleanup_test",
		Timeout: 1 * time.Second,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Even completed operations are kept for retention period
	// This is by design to avoid race conditions
	ops := executor.ListOperations()
	initialOps := len(ops)
	
	// Create a slow operation that will timeout
	slowOp := func(ctx context.Context) (interface{}, error) {
		select {
		case <-time.After(5 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	result, _ := executor.Execute(context.Background(), slowOp, ExecuteOptions{
		Type:    "slow_cleanup_test",
		Timeout: 50 * time.Millisecond,
	})
	
	// Should have one more operation
	ops = executor.ListOperations()
	if len(ops) != initialOps+1 {
		t.Errorf("expected %d operations, got %d", initialOps+1, len(ops))
	}
	
	// Cancel it to make it completed
	executor.Cancel(result.OperationID)
	
	// Wait for retention period + cleanup interval
	// Both operations should be cleaned up after retention period
	time.Sleep(400 * time.Millisecond)
	
	// Should be cleaned up
	ops = executor.ListOperations()
	if len(ops) != 0 {
		t.Errorf("expected 0 operations after cleanup, got %d", len(ops))
	}
}

// Test context cancellation during Execute
func TestExecute_ContextCancellation(t *testing.T) {
	executor := createTestExecutor()
	defer executor.Stop()
	
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start a slow operation
	operation := func(opCtx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "completed", nil
	}
	
	// Cancel context immediately
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	
	result, err := executor.Execute(ctx, operation, ExecuteOptions{
		Type:    "ctx_cancel_op",
		Timeout: 1 * time.Second,
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should return running status (operation continues despite context cancellation)
	if result.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, result.Status)
	}
	
	if result.OperationID == "" {
		t.Errorf("expected operation ID")
	}
	
	// Wait and verify operation completes successfully
	time.Sleep(300 * time.Millisecond)
	continueResult, _ := executor.Continue(context.Background(), result.OperationID, 50*time.Millisecond)
	
	if continueResult.Status != StatusCompleted {
		t.Errorf("expected operation to complete despite context cancellation, got %s", continueResult.Status)
	}
}