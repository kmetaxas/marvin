package kafka

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider is the runtime-configurable Kafka provider.
type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.KafkaConfig, *clientImpl]
	base  provider.BaseProvider
}

// Name returns the provider name.
func (p *Provider) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.base.ProviderName
}

// Capabilities returns a defensive copy of the provider capabilities.
func (p *Provider) Capabilities() []capability.Capability {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]capability.Capability, len(p.base.ProviderCapabilities))
	copy(out, p.base.ProviderCapabilities)
	return out
}

// IsEnabled always returns true for the Kafka provider.
func (p *Provider) IsEnabled(config map[string]any) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.base.EnabledFunc != nil {
		return p.base.EnabledFunc(config)
	}
	return true
}

// GetTask looks up a task by capability name.
func (p *Provider) GetTask(name string) (task.Task, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.base.Tasks == nil {
		return nil, false
	}
	t, ok := p.base.Tasks[name]
	return t, ok
}

// CurrentClient returns the current Kafka client under a read lock.
func (p *Provider) CurrentClient() KafkaClient {
	return p.state.Client()
}

func (p *Provider) IsConfigured() bool {
	return len(p.state.Config().BootstrapServers) > 0
}

// UpdateConfig parses cfg fields and, if the configuration has changed,
// updates the stored config and recreates the Kafka client.
func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName // not used; Kafka uses a single shared config
	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.KafkaConfig, *clientImpl]{
		Parse: parseKafkaConfig,
		Build: func(cfg config.KafkaConfig) (*clientImpl, error) {
			return newKafkaClient(cfg), nil
		},
		Close: func(client *clientImpl) {
			client.close()
		},
	})
}

func parseKafkaConfig(_ config.KafkaConfig, cfg map[string]any) (config.KafkaConfig, error) {
	out := config.KafkaConfig{}

	if v, ok := autoconfig.StringSliceOK(cfg, "bootstrap_servers"); ok {
		out.BootstrapServers = v
	}

	out.TLS, _ = parseKafkaTLSConfig(config.KafkaTLSConfig{}, cfg)
	out.SASL, _ = parseKafkaSASLConfig(config.KafkaSASLConfig{}, cfg)

	if d, ok := autoconfig.DurationOK(cfg, "dial_timeout"); ok {
		out.DialTimeout = d
	}
	if d, ok := autoconfig.DurationOK(cfg, "request_timeout"); ok {
		out.RequestTimeout = d
	}
	if d, ok := autoconfig.DurationOK(cfg, "metadata_max_age"); ok {
		out.MetadataMaxAge = d
	}

	out.Guardrails, _ = parseKafkaGuardrails(config.KafkaGuardrails{}, cfg)

	return out, nil
}

func parseKafkaTLSConfig(_ config.KafkaTLSConfig, cfg map[string]any) (config.KafkaTLSConfig, error) {
	out := config.KafkaTLSConfig{}

	if v, ok := autoconfig.BoolOK(cfg, "tls.enabled"); ok {
		out.Enabled = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.ca_file"); ok {
		out.CAFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.ca_data"); ok {
		out.CAData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.cert_file"); ok {
		out.CertFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.cert_data"); ok {
		out.CertData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.key_file"); ok {
		out.KeyFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.key_data"); ok {
		out.KeyData = v
	}
	if v, ok := autoconfig.BoolOK(cfg, "tls.insecure_skip_verify"); ok {
		out.Insecure = v
	}

	return out, nil
}

func parseKafkaSASLConfig(_ config.KafkaSASLConfig, cfg map[string]any) (config.KafkaSASLConfig, error) {
	out := config.KafkaSASLConfig{}

	if v, ok := autoconfig.StringOK(cfg, "sasl.mechanism"); ok {
		out.Mechanism = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.username"); ok {
		out.Username = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.password"); ok {
		out.Password = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.kerberos_service_name"); ok {
		out.KerberosServiceName = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.kerberos_realm"); ok {
		out.KerberosRealm = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.keytab_path"); ok {
		out.KeytabPath = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.keytab_data"); ok {
		out.KeytabData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.krb5_conf_data"); ok {
		out.Krb5ConfData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "sasl.krb5_conf_path"); ok {
		out.Krb5ConfPath = v
	}

	return out, nil
}

