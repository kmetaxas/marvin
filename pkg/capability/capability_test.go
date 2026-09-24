package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid two segments", input: "network.lookup"},
		{name: "valid many segments", input: "network.dns.lookup"},
		{name: "valid underscore", input: "network.icmp.echo_request"},
		{name: "empty", input: "", wantErr: true},
		{name: "single segment", input: "network", wantErr: true},
		{name: "uppercase", input: "network.DNS.lookup", wantErr: true},
		{name: "dash", input: "network.dns-lookup", wantErr: true},
		{name: "numeric", input: "network.v2.lookup", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestCapabilityValidateAndString(t *testing.T) {
	t.Parallel()

	c := Capability{
		Name:        "network.dns.lookup",
		Version:     "v1.0.0",
		Description: "Performs DNS lookups",
		Provider:    "network",
	}

	require.NoError(t, c.Validate())
	assert.Equal(t, "network.dns.lookup", c.String())
	assert.Equal(t, "v1.0.0", c.Version)
	assert.Equal(t, "network", c.Provider)
}

func TestCapabilityToProto(t *testing.T) {
	t.Parallel()

	c := Capability{
		Name:                 "network.dns.lookup",
		Version:              "v1.0.0",
		Description:          "Performs DNS lookups",
		Provider:             "network",
		ParametersJSONSchema: `{"type":"object"}`,
		Config:               map[string]any{"timeout": "5s"},
		ConfigSummary:        "timeout=5s",
		Enabled:              true,
	}

	manifest := c.ToProto()
	assert.Equal(t, "network.dns.lookup", manifest.Name)
	assert.Equal(t, "Performs DNS lookups", manifest.Description)
	assert.True(t, manifest.Enabled)
	assert.Equal(t, `{"type":"object"}`, manifest.ParametersJsonSchema)
	assert.Equal(t, "timeout=5s", manifest.ConfigSummary)
	require.NotNil(t, manifest.Config)
	assert.Equal(t, "5s", manifest.Config.GetFields()["timeout"].GetStringValue())
}

func TestCapabilityToProtoNoConfig(t *testing.T) {
	t.Parallel()

	c := Capability{Name: "network.dns.lookup"}
	manifest := c.ToProto()
	assert.Nil(t, manifest.Config)
	assert.False(t, manifest.Enabled)
}

func TestIsPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		pattern bool
	}{
		{name: "exact name", input: "network.dns.lookup", pattern: false},
		{name: "single wildcard", input: "kubernetes.*", pattern: true},
		{name: "global wildcard", input: "*", pattern: true},
		{name: "suffix wildcard", input: "*.list", pattern: true},
		{name: "no wildcard", input: "os.process.list", pattern: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.pattern, IsPattern(tt.input))
		})
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		capName string
		pattern string
		want    bool
		wantErr bool
	}{
		{name: "exact match", capName: "network.dns.lookup", pattern: "network.dns.lookup", want: true},
		{name: "global wildcard", capName: "network.dns.lookup", pattern: "*", want: true},
		{name: "provider prefix", capName: "kubernetes.pod.list", pattern: "kubernetes.*", want: true},
		{name: "provider prefix no match", capName: "azure.network.nsg.list", pattern: "kubernetes.*", want: false},
		{name: "suffix wildcard", capName: "kubernetes.pod.list", pattern: "*.list", want: true},
		{name: "suffix wildcard no match", capName: "kubernetes.pod.get", pattern: "*.list", want: false},
		{name: "mid wildcard", capName: "network.dns.lookup", pattern: "network.*.lookup", want: true},
		{name: "invalid pattern", capName: "network.dns.lookup", pattern: "[abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			matched, err := Match(tt.capName, tt.pattern)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, matched)
		})
	}
}

func TestValidatePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "global wildcard", input: "*", wantErr: false},
		{name: "provider prefix", input: "kubernetes.*", wantErr: false},
		{name: "suffix wildcard", input: "*.list", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "only wildcards", input: "***", wantErr: true},
		{name: "invalid chars", input: "kubernetes.!", wantErr: true},
		{name: "mid wildcard", input: "network.*.lookup", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePattern(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
