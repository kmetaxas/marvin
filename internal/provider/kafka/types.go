package kafka

// BrokerInfo represents a Kafka broker in result data.
type BrokerInfo struct {
	ID           int32  `json:"broker_id"`
	Host         string `json:"host"`
	Port         int32  `json:"port"`
	Rack         string `json:"rack,omitempty"`
	IsController bool   `json:"is_controller,omitempty"`
}

// ConfigEntry represents a configuration key-value pair.
type ConfigEntry struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Default   bool   `json:"is_default"`
	Source    string `json:"source,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	ReadOnly  bool   `json:"read_only,omitempty"`
}

// TopicSummary represents a topic in list results.
type TopicSummary struct {
	Name       string `json:"topic"`
	Internal   bool   `json:"internal"`
	Partitions int    `json:"partition_count"`
}

// ConsumerGroupSummary represents a consumer group.
type ConsumerGroupSummary struct {
	GroupID      string     `json:"group_id"`
	State        string     `json:"state"`
	Protocol     string     `json:"protocol,omitempty"`
	ProtocolType string     `json:"protocol_type,omitempty"`
	Coordinator  BrokerInfo `json:"coordinator,omitempty"`
}

// PartitionInfo represents partition metadata.
type PartitionInfo struct {
	Topic       string  `json:"topic"`
	Partition   int32   `json:"partition"`
	Leader      int32   `json:"leader_id"`
	Replicas    []int32 `json:"replicas"`
	ISR         []int32 `json:"isr"`
	LeaderEpoch int32   `json:"leader_epoch,omitempty"`
	Offline     bool    `json:"offline,omitempty"`
}