func parseKafkaGuardrails(_ config.KafkaGuardrails, cfg map[string]any) (config.KafkaGuardrails, error) {
	out := config.KafkaGuardrails{}

	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_topics_per_request"); ok {
		out.MaxTopicsPerRequest = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_partitions_per_request"); ok {
		out.MaxPartitionsPerRequest = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_consumer_groups"); ok {
		out.MaxConsumerGroups = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_records_per_sample"); ok {
		out.MaxRecordsPerSample = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_response_bytes"); ok {
		out.MaxResponseBytes = v
	}
	if v, ok := autoconfig.DurationOK(cfg, "guardrails.max_query_duration"); ok {
		out.MaxQueryDuration = v
	}

	return out, nil
}

func NewProvider(cfg config.KafkaConfig) *Provider {
	client := newKafkaClient(cfg)
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, client)
	initialClient := p.state.Client()

	tasks := []task.Task{
		&clusterDescribeTask{client: initialClient},
		&brokerListTask{client: initialClient},
		&brokerDescribeTask{client: initialClient},
		&brokerConfigGetTask{client: initialClient},
		&brokerConfigDiffTask{client: initialClient},
		&clusterConfigGetTask{client: initialClient},
		&clusterFeaturesGetTask{client: initialClient},
		&clusterApiVersionsGetTask{client: initialClient},
		&clusterMetadataGetTask{client: initialClient},
		&logdirListTask{client: initialClient},
		&logdirDescribeTask{client: initialClient},
		&logdirReplicaSizesTask{client: initialClient},
		&logdirErrorsListTask{client: initialClient},
		&storageTopicSizesTask{client: initialClient},
		&consumerGroupListTask{client: initialClient},
		&consumerGroupDescribeTask{client: initialClient},
		&consumerGroupMembersListTask{client: initialClient},
		&consumerGroupOffsetsGetTask{client: initialClient},
		&consumerGroupLagGetTask{client: initialClient},
		&consumerGroupCoordinatorGetTask{client: initialClient},
		&consumerGroupStateGetTask{client: initialClient},
		&consumerGroupAssignmentGetTask{client: initialClient},
		&consumerGroupOffsetCommitVerifyTask{client: initialClient},
		&topicListTask{client: initialClient},
		&topicDescribeTask{client: initialClient},
		&topicConfigGetTask{client: initialClient},
		&topicConfigDiffTask{client: initialClient},
		&partitionDescribeTask{client: initialClient},
		&partitionListTask{client: initialClient},
		&partitionOfflineListTask{client: initialClient},
		&partitionUnderReplicatedListTask{client: initialClient},
		&partitionUnderMinISRListTask{client: initialClient},
		&partitionISRDescribeTask{client: initialClient},
		&partitionLeaderDistributionTask{client: initialClient},
		&partitionReplicaDistributionTask{client: initialClient},
		&partitionLogdirsGetTask{client: initialClient},
		&partitionOffsetsGetTask{client: initialClient},
		&partitionOffsetsCompareTask{client: initialClient},
	}

	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "kafka",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.base = provider.BaseProvider{
		ProviderName:         "kafka",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	// Wire tasks back to the provider so they can call CurrentClient().
	for _, t := range tasks {
		switch task := t.(type) {
		case *clusterDescribeTask:
			task.provider = p
		case *brokerListTask:
			task.provider = p
		case *brokerDescribeTask:
			task.provider = p
		case *brokerConfigGetTask:
			task.provider = p
		case *brokerConfigDiffTask:
			task.provider = p
		case *clusterConfigGetTask:
			task.provider = p
		case *clusterFeaturesGetTask:
			task.provider = p
		case *clusterApiVersionsGetTask:
			task.provider = p
		case *clusterMetadataGetTask:
			task.provider = p
		case *logdirListTask:
			task.provider = p
		case *logdirDescribeTask:
			task.provider = p
		case *logdirReplicaSizesTask:
			task.provider = p
		case *logdirErrorsListTask:
			task.provider = p
		case *storageTopicSizesTask:
			task.provider = p
		case *consumerGroupListTask:
			task.provider = p
		case *consumerGroupDescribeTask:
			task.provider = p
		case *consumerGroupMembersListTask:
			task.provider = p
		case *consumerGroupOffsetsGetTask:
			task.provider = p
		case *consumerGroupLagGetTask:
			task.provider = p
		case *consumerGroupCoordinatorGetTask:
			task.provider = p
		case *consumerGroupStateGetTask:
			task.provider = p
		case *consumerGroupAssignmentGetTask:
			task.provider = p
		case *consumerGroupOffsetCommitVerifyTask:
			task.provider = p
		case *topicListTask:
			task.provider = p
		case *topicDescribeTask:
			task.provider = p
		case *topicConfigGetTask:
			task.provider = p
		case *topicConfigDiffTask:
			task.provider = p
		case *partitionDescribeTask:
			task.provider = p
		case *partitionListTask:
			task.provider = p
		case *partitionOfflineListTask:
			task.provider = p
		case *partitionUnderReplicatedListTask:
			task.provider = p
		case *partitionUnderMinISRListTask:
			task.provider = p
		case *partitionISRDescribeTask:
			task.provider = p
		case *partitionLeaderDistributionTask:
			task.provider = p
		case *partitionReplicaDistributionTask:
			task.provider = p
		case *partitionLogdirsGetTask:
			task.provider = p
		case *partitionOffsetsGetTask:
			task.provider = p
		case *partitionOffsetsCompareTask:
			task.provider = p
		}
	}

	return p
}

