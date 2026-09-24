package autoconfig

import (
	"math"
	"strings"
	"time"
)

func Lookup(raw map[string]any, key string) (any, bool) {
	if raw == nil {
		return nil, false
	}
	if v, ok := raw[key]; ok {
		return v, true
	}

	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		return nil, false
	}

	cur := any(raw)
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func String(raw map[string]any, key string) string {
	v, _ := StringOK(raw, key)
	return v
}

func StringOK(raw map[string]any, key string) (string, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return "", false
	}
	return AsStringOK(v)
}

func Bool(raw map[string]any, key string) bool {
	v, _ := BoolOK(raw, key)
	return v
}

func BoolOK(raw map[string]any, key string) (bool, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return false, false
	}
	return AsBoolOK(v)
}

func Int(raw map[string]any, key string) int {
	v, _ := IntOK(raw, key)
	return v
}

func IntOK(raw map[string]any, key string) (int, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return 0, false
	}
	return AsIntOK(v)
}

func Int64OK(raw map[string]any, key string) (int64, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return 0, false
	}
	return asInt64OK(v)
}

func Duration(raw map[string]any, key string) time.Duration {
	v, _ := DurationOK(raw, key)
	return v
}

func DurationOK(raw map[string]any, key string) (time.Duration, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return 0, false
	}
	return AsDurationOK(v)
}

func StringSlice(raw map[string]any, key string) []string {
	v, _ := StringSliceOK(raw, key)
	return v
}

func StringSliceOK(raw map[string]any, key string) ([]string, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return nil, false
	}
	return AsStringSliceOK(v)
}

func Map(raw map[string]any, key string) map[string]any {
	v, _ := MapOK(raw, key)
	return v
}

func MapOK(raw map[string]any, key string) (map[string]any, bool) {
	v, ok := Lookup(raw, key)
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

func AsString(v any) string {
	out, _ := AsStringOK(v)
	return out
}

func AsStringOK(v any) (string, bool) {
	out, ok := v.(string)
	return out, ok
}

func AsBoolOK(v any) (bool, bool) {
	out, ok := v.(bool)
	return out, ok
}

func AsInt(v any) int {
	out, _ := AsIntOK(v)
	return out
}

func AsIntOK(v any) (int, bool) {
	i, ok := asInt64OK(v)
	if !ok || i < int64(minInt()) || i > int64(maxInt()) {
		return 0, false
	}
	return int(i), true
}

func AsDurationOK(v any) (time.Duration, bool) {
	switch n := v.(type) {
	case time.Duration:
		return n, true
	case string:
		d, err := time.ParseDuration(n)
		if err != nil {
			return 0, false
		}
		return d, true
	case float32:
		return durationFromFloatSeconds(float64(n))
	case float64:
		return durationFromFloatSeconds(n)
	}

	seconds, ok := asInt64OK(v)
	if !ok || seconds < 0 || seconds > int64(math.MaxInt64/int64(time.Second)) {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func AsStringSliceOK(v any) ([]string, bool) {
	switch values := v.(type) {
	case nil:
		return nil, false
	case []string:
		out := make([]string, len(values))
		copy(out, values)
		return out, true
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func asInt64OK(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		if uint64(n) > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float32:
		return int64FromFloat(float64(n))
	case float64:
		return int64FromFloat(n)
	default:
		return 0, false
	}
}

func int64FromFloat(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f {
		return 0, false
	}
	if f < float64(math.MinInt64) || f > float64(math.MaxInt64) {
		return 0, false
	}
	return int64(f), true
}

func durationFromFloatSeconds(seconds float64) (time.Duration, bool) {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, false
	}
	nanos := seconds * float64(time.Second)
	if nanos > float64(math.MaxInt64) {
		return 0, false
	}
	return time.Duration(nanos), true
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}
