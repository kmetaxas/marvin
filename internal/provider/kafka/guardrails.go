package kafka

import "time"

// GuardrailPolicy holds configurable limits for all Kafka operations.
type GuardrailPolicy struct {
	MaxTopicsPerRequest     int
	MaxPartitionsPerRequest int
	MaxConsumerGroups       int
	MaxRecordsPerSample     int
	MaxResponseBytes        int
	MaxQueryDuration        time.Duration
}

// DefaultGuardrailPolicy returns the built-in defaults.
func DefaultGuardrailPolicy() GuardrailPolicy {
	return GuardrailPolicy{
		MaxTopicsPerRequest:     100,
		MaxPartitionsPerRequest: 1000,
		MaxConsumerGroups:       1000,
		MaxRecordsPerSample:     100,
		MaxResponseBytes:        10 * 1024 * 1024,
		MaxQueryDuration:        30 * time.Second,
	}
}

// ApplyDefaults fills zero values with defaults.
func (g *GuardrailPolicy) ApplyDefaults() {
	d := DefaultGuardrailPolicy()
	if g.MaxTopicsPerRequest == 0 {
		g.MaxTopicsPerRequest = d.MaxTopicsPerRequest
	}
	if g.MaxPartitionsPerRequest == 0 {
		g.MaxPartitionsPerRequest = d.MaxPartitionsPerRequest
	}
	if g.MaxConsumerGroups == 0 {
		g.MaxConsumerGroups = d.MaxConsumerGroups
	}
	if g.MaxRecordsPerSample == 0 {
		g.MaxRecordsPerSample = d.MaxRecordsPerSample
	}
	if g.MaxResponseBytes == 0 {
		g.MaxResponseBytes = d.MaxResponseBytes
	}
	if g.MaxQueryDuration == 0 {
		g.MaxQueryDuration = d.MaxQueryDuration
	}
}
