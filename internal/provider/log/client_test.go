package log

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientSearchMessagesSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/messages", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		var req searchMessagesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "source:web-01", req.Query)

		resp := searchMessagesResponse{
			Messages:     []map[string]any{{"message": "hello"}},
			TotalResults: 42,
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client := newClientForTest(server.URL)
	resp, err := client.SearchMessages(context.Background(), searchMessagesRequest{Query: "source:web-01"})
	require.NoError(t, err)
	assert.Equal(t, 42, resp.TotalResults)
	assert.Len(t, resp.Messages, 1)
}

func TestClientSearchAggregateSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/aggregate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req searchAggregateRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		resp := searchAggregateResponse{
			Rows: []aggregateRow{
				{Key: "bucket1", Values: map[string]any{"count": 10}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client := newClientForTest(server.URL)
	resp, err := client.SearchAggregate(context.Background(), searchAggregateRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Rows, 1)
}

func TestClientTokenAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "mytoken", user)
		assert.Equal(t, "token", pass)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(searchMessagesResponse{}))
	}))
	defer server.Close()

	client := newGraylogClient(config.GraylogConfig{
		URL: server.URL,
		Auth: config.GraylogAuthConfig{
			Type:  "token",
			Token: "mytoken",
		},
	})
	_, err := client.SearchMessages(context.Background(), searchMessagesRequest{})
	require.NoError(t, err)
}

func TestClientBasicAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(searchMessagesResponse{}))
	}))
	defer server.Close()

	client := newGraylogClient(config.GraylogConfig{
		URL: server.URL,
		Auth: config.GraylogAuthConfig{
			Type:     "basic",
			Username: "admin",
			Password: "secret",
		},
	})
	_, err := client.SearchMessages(context.Background(), searchMessagesRequest{})
	require.NoError(t, err)
}

func TestClientContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(searchMessagesResponse{}))
	}))
	defer server.Close()

	client := newClientForTest(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.SearchMessages(ctx, searchMessagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestClientHTTPErrorStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, "Unauthorized")
	}))
	defer server.Close()

	client := newClientForTest(server.URL)
	_, err := client.SearchMessages(context.Background(), searchMessagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestClientIsConfigured(t *testing.T) {
	t.Parallel()

	c := &clientImpl{}
	assert.False(t, c.IsConfigured())

	u, _ := url.Parse("https://graylog.example.com")
	c2 := &clientImpl{baseURL: u}
	assert.True(t, c2.IsConfigured())
}

func TestClientGuardrailsDefaults(t *testing.T) {
	t.Parallel()

	client := newGraylogClient(config.GraylogConfig{URL: "https://example.com"})
	g := client.Guardrails()
	assert.Equal(t, 24*time.Hour, g.MaxQueryRange)
	assert.Equal(t, 100, g.MaxResults)
	assert.Equal(t, 100, g.MaxHistogramBuckets)
	assert.Equal(t, 30*time.Second, g.QueryTimeout)
}

func newClientForTest(baseURL string) graylogClient {
	return newGraylogClient(config.GraylogConfig{URL: baseURL})
}
