package prometheus

import "time"

// GuardrailPolicy holds configurable limits for all Prometheus operations.
// It is designed to be populated from YAML config today, and from Control Plane
// policy messages tomorrow.
type GuardrailPolicy struct {
	MaxQueryRange      time.Duration
	MinStep            time.Duration
	QueryTimeout       time.Duration
	MaxReturnedSeries  int
	MaxReturnedSamples int
	MaxMetadataResults int
	MaxResponseBytes   int
}

// DefaultGuardrailPolicy returns the built-in defaults.
func DefaultGuardrailPolicy() GuardrailPolicy {
	return GuardrailPolicy{
		MaxQueryRange:      168 * time.Hour,
		MinStep:            15 * time.Second,
		QueryTimeout:       30 * time.Second,
		MaxReturnedSeries:  1000,
		MaxReturnedSamples: 5000,
		MaxMetadataResults: 1000,
		MaxResponseBytes:   10 * 1024 * 1024,
	}
}

// ApplyDefaults fills zero values with defaults.
func (g *GuardrailPolicy) ApplyDefaults() {
	d := DefaultGuardrailPolicy()
	if g.MaxQueryRange == 0 {
		g.MaxQueryRange = d.MaxQueryRange
	}
	if g.MinStep == 0 {
		g.MinStep = d.MinStep
	}
	if g.QueryTimeout == 0 {
		g.QueryTimeout = d.QueryTimeout
	}
	if g.MaxReturnedSeries == 0 {
		g.MaxReturnedSeries = d.MaxReturnedSeries
	}
	if g.MaxReturnedSamples == 0 {
		g.MaxReturnedSamples = d.MaxReturnedSamples
	}
	if g.MaxMetadataResults == 0 {
		g.MaxMetadataResults = d.MaxMetadataResults
	}
	if g.MaxResponseBytes == 0 {
		g.MaxResponseBytes = d.MaxResponseBytes
	}
}
