package log

import "fmt"

// parseTimerange extracts a timerange from params with a sensible default.
func parseTimerange(params map[string]any) (*timerange, error) {
	raw, ok := params["timerange"]
	if !ok || raw == nil {
		return &timerange{Type: "relative", Range: 900}, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("timerange must be an object")
	}

	tr := &timerange{}
	if v, ok := m["type"].(string); ok {
		tr.Type = v
	}
	if tr.Type == "" {
		return nil, fmt.Errorf("timerange.type is required")
	}

	switch tr.Type {
	case "relative":
		if v, ok := m["range"]; ok {
			switch n := v.(type) {
			case int:
				tr.Range = n
			case int64:
				tr.Range = int(n)
			case float64:
				tr.Range = int(n)
			default:
				return nil, fmt.Errorf("timerange.range must be an integer (seconds)")
			}
		}
		if tr.Range <= 0 {
			tr.Range = 900
		}
	case "absolute":
		if v, ok := m["from"].(string); ok {
			tr.From = v
		}
		if v, ok := m["to"].(string); ok {
			tr.To = v
		}
	case "keyword":
		if v, ok := m["keyword"].(string); ok {
			tr.Keyword = v
		}
	default:
		return nil, fmt.Errorf("unsupported timerange.type %q", tr.Type)
	}

	return tr, nil
}

// parseStreams extracts an optional streams list from params.
func parseStreams(params map[string]any) ([]string, error) {
	raw, ok := params["streams"]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("streams entries must be strings")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("streams must be an array of strings")
	}
}

// parseFields extracts an optional fields list from params with a default.
func parseFields(params map[string]any, defaultFields []string) ([]string, error) {
	raw, ok := params["fields"]
	if !ok || raw == nil {
		return defaultFields, nil
	}

	switch v := raw.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("fields entries must be strings")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("fields must be an array of strings")
	}
}

// parseInterval extracts an optional interval from params with a default.
func parseInterval(params map[string]any, defaultValue int, defaultUnit string) (*interval, error) {
	raw, ok := params["interval"]
	if !ok || raw == nil {
		return &interval{Type: "timeunit", Value: defaultValue, Unit: defaultUnit}, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("interval must be an object")
	}

	iv := &interval{Type: "timeunit"}
	if v, ok := m["value"]; ok {
		switch n := v.(type) {
		case int:
			iv.Value = n
		case int64:
			iv.Value = int(n)
		case float64:
			iv.Value = int(n)
		default:
			return nil, fmt.Errorf("interval.value must be an integer")
		}
	}
	if iv.Value <= 0 {
		iv.Value = defaultValue
	}
	if v, ok := m["unit"].(string); ok && v != "" {
		iv.Unit = v
	} else {
		iv.Unit = defaultUnit
	}

	return iv, nil
}

func makeTimestamp(r aggregateRow) string {
	return r.Key
}

func makeCount(r aggregateRow) int {
	if v, ok := r.Values["count"]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}
