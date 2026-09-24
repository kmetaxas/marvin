package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithArgs_Network(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runWithArgs([]string{"network"}, &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "=== Provider: network ===")
	assert.Contains(t, out, "--- Configuration JSON Schema ---")
	assert.Contains(t, out, "--- Capabilities ---")
	assert.Contains(t, out, "network.dns.lookup")
	assert.Contains(t, out, "network.icmp.echo_request")
	assert.Contains(t, out, "network.socket.connect")
	assert.Contains(t, out, "network.tls.handshake")
}

func TestRunWithArgs_Os(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runWithArgs([]string{"os"}, &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "=== Provider: os ===")
	assert.Contains(t, out, "os.process.list")
	assert.Contains(t, out, "os.file.read")
}

func TestRunWithArgs_InvalidProvider(t *testing.T) {
	t.Parallel()

	err := runWithArgs([]string{"nonexistent"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
	assert.Contains(t, err.Error(), "linux")
	assert.Contains(t, err.Error(), "network")
}

func TestRunWithArgs_MissingProviderArg(t *testing.T) {
	t.Parallel()

	err := runWithArgs([]string{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
	assert.Contains(t, err.Error(), "available providers")
}

func TestRunWithArgs_UnsupportedOutputFormat(t *testing.T) {
	t.Parallel()

	err := runWithArgs([]string{"-o", "json", "linux"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestAllProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		provider         string
		expectConfig     bool
		expectCapability string
	}{
		{"linux", "linux", true, "linux.system.info"},
		{"network", "network", false, "network.dns.lookup"},
		{"azure", "azure", true, "azure.network.nsg.list"},
		{"kubernetes", "kubernetes", true, "kubernetes.pod.list"},
		{"prometheus", "prometheus", true, "prometheus.query.instant"},
		{"kafka", "kafka", true, "kafka.cluster.describe"},
		{"log", "log", true, "log.graylog.search"},
		{"os", "os", false, "os.process.list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := runWithArgs([]string{tt.provider}, &buf)
			require.NoError(t, err)
			out := buf.String()

			assert.Contains(t, out, fmt.Sprintf("=== Provider: %s ===", tt.provider))
			assert.Contains(t, out, tt.expectCapability)

			if tt.expectConfig {
				configStart := strings.Index(out, "--- Configuration JSON Schema ---")
				require.GreaterOrEqual(t, configStart, 0)
				capsStart := strings.Index(out, "--- Capabilities ---")
				require.GreaterOrEqual(t, capsStart, 0)
				schemaJSON := strings.TrimSpace(out[configStart+len("--- Configuration JSON Schema ---") : capsStart])

				require.NotEmpty(t, schemaJSON)
				var schema map[string]any
				require.NoError(t, json.Unmarshal([]byte(schemaJSON), &schema))
				// jsonschema.Reflect often produces $ref/$defs rather than a top-level type.
				assert.True(t, schema["type"] == "object" || schema["$defs"] != nil || schema["$ref"] != nil, "schema should have object type, $defs, or $ref")
			} else {
				assert.Contains(t, out, "--- Configuration JSON Schema ---")
				assert.Contains(t, out, "{}")
			}

			capsStart := strings.Index(out, "--- Capabilities ---")
			require.GreaterOrEqual(t, capsStart, 0)
			capsSection := out[capsStart:]
			assert.Contains(t, capsSection, "Total:")
			assert.Contains(t, capsSection, "Capability names (Resource Type list):")
			assert.Contains(t, capsSection, fmt.Sprintf("\"%s\"", tt.expectCapability))
		})
	}
}
