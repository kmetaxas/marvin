package kafka

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/kafka/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// newAdminClient builds a kadm admin client (backed by franz-go) pointed at the
// given bootstrap servers. It is used to exercise the same Kafka operations
// that the kafka provider's tasks will perform.
func newAdminClient(t *testing.T, bootstrapServers []string, opts ...kgo.Opt) *kadm.Client {
	t.Helper()

	allOpts := append([]kgo.Opt{
		kgo.SeedBrokers(bootstrapServers...),
		kgo.ClientID("marvin-kafka-integration-test"),
		kgo.RequestTimeoutOverhead(10 * time.Second),
	}, opts...)

	cl, err := kadm.NewOptClient(allOpts...)
	require.NoError(t, err, "failed to create kafka admin client")
	t.Cleanup(cl.Close)

	return cl
}

// TestIntegration_Plaintext starts a plaintext Kafka broker and verifies the
// core cluster/topic/broker operations that the kafka provider exposes.
func TestIntegration_Plaintext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	bootstrapServers, _ := testutil.StartKafkaPlaintext(t)
	cl := newAdminClient(t, bootstrapServers)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("cluster.describe", func(t *testing.T) {
		meta, err := cl.Metadata(ctx)
		require.NoError(t, err, "metadata request should succeed")
		assert.NotEmpty(t, meta.Cluster, "cluster id should be present")
		assert.NotEmpty(t, meta.Brokers, "at least one broker should be present")
	})

	t.Run("broker.list", func(t *testing.T) {
		brokers, err := cl.ListBrokers(ctx)
		require.NoError(t, err, "list brokers should succeed")
		assert.NotEmpty(t, brokers, "at least one broker should be listed")
		for _, b := range brokers {
			assert.NotZero(t, b.NodeID, "broker node id should be non-zero")
			assert.NotEmpty(t, b.Host, "broker host should be non-empty")
		}
	})

	t.Run("topic.list", func(t *testing.T) {
		topics, err := cl.ListTopics(ctx)
		require.NoError(t, err, "list topics should succeed")
		// A fresh broker has no user topics, but the request must succeed.
		assert.NotNil(t, topics, "topic list should be non-nil")
	})

	t.Run("topic.describe", func(t *testing.T) {
		// Create a topic first so there is something to describe.
		resp, err := cl.CreateTopic(ctx, 1, 1, nil, "marvin-test-topic")
		require.NoError(t, err, "create topic should succeed")
		require.NoError(t, resp.Err, "create topic should not return an error")

		topics, err := cl.ListTopics(ctx, "marvin-test-topic")
		require.NoError(t, err, "describe topic should succeed")
		require.True(t, topics.Has("marvin-test-topic"), "topic should exist")
		detail := topics["marvin-test-topic"]
		assert.Equal(t, "marvin-test-topic", detail.Topic)
		assert.NotEmpty(t, detail.Partitions, "topic should have partitions")
	})
}

// TestIntegration_SCRAM starts a SCRAM-SHA-256 secured Kafka broker and
// verifies that a client can authenticate and perform operations.
func TestIntegration_SCRAM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	bootstrapServers, _, scramConfig := testutil.StartKafkaWithSCRAM(t)

	cl := newAdminClient(t, bootstrapServers,
		kgo.SASL(scram.Auth{
			User: scramConfig.Username,
			Pass: scramConfig.Password,
		}.AsSha256Mechanism()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("authenticated.metadata", func(t *testing.T) {
		meta, err := cl.Metadata(ctx)
		require.NoError(t, err, "metadata request with SCRAM should succeed")
		assert.NotEmpty(t, meta.Cluster, "cluster id should be present")
	})

	t.Run("authenticated.topic.create", func(t *testing.T) {
		resp, err := cl.CreateTopic(ctx, 1, 1, nil, "marvin-scram-topic")
		require.NoError(t, err, "create topic should succeed")
		require.NoError(t, resp.Err, "create topic should not return an error")
	})
}

// TestIntegration_WithSchemaRegistry starts a Kafka broker and a Schema
// Registry, then verifies the Schema Registry status, subject registration,
// and schema lookup operations.
func TestIntegration_WithSchemaRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	_, srURL, _ := testutil.StartKafkaWithSchemaRegistry(t)

	httpClient := &http.Client{Timeout: 15 * time.Second}

	t.Run("schema_registry.status.get", func(t *testing.T) {
		resp, err := httpClient.Get(srURL + "/")
		require.NoError(t, err, "schema registry status request should succeed")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "schema registry should return 200")
	})

	t.Run("schema_registry.schema.register", func(t *testing.T) {
		schema := `{"type":"record","name":"MarvinTest","fields":[{"name":"id","type":"int"}]}`
		body, err := json.Marshal(map[string]string{"schema": schema})
		require.NoError(t, err)

		resp, err := httpClient.Post(
			srURL+"/subjects/marvin-test-value/versions",
			"application/vnd.schemaregistry.v1+json",
			bytes.NewReader(body),
		)
		require.NoError(t, err, "register schema should succeed")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "register schema should return 200")

		var regResp struct {
			ID int `json:"id"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&regResp))
		assert.Positive(t, regResp.ID, "schema id should be positive")

		t.Run("schema_registry.schema.get_by_id", func(t *testing.T) {
			resp, err := httpClient.Get(fmt.Sprintf("%s/schemas/ids/%d", srURL, regResp.ID))
			require.NoError(t, err, "get schema by id should succeed")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "get schema by id should return 200")

			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var schemaResp struct {
				Schema string `json:"schema"`
			}
			require.NoError(t, json.Unmarshal(data, &schemaResp))
			assert.Contains(t, schemaResp.Schema, "MarvinTest", "schema should contain the registered record name")
		})
	})
}

// TestIntegration_MTLS starts an mTLS-secured Kafka broker and verifies that a
// client presenting a valid client certificate can connect.
func TestIntegration_MTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	bootstrapServers, _, _, certPaths := testutil.StartKafkaWithMTLS(t)

	caPEM, err := os.ReadFile(certPaths.CAPath)
	require.NoError(t, err, "failed to read CA cert")
	clientCert, err := tls.LoadX509KeyPair(certPaths.CertPath, certPaths.KeyPath)
	require.NoError(t, err, "failed to load client cert/key")

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(caPEM), "failed to append CA cert to pool")

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      pool,
		ServerName:   "localhost",
		MinVersion:   tls.VersionTLS12,
	}

	cl := newAdminClient(t, bootstrapServers, kgo.DialTLSConfig(tlsConfig))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("mtls.metadata", func(t *testing.T) {
		meta, err := cl.Metadata(ctx)
		require.NoError(t, err, "metadata request with mTLS should succeed")
		assert.NotEmpty(t, meta.Cluster, "cluster id should be present")
	})
}
