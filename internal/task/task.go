package task

import (
	"context"
	"time"
)

// Result is the structured output of any task execution.
type Result struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Task is a unit of work that a Provider can execute.
type Task interface {
	// Name returns the capability name this task handles, e.g. "network.dns.lookup".
	Name() string

	// Execute runs the task with the given parameters and returns a structured result.
	Execute(ctx context.Context, params map[string]any) (Result, error)

	// JSONSchema returns a JSON Schema (draft 2020-12) string describing the
	// expected parameters for Execute. The control plane validates user input
	// against this schema before dispatching the capability.
	JSONSchema() string
}
