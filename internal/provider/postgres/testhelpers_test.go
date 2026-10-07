package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
)

// fakePostgresClient implements PostgresClient for tests. Only QueryRow is
// exercised by most tasks; the remaining methods are no-ops.
type fakePostgresClient struct {
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	guardrails   config.PostgresGuardrails
	poolStats    PoolStats
}

func (f *fakePostgresClient) Ping(ctx context.Context) error { return nil }

func (f *fakePostgresClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakePostgresClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFunc == nil {
		return &errorRow{err: errors.New("no queryRowFunc configured")}
	}
	return f.queryRowFunc(ctx, sql, args...)
}

func (f *fakePostgresClient) Close() error { return nil }

func (f *fakePostgresClient) Guardrails() config.PostgresGuardrails { return f.guardrails }

func (f *fakePostgresClient) Stat() PoolStats { return f.poolStats }

// fakeRow is a pgx.Row that copies canned values into the scan destinations.
type fakeRow struct {
	values []any
	err    error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan destination count mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = r.values[i].(string)
		case *int:
			*p = r.values[i].(int)
		case *int32:
			*p = r.values[i].(int32)
		case *int64:
			*p = r.values[i].(int64)
		case *bool:
			*p = r.values[i].(bool)
		default:
			return errors.New("unsupported scan destination type")
		}
	}
	return nil
}

// fakePostgresProvider builds a Provider whose state holds the given fake client.
func fakePostgresProvider(client PostgresClient) *Provider {
	p := NewProvider(config.PostgresConfig{Host: "localhost"})
	cfg := p.state.Config()
	p.mu.Lock()
	p.state = autoconfig.NewState(&p.mu, cfg, client)
	p.mu.Unlock()
	return p
}
