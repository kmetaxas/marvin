package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestLogdirListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &logdirListTask{}
	assert.Equal(t, "kafka.logdir.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLogdirListTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &logdirListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLogdirListTaskSuccess(t *testing.T) {
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
					"/var/lib/kafka/data-2": {Broker: 1, Dir: "/var/lib/kafka/data-2", Topics: kadm.DescribedLogDirTopics{
						"orders": {1: {Size: 300}},
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
	task := &logdirListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 3, data["count"])
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 3)
	assert.Equal(t, int32(1), logdirs[0]["broker_id"])
	assert.Equal(t, int64(300), logdirs[0]["total_size_bytes"])
	assert.Equal(t, 2, logdirs[0]["topic_count"])
}

func TestLogdirListTaskBrokerFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
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
	task := &logdirListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_ids": []any{2}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 1)
	assert.Equal(t, int32(2), logdirs[0]["broker_id"])
}

func TestLogdirListTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &logdirListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
