package async

import (
	"context"
	"time"
)

// OperationFunc is a long-running operation that can be executed asynchronously
type OperationFunc func(ctx context.Context) (interface{}, error)

// OperationStatus represents the current state of an operation
type OperationStatus string

const (
	StatusRunning   OperationStatus = "running"
	StatusCompleted OperationStatus = "completed"
	StatusFailed    OperationStatus = "failed"
)

// Operation represents a tracked async operation
type Operation struct {
	ID         string
	Type       string
	Status     OperationStatus
	Result     interface{}
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	CompleteCh chan struct{}
	cancelFunc context.CancelFunc // For cancelling the operation
}

// ExecuteOptions configures how an operation should be executed
type ExecuteOptions struct {
	Type    string        // Operation type (e.g., "generate_image")
	Timeout time.Duration // How long to wait before returning "processing" status
}

// ExecutorConfig configures the operation executor
type ExecutorConfig struct {
	DefaultTimeout  time.Duration // Default timeout before returning "processing" (default: 15s)
	MaxLifetime     time.Duration // Maximum operation lifetime (default: 10m)
	RetentionPeriod time.Duration // How long to keep completed operations (default: 5m)
	CleanupInterval time.Duration // How often to clean up expired operations (default: 1m)
}

// DefaultConfig returns a default configuration
func DefaultConfig() ExecutorConfig {
	return ExecutorConfig{
		DefaultTimeout:  15 * time.Second,
		MaxLifetime:     10 * time.Minute,
		RetentionPeriod: 5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// ExecuteResult is returned from Execute method
type ExecuteResult struct {
	Status        OperationStatus        `json:"status"`
	OperationID   string                 `json:"operation_id,omitempty"`
	OperationType string                 `json:"operation_type,omitempty"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ContinueResult is returned from Continue method
type ContinueResult struct {
	Status        OperationStatus        `json:"status"`
	OperationID   string                 `json:"operation_id"`
	OperationType string                 `json:"operation_type"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}