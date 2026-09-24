package log

import (
	"fmt"
	"time"
)

// logGuardrails holds configurable limits for Graylog queries.
type logGuardrails struct {
	MaxQueryRange       time.Duration
	MaxResults          int
	MaxHistogramBuckets int
	QueryTimeout        time.Duration
}

// ApplyDefaults fills in zero values with safe defaults.
func (g *logGuardrails) ApplyDefaults() {
	if g.MaxQueryRange <= 0 {
		g.MaxQueryRange = 24 * time.Hour
	}
	if g.MaxResults <= 0 {
		g.MaxResults = 100
	}
	if g.MaxHistogramBuckets <= 0 {
		g.MaxHistogramBuckets = 100
	}
	if g.QueryTimeout <= 0 {
		g.QueryTimeout = 30 * time.Second
	}
}

// ValidateTimerange checks that a timerange does not exceed MaxQueryRange.
func (g logGuardrails) ValidateTimerange(tr *timerange) error {
	if tr == nil {
		return nil
	}
	switch tr.Type {
	case "relative":
		span := time.Duration(tr.Range) * time.Second
		if span > g.MaxQueryRange {
			return fmt.Errorf("timerange span %v exceeds maximum %v", span, g.MaxQueryRange)
		}
	case "absolute":
		if tr.From == "" || tr.To == "" {
			return fmt.Errorf("absolute timerange requires both from and to")
		}
		from, err := time.Parse(time.RFC3339, tr.From)
		if err != nil {
			return fmt.Errorf("timerange.from must be RFC3339: %w", err)
		}
		to, err := time.Parse(time.RFC3339, tr.To)
		if err != nil {
			return fmt.Errorf("timerange.to must be RFC3339: %w", err)
		}
		span := to.Sub(from)
		if span < 0 {
			span = -span
		}
		if span > g.MaxQueryRange {
			return fmt.Errorf("timerange span %v exceeds maximum %v", span, g.MaxQueryRange)
		}
	case "keyword":
		// Cannot validate span for keyword timeranges; trust the guardrail.
		// Control plane can choose to avoid keyword timeranges for strict guardrails.
		return nil
	default:
		return fmt.Errorf("unsupported timerange type %q", tr.Type)
	}
	return nil
}

// LimitResults clamps results to MaxResults.
func (g logGuardrails) LimitResults(n int) int {
	if n <= 0 {
		return g.MaxResults
	}
	if n > g.MaxResults {
		return g.MaxResults
	}
	return n
}

// LimitBuckets clamps histogram buckets to MaxHistogramBuckets.
func (g logGuardrails) LimitBuckets(n int) int {
	if n <= 0 {
		return 0
	}
	if n > g.MaxHistogramBuckets {
		return g.MaxHistogramBuckets
	}
	return n
}
