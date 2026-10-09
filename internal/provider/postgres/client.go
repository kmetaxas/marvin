package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marvin-agent/marvin/internal/config"
)

type PostgresClient interface {
	Ping(ctx context.Context) error
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Close() error
	Guardrails() config.PostgresGuardrails
}

// PoolStats is a snapshot of the connection pool counters.
type PoolStats struct {
	TotalConns        int32
	IdleConns         int32
	AcquiredConns     int32
	MaxConns          int32
	EmptyAcquireCount int64
}

type clientImpl struct {
	pool       *pgxpool.Pool
	initErr    error
	guardrails config.PostgresGuardrails
}

var newPostgresClient = func(cfg config.PostgresConfig) *clientImpl {
	return newClient(cfg)
}

func newClient(cfg config.PostgresConfig) *clientImpl {
	poolCfg, err := parsePostgresConfig(cfg)
	if err != nil {
		return &clientImpl{initErr: fmt.Errorf("parse postgres config: %w", err)}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return &clientImpl{initErr: fmt.Errorf("create pgxpool: %w", err)}
	}

	return &clientImpl{
		pool:       pool,
		guardrails: cfg.Guardrails,
	}
}

func (c *clientImpl) Ping(ctx context.Context) error {
	if c.initErr != nil {
		return c.initErr
	}
	if c.guardrails.StatementTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.guardrails.StatementTimeout)
		defer cancel()
	}
	return c.pool.Ping(ctx)
}

func (c *clientImpl) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	if c.guardrails.StatementTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.guardrails.StatementTimeout)
		defer cancel()
	}
	return c.pool.Query(ctx, sql, args...)
}

func (c *clientImpl) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if c.initErr != nil {
		return &errorRow{err: c.initErr}
	}
	if c.guardrails.StatementTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.guardrails.StatementTimeout)
		defer cancel()
	}
	return c.pool.QueryRow(ctx, sql, args...)
}

func (c *clientImpl) Close() error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}

func (c *clientImpl) Guardrails() config.PostgresGuardrails {
	return c.guardrails
}

type errorRow struct {
	err error
}

func (r *errorRow) Scan(dest ...any) error {
	return r.err
}

func parsePostgresConfig(cfg config.PostgresConfig) (*pgxpool.Config, error) {
	var connString string
	if cfg.DSN != "" {
		connString = cfg.DSN
	} else {
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 5432
		}
		connString = fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
			cfg.User, cfg.Password, host, port, cfg.Database)
	}

	poolCfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse connection string: %w", err)
	}

	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	if cfg.TLS.Enabled {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		poolCfg.ConnConfig.TLSConfig = tlsCfg
	}

	// Defense-in-depth: set read-only mode after connection
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET default_transaction_read_only = on")
		return err
	}

	return poolCfg, nil
}

func buildTLSConfig(cfg config.PostgresTLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.CAFile != "" || cfg.CAData != "" {
		pool := x509.NewCertPool()
		pemData, err := pemDataFromConfig(cfg.CAFile, cfg.CAData)
		if err != nil {
			return nil, fmt.Errorf("load CA: %w", err)
		}
		if ok := pool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("invalid CA PEM data")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.CertFile != "" || cfg.CertData != "" {
		certPEM, err := pemDataFromConfig(cfg.CertFile, cfg.CertData)
		if err != nil {
			return nil, fmt.Errorf("load cert: %w", err)
		}
		keyPEM, err := pemDataFromConfig(cfg.KeyFile, cfg.KeyData)
		if err != nil {
			return nil, fmt.Errorf("load key: %w", err)
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse cert/key pair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

func pemDataFromConfig(filePath, base64Data string) ([]byte, error) {
	if filePath != "" {
		return os.ReadFile(filePath)
	}
	if base64Data != "" {
		return base64.StdEncoding.DecodeString(base64Data)
	}
	return nil, fmt.Errorf("neither file path nor base64 data provided")
}
