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

func TestIntegration_ConsumerGroupTasks(t *testing.T) {
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

	t.Run("kafka.consumer_group.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.consumer_group.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotNil(t, data["groups"])
		assert.NotNil(t, data["count"])
	})

	t.Run("kafka.consumer_group.coordinator.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.consumer_group.coordinator.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{"group_id": "nonexistent-group"})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		coordinator := data["coordinator"].(BrokerInfo)
		assert.GreaterOrEqual(t, coordinator.ID, int32(0))
	})

	t.Run("kafka.consumer_group.offset_commit.verify", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.consumer_group.offset_commit.verify")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{"group_id": "nonexistent-group"})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.True(t, data["coordinator_reachable"].(bool))
		assert.Equal(t, "passed", data["protocol_check"])
	})
}
