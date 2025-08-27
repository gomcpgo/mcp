package async

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// OperationRegistry manages tracked operations
type OperationRegistry struct {
	operations map[string]*Operation
	mu         sync.RWMutex
	config     ExecutorConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewRegistry creates a new operation registry
func NewRegistry(config ExecutorConfig) *OperationRegistry {
	r := &OperationRegistry{
		operations: make(map[string]*Operation),
		config:     config,
		stopCh:     make(chan struct{}),
	}
	
	// Start cleanup goroutine
	r.startCleanup()
	
	return r
}

// Add registers a new operation
func (r *OperationRegistry) Add(op *Operation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.operations[op.ID] = op
	log.Printf("[REGISTRY] Added operation %s (type: %s, status: %s)", op.ID, op.Type, op.Status)
}

// Get retrieves an operation by ID
func (r *OperationRegistry) Get(id string) (*Operation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	op, exists := r.operations[id]
	if !exists {
		log.Printf("[REGISTRY] Operation %s not found. Current operations: %v", id, r.getOperationIDs())
		return nil, fmt.Errorf("operation not found: %s", id)
	}
	
	log.Printf("[REGISTRY] Retrieved operation %s (type: %s, status: %s)", id, op.Type, op.Status)
	return op, nil
}

// getOperationIDs returns all operation IDs (for debugging)
func (r *OperationRegistry) getOperationIDs() []string {
	ids := make([]string, 0, len(r.operations))
	for id := range r.operations {
		ids = append(ids, id)
	}
	return ids
}

// Remove deletes an operation by ID
func (r *OperationRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if op, exists := r.operations[id]; exists {
		// Cancel the operation context if it exists
		if op.cancelFunc != nil {
			op.cancelFunc()
		}
		delete(r.operations, id)
	}
}

// List returns all operation IDs
func (r *OperationRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	ids := make([]string, 0, len(r.operations))
	for id := range r.operations {
		ids = append(ids, id)
	}
	return ids
}

// startCleanup starts the background cleanup goroutine
func (r *OperationRegistry) startCleanup() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		
		ticker := time.NewTicker(r.config.CleanupInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				r.cleanupExpired()
			case <-r.stopCh:
				return
			}
		}
	}()
}

// cleanupExpired removes expired operations
func (r *OperationRegistry) cleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Now()
	
	for id, op := range r.operations {
		// Remove operations that have been completed/failed for longer than retention period
		if op.Status != StatusRunning {
			if now.Sub(op.EndTime) > r.config.RetentionPeriod {
				delete(r.operations, id)
			}
		} else {
			// Remove operations that have been running longer than max lifetime
			if now.Sub(op.StartTime) > r.config.MaxLifetime {
				// Cancel the operation
				if op.cancelFunc != nil {
					op.cancelFunc()
				}
				op.Status = StatusFailed
				op.Error = fmt.Errorf("operation exceeded maximum lifetime")
				op.EndTime = now
				// Don't delete immediately, let retention period handle it
			}
		}
	}
}

// Stop stops the registry and cleanup goroutine
func (r *OperationRegistry) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	
	// Cancel all running operations
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for _, op := range r.operations {
		if op.cancelFunc != nil {
			op.cancelFunc()
		}
	}
}