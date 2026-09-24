// Package common provides shared helper functions used by multiple providers.
package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
)

// RequireString extracts a required string parameter from the params map.
func RequireString(params map[string]any, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	return strings.TrimSpace(s), nil
}

// OptionalString extracts an optional string parameter from the params map.
func OptionalString(params map[string]any, key, defaultValue string) (string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultValue, nil
	}
	return s, nil
}

// OptionalBool extracts an optional boolean parameter from the params map.
func OptionalBool(params map[string]any, key string, defaultValue bool) (bool, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("parameter %s must be a boolean", key)
	}
	return b, nil
}

// OptionalInt extracts an optional integer parameter from the params map.
func OptionalInt(params map[string]any, key string, defaultValue int) (int, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	switch value := v.(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("parameter %s must be an integer", key)
		}
		return int(value), nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

// NormalizeLimit extracts and normalizes a limit parameter from the params map.
// Default is 100, maximum is 1000.
func NormalizeLimit(params map[string]any) (int, error) {
	limit, err := OptionalInt(params, "limit", 100)
	if err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("parameter limit must be greater than 0")
	}
	if limit > 1000 {
		limit = 1000
	}
	return limit, nil
}

// TaskFailure creates a task.Result indicating failure with the given error.
func TaskFailure(err error) (task.Result, error) {
	return task.Result{Success: false, Error: err.Error(), Timestamp: time.Now().UTC()}, nil
}

// SuccessResult creates a task.Result indicating success with the given data.
func SuccessResult(data any) task.Result {
	return task.Result{Success: true, Data: data, Timestamp: time.Now().UTC()}
}
