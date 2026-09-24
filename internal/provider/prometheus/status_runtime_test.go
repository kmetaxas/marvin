package prometheus

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusRuntimeTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusRuntimeTask{}
	assert.Equal(t, "prometheus.status.runtime", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusRuntimeTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusRuntimeTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusRuntimeTaskSuccess(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	client := &fakePrometheusClient{
		runtimeinfoFunc: func(ctx context.Context) (v1.RuntimeinfoResult, error) {
			return v1.RuntimeinfoResult{
				StartTime:        startTime,
				StorageRetention: "15d",
			}, nil
		},
		buildinfoFunc: func(ctx context.Context) (v1.BuildinfoResult, error) {
			return v1.BuildinfoResult{
				Version:   "2.50.0",
				Revision:  "abc123",
				Branch:    "HEAD",
				BuildDate: "20240101-10:00:00",
				GoVersion: "go1.21.5",
			}, nil
		},
	}
	task := &statusRuntimeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "2.50.0", data["version"])
	assert.Equal(t, "abc123", data["build_revision"])
	assert.Equal(t, "HEAD", data["build_branch"])
	assert.Equal(t, "20240101-10:00:00", data["build_date"])
	assert.Equal(t, "go1.21.5", data["go_version"])
	assert.Equal(t, "2024-01-15T10:30:00Z", data["start_time"])
	assert.Equal(t, "15d", data["storage_retention"])
}

func TestStatusRuntimeTaskRuntimeError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		runtimeinfoFunc: func(ctx context.Context) (v1.RuntimeinfoResult, error) {
			return v1.RuntimeinfoResult{}, errors.New("runtime failed")
		},
	}
	task := &statusRuntimeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "runtime failed")
}

func TestStatusRuntimeTaskBuildinfoError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		runtimeinfoFunc: func(ctx context.Context) (v1.RuntimeinfoResult, error) {
			return v1.RuntimeinfoResult{}, nil
		},
		buildinfoFunc: func(ctx context.Context) (v1.BuildinfoResult, error) {
			return v1.BuildinfoResult{}, errors.New("buildinfo failed")
		},
	}
	task := &statusRuntimeTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "buildinfo failed")
}
