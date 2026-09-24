package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestLogdirReplicaSizesTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &logdirReplicaSizesTask{}
	assert.Equal(t, "kafka.logdir.replica_sizes.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLogdirReplicaSizesTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &logdirReplicaSizesTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLogdirReplicaSizesTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
						"users":  {0: {Size: 200}},
					}},
				},
			}, nil
		},
	}
	task := &logdirReplicaSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	assert.Equal(t, int64(300), data["total_size_bytes"])
	assert.Equal(t, false, data["truncated"])

	replicas := data["replicas"].([]map[string]any)
	require.Len(t, replicas, 2)
	assert.Equal(t, int32(1), replicas[0]["broker_id"])
	assert.Equal(t, "/var/lib/kafka/data-1", replicas[0]["log_dir"])
	assert.Equal(t, "orders", replicas[0]["topic"])
	assert.Equal(t, int32(0), replicas[0]["partition"])
	assert.Equal(t, int64(100), replicas[0]["size_bytes"])
}

func TestLogdirReplicaSizesTaskLimit(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}, 1: {Size: 200}, 2: {Size: 300}},
					}},
				},
			}, nil
		},
	}
	task := &logdirReplicaSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, int64(300), data["total_size_bytes"])
}

func TestLogdirReplicaSizesTaskBrokerAndTopicFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
						"users":  {0: {Size: 200}},
					}},
				},
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 400}},
					}},
				},
			}, nil
		},
	}
	task := &logdirReplicaSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{
		"broker_ids": []any{2},
		"topics":     []any{"orders"},
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	assert.Equal(t, int64(400), data["total_size_bytes"])
	replicas := data["replicas"].([]map[string]any)
	assert.Equal(t, int32(2), replicas[0]["broker_id"])
}

func TestLogdirReplicaSizesTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &logdirReplicaSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
