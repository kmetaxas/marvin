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

func TestIntegration_ClusterTasks(t *testing.T) {
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

	t.Run("kafka.cluster.describe", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.cluster.describe")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotEmpty(t, data["cluster_id"], "cluster_id should be present")
		assert.NotEmpty(t, data["brokers"], "brokers should be present")
		assert.GreaterOrEqual(t, data["broker_count"], 1, "should have at least one broker")
	})

	t.Run("kafka.broker.list", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.broker.list")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		brokers := data["brokers"].([]BrokerInfo)
		assert.NotEmpty(t, brokers, "should have brokers")
		assert.GreaterOrEqual(t, data["count"], 1, "count should be >= 1")
	})

	t.Run("kafka.broker.describe", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.broker.describe")
		require.True(t, ok, "task should be registered")

		// First get broker list to find a broker_id
		listTk, _ := p.GetTask("kafka.broker.list")
		listRes, _ := listTk.Execute(ctx, map[string]any{})
		listData := listRes.Data.(map[string]any)
		brokers := listData["brokers"].([]BrokerInfo)
		require.NotEmpty(t, brokers, "need at least one broker")
		brokerID := brokers[0].ID

		res, err := tk.Execute(ctx, map[string]any{"broker_id": brokerID})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		broker := data["broker"].(BrokerInfo)
		assert.Equal(t, brokerID, broker.ID)
		assert.NotEmpty(t, broker.Host)
	})

	t.Run("kafka.broker.config.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.broker.config.get")
		require.True(t, ok, "task should be registered")

		// Get broker ID from broker list
		listTk, _ := p.GetTask("kafka.broker.list")
		listRes, _ := listTk.Execute(ctx, map[string]any{})
		listData := listRes.Data.(map[string]any)
		brokers := listData["brokers"].([]BrokerInfo)
		require.NotEmpty(t, brokers)
		brokerID := brokers[0].ID

		res, err := tk.Execute(ctx, map[string]any{"broker_id": brokerID})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		configs := data["configs"].([]ConfigEntry)
		assert.NotEmpty(t, configs, "should have config entries")
	})

	t.Run("kafka.cluster.config.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.cluster.config.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		configs := data["configs"].([]ConfigEntry)
		assert.NotEmpty(t, configs, "should have cluster config entries")
	})

	t.Run("kafka.cluster.api_versions.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.cluster.api_versions.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		apiVersions := data["api_versions"].([]map[string]any)
		assert.NotEmpty(t, apiVersions, "should have api versions")
	})

	t.Run("kafka.cluster.metadata.get", func(t *testing.T) {
		tk, ok := p.GetTask("kafka.cluster.metadata.get")
		require.True(t, ok, "task should be registered")

		res, err := tk.Execute(ctx, map[string]any{})
		require.NoError(t, err)
		require.True(t, res.Success, res.Error)

		data := res.Data.(map[string]any)
		assert.NotEmpty(t, data["cluster_id"])
		assert.NotEmpty(t, data["brokers"])
	})
}