func capabilityDescription(name string) string {
	descriptions := map[string]string{
		"kafka.cluster.describe":                    "Describes basic cluster identity and metadata",
		"kafka.broker.list":                         "Lists brokers currently visible in cluster metadata",
		"kafka.broker.describe":                     "Describes detailed metadata for one broker",
		"kafka.broker.config.get":                   "Reads effective broker configuration",
		"kafka.broker.config.diff":                  "Compares effective configuration between brokers",
		"kafka.cluster.config.get":                  "Retrieves cluster-wide dynamic/default broker configuration",
		"kafka.cluster.features.get":                "Retrieves Kafka feature/version metadata",
		"kafka.cluster.api_versions.get":            "Queries supported Kafka protocol API versions",
		"kafka.cluster.metadata.get":                "Returns raw-ish cluster metadata snapshot",
		"kafka.logdir.list":                         "Lists Kafka log directories known for brokers",
		"kafka.logdir.describe":                     "Describes log directories including replica sizes and errors",
		"kafka.logdir.replica_sizes.get":            "Retrieves replica sizes grouped by broker/logdir/topic",
		"kafka.logdir.errors.list":                  "Finds log directories reporting Kafka storage errors",
		"kafka.storage.topic_sizes.get":             "Aggregates partition sizes into topic/broker totals",
		"kafka.consumer_group.list":                 "Lists consumer groups and their states",
		"kafka.consumer_group.describe":             "Describes a consumer group's state, protocol, coordinator and members",
		"kafka.consumer_group.members.list":         "Lists detailed members and assignments of a consumer group",
		"kafka.consumer_group.offsets.get":          "Returns committed offsets for a consumer group",
		"kafka.consumer_group.lag.get":              "Calculates consumer lag per partition with optional aggregation",
		"kafka.consumer_group.coordinator.get":      "Finds the coordinator broker for a consumer group",
		"kafka.consumer_group.state.get":            "Returns lightweight consumer group state",
		"kafka.consumer_group.assignment.get":       "Returns assignment distribution among group members",
		"kafka.consumer_group.offset_commit.verify": "Non-mutating protocol path check for offset commits",
		"kafka.topic.list":                          "Lists topics, optionally filtering internal topics",
		"kafka.topic.describe":                      "Describes topic partitions, replicas, ISR, leaders and configuration summary",
		"kafka.topic.config.get":                    "Reads effective topic configuration",
		"kafka.topic.config.diff":                   "Compares configuration between several topics",
		"kafka.partition.describe":                  "Describes detailed state for one partition",
		"kafka.partition.list":                      "Returns partition metadata across selected/all topics with filters",
		"kafka.partition.offline.list":              "Returns partitions with no active leader",
		"kafka.partition.under_replicated.list":     "Returns partitions where ISR is smaller than replica set",
		"kafka.partition.under_min_isr.list":        "Finds partitions below configured min.insync.replicas",
		"kafka.partition.isr.describe":              "Returns detailed replica/ISR state for partitions",
		"kafka.partition.leader_distribution.get":   "Gets leader distribution across brokers",
		"kafka.partition.replica_distribution.get":  "Gets replica distribution across brokers/racks",
		"kafka.partition.logdirs.get":               "Finds which broker/log directory hosts each replica",
		"kafka.partition.offsets.get":               "Retrieves earliest/latest offsets for partitions",
		"kafka.partition.offsets.compare":           "Compares high-water/end offsets across replicas",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Kafka capability"
}
