package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// fakeKafkaClient implements KafkaClient for cluster task tests.
type fakeKafkaClient struct {
	metadataFunc              func(ctx context.Context, topics ...string) (kadm.Metadata, error)
	apiVersionsFunc           func(ctx context.Context) (kadm.BrokersApiVersions, error)
	listBrokersFunc           func(ctx context.Context) (kadm.BrokerDetails, error)
	describeConfigsFunc       func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error)
	requestFunc               func(ctx context.Context, req kmsg.Request) (kmsg.Response, error)
	listTopicsFunc            func(ctx context.Context, topics ...string) (kadm.TopicDetails, error)
	describeTopicConfigsFunc  func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error)
	describeAllLogDirsFunc    func(ctx context.Context) (kadm.DescribedAllLogDirs, error)
	listGroupsFunc            func(ctx context.Context, states ...string) (kadm.ListedGroups, error)
	describeGroupsFunc        func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error)
	fetchOffsetsFunc          func(ctx context.Context, group string) (kadm.OffsetResponses, error)
	lagFunc                   func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error)
	findGroupCoordinatorsFunc func(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses
	listEndOffsetsFunc        func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error)
	listStartOffsetsFunc      func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error)
	requestShardedFunc        func(ctx context.Context, req kmsg.Request) []kgo.ResponseShard
	guardrails                GuardrailPolicy
}

func (f *fakeKafkaClient) Metadata(ctx context.Context, topics ...string) (kadm.Metadata, error) {
	if f.metadataFunc == nil {
		return kadm.Metadata{}, nil
	}
	return f.metadataFunc(ctx, topics...)
}

func (f *fakeKafkaClient) ApiVersions(ctx context.Context) (kadm.BrokersApiVersions, error) {
	if f.apiVersionsFunc == nil {
		return nil, nil
	}
	return f.apiVersionsFunc(ctx)
}

func (f *fakeKafkaClient) ListBrokers(ctx context.Context) (kadm.BrokerDetails, error) {
	if f.listBrokersFunc == nil {
		return nil, nil
	}
	return f.listBrokersFunc(ctx)
}

func (f *fakeKafkaClient) DescribeBrokerConfigs(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
	if f.describeConfigsFunc == nil {
		return nil, nil
	}
	return f.describeConfigsFunc(ctx, brokers...)
}

func (f *fakeKafkaClient) Request(ctx context.Context, req kmsg.Request) (kmsg.Response, error) {
	if f.requestFunc == nil {
		return nil, nil
	}
	return f.requestFunc(ctx, req)
}

func (f *fakeKafkaClient) Guardrails() GuardrailPolicy { return f.guardrails }

func (f *fakeKafkaClient) ListTopics(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
	if f.listTopicsFunc == nil {
		return nil, nil
	}
	return f.listTopicsFunc(ctx, topics...)
}

func (f *fakeKafkaClient) DescribeTopicConfigs(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
	if f.describeTopicConfigsFunc == nil {
		return nil, nil
	}
	return f.describeTopicConfigsFunc(ctx, topics...)
}

func (f *fakeKafkaClient) DescribeAllLogDirs(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
	if f.describeAllLogDirsFunc == nil {
		return nil, nil
	}
	return f.describeAllLogDirsFunc(ctx)
}

func (f *fakeKafkaClient) ListGroups(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
	if f.listGroupsFunc == nil {
		return nil, nil
	}
	return f.listGroupsFunc(ctx, states...)
}

func (f *fakeKafkaClient) DescribeGroups(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
	if f.describeGroupsFunc == nil {
		return nil, nil
	}
	return f.describeGroupsFunc(ctx, groups...)
}

func (f *fakeKafkaClient) FetchOffsets(ctx context.Context, group string) (kadm.OffsetResponses, error) {
	if f.fetchOffsetsFunc == nil {
		return nil, nil
	}
	return f.fetchOffsetsFunc(ctx, group)
}

func (f *fakeKafkaClient) Lag(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
	if f.lagFunc == nil {
		return nil, nil
	}
	return f.lagFunc(ctx, groups...)
}

func (f *fakeKafkaClient) FindGroupCoordinators(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
	if f.findGroupCoordinatorsFunc == nil {
		return nil
	}
	return f.findGroupCoordinatorsFunc(ctx, groups...)
}

func (f *fakeKafkaClient) ListEndOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
	if f.listEndOffsetsFunc == nil {
		return nil, nil
	}
	return f.listEndOffsetsFunc(ctx, topics...)
}

func (f *fakeKafkaClient) ListStartOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
	if f.listStartOffsetsFunc == nil {
		return nil, nil
	}
	return f.listStartOffsetsFunc(ctx, topics...)
}

func (f *fakeKafkaClient) RequestSharded(ctx context.Context, req kmsg.Request) []kgo.ResponseShard {
	if f.requestShardedFunc == nil {
		return nil
	}
	return f.requestShardedFunc(ctx, req)
}
