package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	krb5client "github.com/jcmturner/gokrb5/v8/client"
	krb5config "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/kversion"
	"github.com/twmb/franz-go/pkg/sasl"
	franzkerberos "github.com/twmb/franz-go/pkg/sasl/kerberos"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// KafkaClient is the narrow interface exposed to tasks.
type KafkaClient interface {
	Metadata(ctx context.Context, topics ...string) (kadm.Metadata, error)
	ApiVersions(ctx context.Context) (kadm.BrokersApiVersions, error)
	ListBrokers(ctx context.Context) (kadm.BrokerDetails, error)
	DescribeBrokerConfigs(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error)
	Request(ctx context.Context, req kmsg.Request) (kmsg.Response, error)
	Guardrails() GuardrailPolicy

	ListTopics(ctx context.Context, topics ...string) (kadm.TopicDetails, error)
	DescribeTopicConfigs(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error)
	DescribeAllLogDirs(ctx context.Context) (kadm.DescribedAllLogDirs, error)
	ListGroups(ctx context.Context, states ...string) (kadm.ListedGroups, error)
	DescribeGroups(ctx context.Context, groups ...string) (kadm.DescribedGroups, error)
	FetchOffsets(ctx context.Context, group string) (kadm.OffsetResponses, error)
	Lag(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error)
	FindGroupCoordinators(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses
	ListEndOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error)
	ListStartOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error)
	RequestSharded(ctx context.Context, req kmsg.Request) []kgo.ResponseShard
}

// clientImpl wraps kgo.Client + kadm.Client with guardrails.
type clientImpl struct {
	kgoClient  *kgo.Client
	admClient  *kadm.Client
	guardrails GuardrailPolicy
	initErr    error
	config     config.KafkaConfig
	tempPaths  []string
}

// newKafkaClient is a swappable constructor for test injection.
var newKafkaClient = func(cfg config.KafkaConfig) *clientImpl {
	c := &clientImpl{config: cfg}
	c.guardrails = GuardrailPolicy{
		MaxTopicsPerRequest:     cfg.Guardrails.MaxTopicsPerRequest,
		MaxPartitionsPerRequest: cfg.Guardrails.MaxPartitionsPerRequest,
		MaxConsumerGroups:       cfg.Guardrails.MaxConsumerGroups,
		MaxRecordsPerSample:     cfg.Guardrails.MaxRecordsPerSample,
		MaxResponseBytes:        cfg.Guardrails.MaxResponseBytes,
		MaxQueryDuration:        cfg.Guardrails.MaxQueryDuration,
	}
	c.guardrails.ApplyDefaults()

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.BootstrapServers...),
		kgo.ClientID("marvin-kafka"),
		kgo.MaxVersions(kversion.V2_6_0()),
	}

	if cfg.DialTimeout > 0 {
		opts = append(opts, kgo.DialTimeout(cfg.DialTimeout))
	}
	if cfg.RequestTimeout > 0 {
		opts = append(opts, kgo.RequestTimeoutOverhead(cfg.RequestTimeout))
	}
	if cfg.MetadataMaxAge > 0 {
		opts = append(opts, kgo.MetadataMaxAge(cfg.MetadataMaxAge))
	}

	if cfg.TLS.Enabled {
		tlsConfig, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			c.initErr = fmt.Errorf("build kafka tls config: %w", err)
			return c
		}
		opts = append(opts, kgo.DialTLSConfig(tlsConfig))
	}

	if cfg.SASL.Mechanism != "" {
		var mechanism sasl.Mechanism
		var err error
		if cfg.SASL.Mechanism == "GSSAPI" {
			mechanism, err = buildSASLMechanismWithTempFiles(cfg.SASL, &c.tempPaths)
		} else {
			mechanism, err = buildSASLMechanism(cfg.SASL)
		}
		if err != nil {
			c.initErr = fmt.Errorf("build kafka sasl mechanism: %w", err)
			return c
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		c.initErr = fmt.Errorf("create kafka client: %w", err)
		return c
	}

	c.kgoClient = cl
	c.admClient = kadm.NewClient(cl)
	return c
}

func (c *clientImpl) Guardrails() GuardrailPolicy { return c.guardrails }

func (c *clientImpl) close() {
	if c == nil {
		return
	}
	if c.kgoClient != nil {
		c.kgoClient.Close()
	}
	c.cleanupTempFiles()
}

func (c *clientImpl) cleanupTempFiles() {
	if c == nil {
		return
	}
	for _, path := range c.tempPaths {
		_ = os.RemoveAll(path)
	}
	c.tempPaths = nil
}

func (c *clientImpl) Metadata(ctx context.Context, topics ...string) (kadm.Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.Metadata(ctx, topics...)
}

func (c *clientImpl) ApiVersions(ctx context.Context) (kadm.BrokersApiVersions, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ApiVersions(ctx)
}

func (c *clientImpl) ListBrokers(ctx context.Context) (kadm.BrokerDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ListBrokers(ctx)
}

func (c *clientImpl) DescribeBrokerConfigs(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.DescribeBrokerConfigs(ctx, brokers...)
}

func (c *clientImpl) Request(ctx context.Context, req kmsg.Request) (kmsg.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.kgoClient.Request(ctx, req)
}

func (c *clientImpl) ListTopics(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ListTopics(ctx, topics...)
}

func (c *clientImpl) DescribeTopicConfigs(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.DescribeTopicConfigs(ctx, topics...)
}

func (c *clientImpl) DescribeAllLogDirs(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.DescribeAllLogDirs(ctx, nil)
}

func (c *clientImpl) ListGroups(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ListGroups(ctx, states...)
}

