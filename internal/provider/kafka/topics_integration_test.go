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

func TestIntegration_TopicTasks(t *testing.T) {
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

	t.Run("kafka.topic.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.topic.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["topics"])
		assert.NotNil(t, data["count"])
	})

	t.Run("kafka.partition.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["partitions"])
		assert.NotNil(t, data["count"])
	})

	t.Run("kafka.partition.offline.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.offline.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["offline_partitions"])
	})

	t.Run("kafka.partition.under_replicated.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.under_replicated.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["under_replicated_partitions"])
	})

	t.Run("kafka.partition.under_min_isr.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.under_min_isr.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["under_min_isr_partitions"])
	})

	t.Run("kafka.partition.leader_distribution.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.leader_distribution.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["distribution"])
		assert.NotNil(t, data["total_partitions"])
	})

	t.Run("kafka.partition.replica_distribution.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.replica_distribution.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["by_broker"])
	})

	t.Run("kafka.partition.logdirs.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.partition.logdirs.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["logdirs"])
		assert.NotNil(t, data["count"])
	})
}
