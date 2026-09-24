package log

import (
	"context"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
)

// fakeGraylogClient is a test double implementing the graylogClient interface.
type fakeGraylogClient struct {
	searchMessagesFunc  func(ctx context.Context, req searchMessagesRequest) (*searchMessagesResponse, error)
	searchAggregateFunc func(ctx context.Context, req searchAggregateRequest) (*searchAggregateResponse, error)
	configured          bool
	guardrails          logGuardrails
}

func (f *fakeGraylogClient) SearchMessages(ctx context.Context, req searchMessagesRequest) (*searchMessagesResponse, error) {
	return f.searchMessagesFunc(ctx, req)
}

func (f *fakeGraylogClient) SearchAggregate(ctx context.Context, req searchAggregateRequest) (*searchAggregateResponse, error) {
	return f.searchAggregateFunc(ctx, req)
}

func (f *fakeGraylogClient) IsConfigured() bool { return f.configured }

func (f *fakeGraylogClient) Guardrails() logGuardrails {
	g := f.guardrails
	g.ApplyDefaults()
	return g
}

func newFakeClientProvider(client graylogClient) *Provider {
	p := NewProvider(config.GraylogConfig{URL: "https://graylog.example.com:9000/api"})
	cfg := p.state.Config()
	p.mu.Lock()
	p.state = autoconfig.NewState(&p.mu, cfg, client)
	p.mu.Unlock()
	return p
}
