package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/kafka/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_StorageTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	bootstrapServers, _ := testutil.StartKafkaPlaintext(t)

	cfg := config.KafkaConfig{
		BootstrapServers: bootstrapServers,
	}
	p := NewProvider(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("kafka.logdir.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.logdir.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.GreaterOrEqual(t, data["count"], 1, "should have at least one log dir")
		logdirs := data["logdirs"].([]map[string]any)
		assert.NotEmpty(t, logdirs)
		assert.NotEmpty(t, logdirs[0]["log_dir"])
	})

	t.Run("kafka.logdir.describe", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.logdir.describe")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.GreaterOrEqual(t, data["count"], 1, "should have at least one log dir")
		logdirs := data["logdirs"].([]map[string]any)
		assert.NotEmpty(t, logdirs)
		assert.Contains(t, logdirs[0], "replicas")
	})

	t.Run("kafka.logdir.replica_sizes.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.logdir.replica_sizes.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.Contains(t, data, "replicas")
		assert.Contains(t, data, "total_size_bytes")
	})

	t.Run("kafka.logdir.errors.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.logdir.errors.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.Contains(t, data, "errors")
	})

	t.Run("kafka.storage.topic_sizes.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.storage.topic_sizes.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.Contains(t, data, "topic_sizes")
		assert.Contains(t, data, "count")
	})
}
