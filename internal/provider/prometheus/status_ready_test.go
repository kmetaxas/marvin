package prometheus

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusReadyTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusReadyTask{}
	assert.Equal(t, "prometheus.status.ready", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusReadyTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusReadyTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusReadyTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		readyFunc: func(ctx context.Context) (bool, error) {
			return true, nil
		},
	}
	task := &statusReadyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["ready"])
}

func TestStatusReadyTaskNotReady(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		readyFunc: func(ctx context.Context) (bool, error) {
			return false, nil
		},
	}
	task := &statusReadyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["ready"])
}

func TestStatusReadyTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		readyFunc: func(ctx context.Context) (bool, error) {
			return false, errors.New("ready failed")
		},
	}
	task := &statusReadyTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "ready failed")
}
