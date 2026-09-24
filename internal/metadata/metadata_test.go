package metadata

import (
	"os"
	"runtime"
	"testing"

	"github.com/marvin-agent/marvin/pkg/proto/marvin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataToProtoMap(t *testing.T) {
	t.Parallel()

	m := Metadata{
		Name:        "marvin-us-east-1",
		Cluster:     "prod-us-east-1",
		Region:      "us-east-1",
		Environment: "production",
		Labels: map[string]string{
			"role": "agent",
		},
	}

	assert.Equal(t, map[string]string{
		"name":        "marvin-us-east-1",
		"cluster":     "prod-us-east-1",
		"region":      "us-east-1",
		"environment": "production",
		"role":        "agent",
	}, m.ToProtoMap())
}

func TestMetadataToProto(t *testing.T) {
	t.Parallel()

	m := Metadata{
		Name:             "marvin-us-east-1",
		Cluster:          "prod-us-east-1",
		Region:           "us-east-1",
		Environment:      "production",
		Hostname:         "server1.us-east.example.com",
		LocalIP:          "10.0.0.1",
		Provider:         "aws",
		AvailabilityZone: "us-east-1a",
		VMID:             "i-1234567890abcdef0",
		OS:               "Linux",
		OSVersion:        "Ubuntu 22.04.3 LTS",
		Arch:             "x86_64",
	}

	got := m.ToProto()
	assert.Equal(t, &marvin.HostMetadata{
		Hostname:         "server1.us-east.example.com",
		LocalIp:          "10.0.0.1",
		Provider:         "aws",
		Region:           "us-east-1",
		AvailabilityZone: "us-east-1a",
		VmId:             "i-1234567890abcdef0",
		Os:               "Linux",
		OsVersion:        "Ubuntu 22.04.3 LTS",
		Arch:             "x86_64",
	}, got)
}

func TestMetadataToLabels(t *testing.T) {
	t.Parallel()

	m := Metadata{
		Labels: map[string]string{
			"env":  "production",
			"team": "platform",
		},
	}

	assert.ElementsMatch(t, []string{"env=production", "team=platform"}, m.ToLabels())
}

func TestAutodetectDefaults(t *testing.T) {
	t.Parallel()

	hostname, err := os.Hostname()
	require.NoError(t, err)

	m := Metadata{}
	require.NoError(t, m.AutodetectDefaults())

	assert.NotEmpty(t, m.Hostname)
	assert.Equal(t, hostname, m.Hostname)

	assert.NotEmpty(t, m.LocalIP)

	assert.NotEmpty(t, m.OS)
	assert.Equal(t, runtime.GOOS, m.OS)

	assert.NotEmpty(t, m.OSVersion)
	assert.Equal(t, runtime.GOOS, m.OSVersion)

	assert.NotEmpty(t, m.Arch)
	goarch := runtime.GOARCH
	var expectedArch string
	switch goarch {
	case "amd64":
		expectedArch = "x86_64"
	case "arm64":
		expectedArch = "aarch64"
	default:
		expectedArch = goarch
	}
	assert.Equal(t, expectedArch, m.Arch)
}

func TestAutodetectDefaultsPreservesExplicitValues(t *testing.T) {
	t.Parallel()

	m := Metadata{
		Hostname:  "explicit-host",
		LocalIP:   "192.168.1.1",
		OS:        "ExplicitOS",
		OSVersion: "1.0.0",
		Arch:      "s390x",
	}
	require.NoError(t, m.AutodetectDefaults())

	assert.Equal(t, "explicit-host", m.Hostname)
	assert.Equal(t, "192.168.1.1", m.LocalIP)
	assert.Equal(t, "ExplicitOS", m.OS)
	assert.Equal(t, "1.0.0", m.OSVersion)
	assert.Equal(t, "s390x", m.Arch)
}
