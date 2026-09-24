package prometheus

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusHealthyTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusHealthyTask{}
	assert.Equal(t, "prometheus.status.healthy", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusHealthyTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusHealthyTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusHealthyTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		healthyFunc: func(ctx context.Context) (bool, error) {
			return true, nil
		},
	}
	task := &statusHealthyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["healthy"])
}

func TestStatusHealthyTaskNotHealthy(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		healthyFunc: func(ctx context.Context) (bool, error) {
			return false, nil
		},
	}
	task := &statusHealthyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["healthy"])
}

func TestStatusHealthyTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		healthyFunc: func(ctx context.Context) (bool, error) {
			return false, errors.New("healthy failed")
		},
	}
	task := &statusHealthyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "healthy failed")
}
