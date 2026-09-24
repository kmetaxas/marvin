package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestLogdirDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &logdirDescribeTask{}
	assert.Equal(t, "kafka.logdir.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLogdirDescribeTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &logdirDescribeTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLogdirDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100, OffsetLag: 5, IsFuture: false}},
					}},
				},
			}, nil
		},
	}
	task := &logdirDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 1)
	assert.Equal(t, int32(1), logdirs[0]["broker_id"])
	assert.Equal(t, "", logdirs[0]["error"])

	replicas := logdirs[0]["replicas"].([]map[string]any)
	require.Len(t, replicas, 1)
	assert.Equal(t, "orders", replicas[0]["topic"])
	assert.Equal(t, int32(0), replicas[0]["partition"])
	assert.Equal(t, int64(100), replicas[0]["size_bytes"])
	assert.Equal(t, int64(5), replicas[0]["offset_lag"])
	assert.Equal(t, false, replicas[0]["is_future"])
}

func TestLogdirDescribeTaskTopicFilter(t *testing.T) {
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
	task := &logdirDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topics": []any{"orders"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 1)
	replicas := logdirs[0]["replicas"].([]map[string]any)
	require.Len(t, replicas, 1)
	assert.Equal(t, "orders", replicas[0]["topic"])
}

func TestLogdirDescribeTaskErrorField(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Err: errors.New("disk error")},
				},
			}, nil
		},
	}
	task := &logdirDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 1)
	assert.Equal(t, "disk error", logdirs[0]["error"])
}

func TestLogdirDescribeTaskClientError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &logdirDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
