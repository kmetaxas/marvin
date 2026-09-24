package log

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
)

// graylogClient is the minimal interface tasks need.
type graylogClient interface {
	SearchMessages(ctx context.Context, req searchMessagesRequest) (*searchMessagesResponse, error)
	SearchAggregate(ctx context.Context, req searchAggregateRequest) (*searchAggregateResponse, error)
	IsConfigured() bool
	Guardrails() logGuardrails
}

// clientImpl is a thin HTTP client for the Graylog Search Scripting API.
type clientImpl struct {
	baseURL    *url.URL
	httpClient *http.Client
	guardrails logGuardrails
	authUser   string
	authPass   string
}

// newGraylogClient creates a new graylogClient from config.
// It is a swappable package variable for test injection.
var newGraylogClient = func(cfg config.GraylogConfig) graylogClient {
	if cfg.URL == "" {
		return &clientImpl{}
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		return &clientImpl{}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	g := logGuardrails{
		MaxQueryRange:       cfg.Guardrails.MaxQueryRange,
		MaxResults:          cfg.Guardrails.MaxResults,
		MaxHistogramBuckets: cfg.Guardrails.MaxHistogramBuckets,
		QueryTimeout:        cfg.Guardrails.QueryTimeout,
	}
	g.ApplyDefaults()

	var user, pass string
	switch cfg.Auth.Type {
	case "token":
		user = cfg.Auth.Token
		pass = "token"
	case "basic":
		user = cfg.Auth.Username
		pass = cfg.Auth.Password
	}

	return &clientImpl{
		baseURL:    u,
		httpClient: &http.Client{Timeout: timeout},
		guardrails: g,
		authUser:   user,
		authPass:   pass,
	}
}

func (c *clientImpl) IsConfigured() bool {
	return c != nil && c.baseURL != nil && c.baseURL.String() != ""
}

func (c *clientImpl) Guardrails() logGuardrails {
	if c == nil {
		return logGuardrails{}
	}
	g := c.guardrails
	g.ApplyDefaults()
	return g
}

func (c *clientImpl) SearchMessages(ctx context.Context, req searchMessagesRequest) (*searchMessagesResponse, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("graylog client is not configured")
	}

	u := c.baseURL.JoinPath("search", "messages")
	resp, err := c.doPost(ctx, u.String(), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graylog search/messages returned status %d", resp.StatusCode)
	}

	var result searchMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search/messages response: %w", err)
	}
	return &result, nil
}

func (c *clientImpl) SearchAggregate(ctx context.Context, req searchAggregateRequest) (*searchAggregateResponse, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("graylog client is not configured")
	}

	u := c.baseURL.JoinPath("search", "aggregate")
	resp, err := c.doPost(ctx, u.String(), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graylog search/aggregate returned status %d", resp.StatusCode)
	}

	var result searchAggregateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search/aggregate response: %w", err)
	}
	return &result, nil
}

func (c *clientImpl) doPost(ctx context.Context, url string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.authUser != "" || c.authPass != "" {
		req.SetBasicAuth(c.authUser, c.authPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

// basicAuthHeader returns the Authorization header value for Basic auth.
// Useful for tests verifying header contents.
func basicAuthHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}
