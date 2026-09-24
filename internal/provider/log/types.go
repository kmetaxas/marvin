package log

// timerange represents a Graylog timerange specification.
type timerange struct {
	Type    string `json:"type"`              // "relative", "absolute", "keyword"
	Range   int    `json:"range,omitempty"`   // for relative: seconds
	From    string `json:"from,omitempty"`    // for absolute: ISO timestamp
	To      string `json:"to,omitempty"`      // for absolute: ISO timestamp
	Keyword string `json:"keyword,omitempty"` // for keyword: e.g. "last five minutes"
}

// searchMessagesRequest is the POST body for /api/search/messages.
type searchMessagesRequest struct {
	Query     string     `json:"query,omitempty"`
	Streams   []string   `json:"streams,omitempty"`
	Fields    []string   `json:"fields,omitempty"`
	From      int        `json:"from,omitempty"`
	Size      int        `json:"size,omitempty"`
	Sort      string     `json:"sort,omitempty"`
	SortOrder string     `json:"sort_order,omitempty"`
	Timerange *timerange `json:"timerange,omitempty"`
}

// searchMessagesResponse is the JSON response from /api/search/messages.
type searchMessagesResponse struct {
	Messages       []map[string]any `json:"messages"`
	TotalResults   int              `json:"total_results"`
	TimerangeStart string           `json:"timerange_start,omitempty"`
	TimerangeEnd   string           `json:"timerange_end,omitempty"`
}

// searchAggregateRequest is the POST body for /api/search/aggregate.
type searchAggregateRequest struct {
	Query     string     `json:"query,omitempty"`
	Streams   []string   `json:"streams,omitempty"`
	GroupBy   []groupBy  `json:"group_by,omitempty"`
	Metrics   []metric   `json:"metrics,omitempty"`
	Timerange *timerange `json:"timerange,omitempty"`
}

type groupBy struct {
	Field    string    `json:"field"`
	Interval *interval `json:"interval,omitempty"`
}

type interval struct {
	Type  string `json:"type"` // "timeunit"
	Value int    `json:"value"`
	Unit  string `json:"unit"` // "s", "m", "h", "d", "w", "M"
}

type metric struct {
	Function string `json:"function"` // "count", "avg", "min", "max", "sum"
	Field    string `json:"field,omitempty"`
}

// searchAggregateResponse is the JSON response from /api/search/aggregate.
type searchAggregateResponse struct {
	Rows []aggregateRow `json:"rows"`
}

type aggregateRow struct {
	Key    string         `json:"key"`
	Values map[string]any `json:"values"`
}