func (c *clientImpl) DescribeGroups(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.DescribeGroups(ctx, groups...)
}

func (c *clientImpl) FetchOffsets(ctx context.Context, group string) (kadm.OffsetResponses, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.FetchOffsets(ctx, group)
}

func (c *clientImpl) Lag(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.Lag(ctx, groups...)
}

func (c *clientImpl) FindGroupCoordinators(ctx context.Context, groups ...string) kadm.FindCoordinatorResponses {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()

	const maxRetries = 5
	baseDelay := 200 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(baseDelay):
			}
			baseDelay *= 2
		}

		responses := c.admClient.FindGroupCoordinators(ctx, groups...)
		allOK := true
		for _, resp := range responses {
			if resp.Err != nil {
				var kErr *kerr.Error
				if errors.As(resp.Err, &kErr) && kErr.Retriable {
					allOK = false
					break
				}
			}
		}

		if allOK {
			return responses
		}
	}

	return c.admClient.FindGroupCoordinators(ctx, groups...)
}

func (c *clientImpl) ListEndOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ListEndOffsets(ctx, topics...)
}

func (c *clientImpl) ListStartOffsets(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.admClient.ListStartOffsets(ctx, topics...)
}

func (c *clientImpl) RequestSharded(ctx context.Context, req kmsg.Request) []kgo.ResponseShard {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.MaxQueryDuration)
	defer cancel()
	return c.kgoClient.RequestSharded(ctx, req)
}

func buildTLSConfig(cfg config.KafkaTLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.Insecure}
	if cfg.CAData != "" || cfg.CAFile != "" {
		pemData, err := pemDataFromConfig(cfg.CAData, cfg.CAFile, "ca")
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("invalid ca pem data")
		}
		tlsCfg.RootCAs = pool
	}
	if (cfg.CertData != "" || cfg.CertFile != "") && (cfg.KeyData != "" || cfg.KeyFile != "") {
		certPEM, err := pemDataFromConfig(cfg.CertData, cfg.CertFile, "client cert")
		if err != nil {
			return nil, err
		}
		keyPEM, err := pemDataFromConfig(cfg.KeyData, cfg.KeyFile, "client key")
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func pemDataFromConfig(encodedData, filePath, name string) ([]byte, error) {
	if encodedData != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodedData)
		if err != nil {
			return nil, fmt.Errorf("decode %s data: %w", name, err)
		}
		return decoded, nil
	}
	pemData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", name, err)
	}
	return pemData, nil
}

func buildSASLMechanism(cfg config.KafkaSASLConfig) (sasl.Mechanism, error) {
	return buildSASLMechanismWithTempFiles(cfg, nil)
}

func buildSASLMechanismWithTempFiles(cfg config.KafkaSASLConfig, tempPaths *[]string) (sasl.Mechanism, error) {
	switch cfg.Mechanism {
	case "PLAIN":
		return plain.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsMechanism(), nil
	case "SCRAM-SHA-256":
		return scram.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return scram.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsSha512Mechanism(), nil
	case "GSSAPI":
		return buildGSSAPIMechanism(cfg, tempPaths)
	default:
		return nil, fmt.Errorf("unsupported sasl mechanism: %s", cfg.Mechanism)
	}
}

func buildGSSAPIMechanism(cfg config.KafkaSASLConfig, tempPaths *[]string) (sasl.Mechanism, error) {
	krb5ConfPath := cfg.Krb5ConfPath
	if cfg.Krb5ConfData != "" {
		path, err := writeBase64TempFile(cfg.Krb5ConfData, "krb5.conf", tempPaths)
		if err != nil {
			return nil, fmt.Errorf("write krb5.conf data: %w", err)
		}
		krb5ConfPath = path
	}
	if krb5ConfPath == "" {
		krb5ConfPath = "/etc/krb5.conf"
	}

	keytabPath := cfg.KeytabPath
	if cfg.KeytabData != "" {
		path, err := writeBase64TempFile(cfg.KeytabData, "client.keytab", tempPaths)
		if err != nil {
			return nil, fmt.Errorf("write keytab data: %w", err)
		}
		keytabPath = path
	}
	if keytabPath == "" {
		return nil, fmt.Errorf("keytab_path or keytab_data is required for GSSAPI")
	}
	if cfg.KerberosServiceName == "" {
		return nil, fmt.Errorf("kerberos_service_name is required for GSSAPI")
	}
	if cfg.KerberosRealm == "" {
		return nil, fmt.Errorf("kerberos_realm is required for GSSAPI")
	}

	principal := cfg.Username
	if principal == "" {
		principal = cfg.KerberosServiceName
	}
	if principal == "" {
		return nil, fmt.Errorf("username is required for GSSAPI")
	}
	principal = strings.TrimSuffix(principal, "@"+cfg.KerberosRealm)

	krb5Config, err := krb5config.Load(krb5ConfPath)
	if err != nil {
		return nil, fmt.Errorf("load krb5 config: %w", err)
	}
	kt, err := keytab.Load(keytabPath)
	if err != nil {
		return nil, fmt.Errorf("load keytab: %w", err)
	}
	krbClient := krb5client.NewWithKeytab(principal, cfg.KerberosRealm, kt, krb5Config)

	return franzkerberos.Auth{
		Client:  krbClient,
		Service: cfg.KerberosServiceName,
	}.AsMechanismWithClose(), nil
}

func writeBase64TempFile(encodedData, filename string, tempPaths *[]string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	dir, err := os.MkdirTemp("", "marvin-krb5-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	if tempPaths != nil {
		*tempPaths = append(*tempPaths, dir)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, decoded, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return path, nil
}
