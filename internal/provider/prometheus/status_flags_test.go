package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusFlagsTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &statusFlagsTask{}
	assert.Equal(t, "prometheus.status.flags", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStatusFlagsTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &statusFlagsTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStatusFlagsTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		flagsFunc: func(ctx context.Context) (v1.FlagsResult, error) {
			return v1.FlagsResult{
				"storage.tsdb.path":  "/data",
				"web.listen-address": "0.0.0.0:9090",
			}, nil
		},
	}
	task := &statusFlagsTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	flags := data["flags"].(map[string]string)
	assert.Equal(t, "/data", flags["storage.tsdb.path"])
	assert.Equal(t, "0.0.0.0:9090", flags["web.listen-address"])
}

func TestStatusFlagsTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		flagsFunc: func(ctx context.Context) (v1.FlagsResult, error) {
			return nil, errors.New("flags failed")
		},
	}
	task := &statusFlagsTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "flags failed")
}
