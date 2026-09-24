package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestClusterConfigGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &clusterConfigGetTask{}
	assert.Equal(t, "kafka.cluster.config.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestClusterConfigGetTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &clusterConfigGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestClusterConfigGetTaskSuccess(t *testing.T) {
	t.Parallel()
	v := "168"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name:    "",
					Configs: []kadm.Config{{Key: "log.retention.hours", Value: &v, Source: kmsg.ConfigSourceDefaultConfig}},
				},
			}, nil
		},
	}
	task := &clusterConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	configs := data["configs"].([]ConfigEntry)
	assert.Equal(t, "log.retention.hours", configs[0].Name)
}
