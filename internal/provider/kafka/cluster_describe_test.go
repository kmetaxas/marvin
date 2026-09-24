package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestClusterDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &clusterDescribeTask{}
	assert.Equal(t, "kafka.cluster.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestClusterDescribeTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &clusterDescribeTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestClusterDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	rack := "us-east-1a"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Cluster:    "cluster-123",
				Controller: 1,
				Brokers: kadm.BrokerDetails{
					{NodeID: 1, Host: "kafka-1", Port: 9092, Rack: &rack},
					{NodeID: 2, Host: "kafka-2", Port: 9092},
				},
			}, nil
		},
	}
	task := &clusterDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "cluster-123", data["cluster_id"])
	assert.Equal(t, int32(1), data["controller_id"])
	assert.Equal(t, 2, data["broker_count"])

	brokers := data["brokers"].([]BrokerInfo)
	require.Len(t, brokers, 2)
	assert.Equal(t, int32(1), brokers[0].ID)
	assert.True(t, brokers[0].IsController)
	assert.Equal(t, "us-east-1a", brokers[0].Rack)
	assert.False(t, brokers[1].IsController)
}

func TestClusterDescribeTaskMetadataError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{}, errors.New("metadata failed")
		},
	}
	task := &clusterDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "metadata failed")
}
