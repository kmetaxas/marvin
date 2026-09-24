package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVersionGetTask(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	provider.CurrentClient().clientset.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{
		Major:        "1",
		Minor:        "30",
		GitVersion:   "v1.30.0",
		GitCommit:    "abc123",
		GitTreeState: "clean",
		BuildDate:    "2024-01-15T00:00:00Z",
		GoVersion:    "go1.22.0",
		Compiler:     "gc",
		Platform:     "linux/amd64",
	}

	task := &versionGetTask{provider: provider}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	v := data["version"].(versionInfo)
	assert.Equal(t, "1", v.Major)
	assert.Equal(t, "30", v.Minor)
	assert.Equal(t, "v1.30.0", v.GitVersion)
	assert.Equal(t, "linux/amd64", v.Platform)
}

func TestVersionGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &versionGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
