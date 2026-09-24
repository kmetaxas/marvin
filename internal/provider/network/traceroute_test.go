package network

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracerouteTaskMetadata(t *testing.T) {
	t.Parallel()

	task := &tracerouteTask{}
	assert.Equal(t, "network.traceroute", task.Name())

	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(task.JSONSchema()), &schema))
	assert.Equal(t, "Traceroute Parameters", schema["title"])
	assert.Contains(t, task.JSONSchema(), `"target"`)
	assert.Contains(t, task.JSONSchema(), `"max_hops"`)
	assert.Contains(t, task.JSONSchema(), `"timeout"`)
	assert.Contains(t, task.JSONSchema(), `"protocol"`)
	assert.Contains(t, task.JSONSchema(), `"udp"`)
	assert.NotContains(t, task.JSONSchema(), `"icmp"`)
}

func TestTracerouteTaskValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      map[string]any
		errorString string
	}{
		{
			name:        "missing target",
			params:      map[string]any{},
			errorString: "missing required parameter: target",
		},
		{
			name:        "invalid max hops type",
			params:      map[string]any{"target": "example.com", "max_hops": "30"},
			errorString: "parameter max_hops must be an integer",
		},
		{
			name:        "max hops too high",
			params:      map[string]any{"target": "example.com", "max_hops": 65},
			errorString: "max_hops must be between 1 and 64",
		},
		{
			name:        "invalid timeout",
			params:      map[string]any{"target": "example.com", "timeout": "soon"},
			errorString: "invalid timeout duration: soon",
		},
		{
			name:        "invalid protocol",
			params:      map[string]any{"target": "example.com", "protocol": "icmp"},
			errorString: "unsupported protocol: icmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := (&tracerouteTask{}).Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Equal(t, tt.errorString, result.Error)
		})
	}
}

func TestParseTracerouteParamsDefaults(t *testing.T) {
	t.Parallel()

	config, err := parseTracerouteParams(map[string]any{"target": " example.com "})
	require.NoError(t, err)
	assert.Equal(t, "example.com", config.target)
	assert.Equal(t, defaultTracerouteMaxHops, config.maxHops)
	assert.Equal(t, defaultTracerouteTimeout, config.timeout)
	assert.Equal(t, "udp", config.protocol)
}

func TestParseTracerouteParamsAcceptsUDPOnly(t *testing.T) {
	t.Parallel()

	config, err := parseTracerouteParams(map[string]any{"target": "example.com", "protocol": " UDP "})
	require.NoError(t, err)
	assert.Equal(t, "udp", config.protocol)

	_, err = parseTracerouteParams(map[string]any{"target": "example.com", "protocol": "tcp"})
	require.EqualError(t, err, "unsupported protocol: tcp")
}

func TestNewProviderIncludesTracerouteCapabilityMetadata(t *testing.T) {
	t.Parallel()

	for _, cap := range NewProvider().Capabilities() {
		if cap.Name != "network.traceroute" {
			continue
		}

		assert.Contains(t, cap.Description, "Traces the network route")
		assert.Contains(t, cap.UseCases, "Identifying routing paths and network topology")
		assert.Contains(t, cap.Aliases, "trace route")
		assert.Contains(t, cap.Tags, "traceroute")
		assert.Contains(t, cap.Tags, "diagnostics")
		return
	}

	t.Fatal("network.traceroute capability not registered")
}
