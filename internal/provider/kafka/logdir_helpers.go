package kafka

import (
	"github.com/twmb/franz-go/pkg/kadm"
)

// brokerIDSet builds a lookup set from a slice of broker IDs. A nil or empty
// slice yields a nil set, which callers treat as "no filter".
func brokerIDSet(ids []int32) map[int32]struct{} {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// topicSet builds a lookup set from a slice of topic names. A nil or empty
// slice yields a nil set, which callers treat as "no filter".
func topicSet(topics []string) map[string]struct{} {
	if len(topics) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		set[t] = struct{}{}
	}
	return set
}

// brokerMatches reports whether the broker ID passes the optional filter.
func brokerMatches(set map[int32]struct{}, broker int32) bool {
	if set == nil {
		return true
	}
	_, ok := set[broker]
	return ok
}

// topicMatches reports whether the topic passes the optional filter.
func topicMatches(set map[string]struct{}, topic string) bool {
	if set == nil {
		return true
	}
	_, ok := set[topic]
	return ok
}

// sortedLogDirs returns the described log dirs flattened and sorted by broker
// then directory, applying the optional broker filter.
func sortedLogDirs(all kadm.DescribedAllLogDirs, brokers map[int32]struct{}) []kadm.DescribedLogDir {
	out := make([]kadm.DescribedLogDir, 0)
	for _, d := range all.Sorted() {
		if brokerMatches(brokers, d.Broker) {
			out = append(out, d)
		}
	}
	return out
}
