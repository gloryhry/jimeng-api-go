package task

import (
	"fmt"

	"github.com/gloryhry/jimeng-api-go/internal/pkg/logger"
)

// TaskManager defines the interface for managing tasks
type TaskManager interface {
	ExecuteTask(submitFunc func() (string, error), pollFunc func(string) (interface{}, error)) (interface{}, error)
}

// DefaultTaskManager is the default implementation of TaskManager
type DefaultTaskManager struct{}

// NewTaskManager creates a new instance of DefaultTaskManager
func NewTaskManager() *DefaultTaskManager {
	return &DefaultTaskManager{}
}

// ExecuteTask executes a task by first submitting it and then polling for the result
func (m *DefaultTaskManager) ExecuteTask(submitFunc func() (string, error), pollFunc func(string) (interface{}, error)) (interface{}, error) {
	// 1. Submit the task
	taskID, err := submitFunc()
	if err != nil {
		return nil, err
	}

	if taskID == "" {
		return nil, fmt.Errorf("task submission returned empty ID")
	}

	logger.Info(fmt.Sprintf("Task submitted successfully, ID: %s. Starting polling...", taskID))

	// 2. Poll for the result
	// Note: In a fully async system, we would return the taskID here and let the client poll.
	// But since we are maintaining a synchronous API, we block and poll here.
	result, err := pollFunc(taskID)
	if err != nil {
		return nil, err
	}

	return result, nil
}
