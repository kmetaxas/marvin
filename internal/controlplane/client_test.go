package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	agentmetadata "github.com/marvin-agent/marvin/internal/metadata"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

type stubTaskExecutor struct {
	execute func(context.Context, string, map[string]any) (task.Result, error)
}

func (s stubTaskExecutor) Execute(ctx context.Context, capabilityName string, params map[string]any) (task.Result, error) {
	return s.execute(ctx, capabilityName, params)
}

func TestClientRegisterHeartbeatExecuteAndUpdateConfig(t *testing.T) {
	t.Parallel()

	updateCh := make(chan *marvinpb.UpdateConfig, 1)
	var mock *mockControlPlaneServer
	server, listener, mock := newMockGRPCServer(t, func(stream marvinpb.MarvinService_ConnectServer) error {
		msg, err := mock.recvAgentMessage(stream)
		require.NoError(t, err)

		register := msg.GetRegister()
		require.NotNil(t, register)
		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_Registered{Registered: &marvinpb.Registered{HeartbeatIntervalSeconds: 1}},
		}))

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			CommandId: "cmd-1",
			Payload: &marvinpb.ControlMessage_ExecuteCapability{ExecuteCapability: &marvinpb.ExecuteCapability{
				SessionId:      "session-1",
				ThreadId:       "thread-1",
				CapabilityName: "network.dns.lookup",
				ParametersJson: `{"host":"example.com"}`,
			}},
		}))

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_UpdateConfig{UpdateConfig: &marvinpb.UpdateConfig{
				Capabilities: []*marvinpb.CapabilityManifest{{Name: "network.dns.lookup", Enabled: false}},
			}},
		}))

		for {
			_, err := mock.recvAgentMessage(stream)
			if err != nil {
				return err
			}
			if len(mock.resultsCh) > 0 && len(mock.heartbeatCh) > 0 {
				return nil
			}
		}
	})
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "super-secret-token"},
		WithTaskExecutor(stubTaskExecutor{execute: func(_ context.Context, capabilityName string, params map[string]any) (task.Result, error) {
			assert.Equal(t, "network.dns.lookup", capabilityName)
			assert.Equal(t, map[string]any{"host": "example.com"}, params)
			return task.Result{Success: true, Data: map[string]any{"address": "93.184.216.34"}}, nil
		}}),
		WithAgentID("agent-123"),
		WithLabels([]string{"env:test"}),
		WithUpdateConfigHandler(func(_ context.Context, update *marvinpb.UpdateConfig) {
			updateCh <- update
		}),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	meta := agentmetadata.Metadata{
		Name:        "marvin-us-east-1",
		Cluster:     "prod-us-east-1",
		Region:      "us-east-1",
		Environment: "production",
		Hostname:    "marvin-host",
	}
	capabilities := []capability.Capability{{Name: "network.dns.lookup", Provider: "network", Description: "Lookup DNS", Enabled: true}}

	require.NoError(t, client.Register(ctx, meta, capabilities))

	select {
	case register := <-mock.registerCh:
		assert.Equal(t, "super-secret-token", register.GetRegistrationKey())
		assert.Equal(t, defaultAgentVersion, register.GetAgentVersion())
		assert.Equal(t, "marvin-host", register.GetHost().GetHostname())
		assert.Len(t, register.GetCapabilities(), 1)
		assert.Equal(t, []string{"env:test"}, register.GetLabels())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for register message")
	}

	select {
	case result := <-mock.resultsCh:
		assert.Equal(t, "cmd-1", result.GetCommandId())
		assert.Equal(t, "session-1", result.GetSessionId())
		assert.Equal(t, "thread-1", result.GetThreadId())
		assert.Equal(t, "network.dns.lookup", result.GetCapabilityName())
		assert.True(t, result.GetSuccess())
		assert.JSONEq(t, `{"address":"93.184.216.34"}`, result.GetResultJson())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for capability result")
	}

	select {
	case heartbeat := <-mock.heartbeatCh:
		assert.NotZero(t, heartbeat.GetTimestampUnixMs())
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("timed out waiting for heartbeat")
	}

	select {
	case update := <-updateCh:
		assert.Len(t, update.GetCapabilities(), 1)
		assert.Equal(t, "network.dns.lookup", update.GetCapabilities()[0].GetName())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for update config handler")
	}

	cancel()
	assert.Eventually(t, func() bool {
		mock.mu.Lock()
		defer mock.mu.Unlock()
		return mock.connectCalls >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestClientReconnectsAfterTemporaryRegistrationRejection(t *testing.T) {
	t.Parallel()

	var mock *mockControlPlaneServer
	server, listener, mock := newMockGRPCServer(
		t,
		func(stream marvinpb.MarvinService_ConnectServer) error {
			msg, err := mock.recvAgentMessage(stream)
			require.NoError(t, err)
			require.NotNil(t, msg.GetRegister())
			return mock.sendControlMessage(stream, &marvinpb.ControlMessage{
				Payload: &marvinpb.ControlMessage_RegistrationRejected{RegistrationRejected: &marvinpb.RegistrationRejected{
					Reason:    "please retry",
					Permanent: false,
				}},
			})
		},
		func(stream marvinpb.MarvinService_ConnectServer) error {
			msg, err := mock.recvAgentMessage(stream)
			require.NoError(t, err)
			require.NotNil(t, msg.GetRegister())
			require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
				Payload: &marvinpb.ControlMessage_Registered{Registered: &marvinpb.Registered{HeartbeatIntervalSeconds: 1}},
			}))
			<-stream.Context().Done()
			return stream.Context().Err()
		},
	)
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "retry-token"},
		WithAgentID("agent-retry"),
		WithReconnectBackoff(10*time.Millisecond, 20*time.Millisecond),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, client.Register(ctx, agentmetadata.Metadata{Name: "agent-retry", Cluster: "cluster", Region: "region", Environment: "env"}, nil))

	assert.Eventually(t, func() bool {
		mock.mu.Lock()
		defer mock.mu.Unlock()
		return mock.connectCalls >= 2
	}, 2*time.Second, 10*time.Millisecond)
}

func TestClientPermanentRegistrationRejectionFails(t *testing.T) {
	t.Parallel()

	var mock *mockControlPlaneServer
	server, listener, mock := newMockGRPCServer(t, func(stream marvinpb.MarvinService_ConnectServer) error {
		msg, err := mock.recvAgentMessage(stream)
		require.NoError(t, err)
		require.NotNil(t, msg.GetRegister())
		return mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_RegistrationRejected{RegistrationRejected: &marvinpb.RegistrationRejected{
				Reason:    "invalid key",
				Permanent: true,
			}},
		})
	})
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "bad-token"},
		WithAgentID("agent-bad"),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	err = client.Register(context.Background(), agentmetadata.Metadata{Name: "agent-bad", Cluster: "cluster", Region: "region", Environment: "env"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registration rejected permanently")
	assert.Contains(t, err.Error(), "invalid key")
}

func TestBuildTLSConfig(t *testing.T) {
	t.Parallel()

	certFile := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(certFile, mustCreateControlPlaneTestCACertPEM(t), 0o600))

	tlsCfg, err := BuildTLSConfig(config.TLSConfig{CACertFile: certFile})
	require.NoError(t, err)
	assert.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.RootCAs)
}

func TestSecretKeyAuthenticatorValidation(t *testing.T) {
	t.Parallel()

	_, err := SecretKeyAuthenticator{}.RegistrationKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret key is empty")
}

func TestClientAppliesEffectiveConfigOnRegistration(t *testing.T) {
	t.Parallel()

	var mock *mockControlPlaneServer
	updateCh := make(chan *marvinpb.UpdateConfig, 1)
	server, listener, mock := newMockGRPCServer(t, func(stream marvinpb.MarvinService_ConnectServer) error {
		msg, err := mock.recvAgentMessage(stream)
		require.NoError(t, err)
		require.NotNil(t, msg.GetRegister())

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_Registered{Registered: &marvinpb.Registered{
				HeartbeatIntervalSeconds: 1,
				EffectiveConfig: &marvinpb.UpdateConfig{
					ConfigVersion: 1,
					Capabilities: []*marvinpb.CapabilityManifest{
						{Name: "network.dns.lookup", Config: newStruct(t, map[string]any{"ttl": 60})},
					},
				},
			}},
		}))

		<-stream.Context().Done()
		return stream.Context().Err()
	})
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "test-key"},
		WithAgentID("agent-eff"),
		WithUpdateConfigHandler(func(_ context.Context, update *marvinpb.UpdateConfig) {
			updateCh <- update
		}),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, client.Register(ctx, agentmetadata.Metadata{Name: "agent-eff", Cluster: "c", Region: "r", Environment: "e"}, nil))

	select {
	case update := <-updateCh:
		assert.Equal(t, int64(1), update.GetConfigVersion())
		assert.Len(t, update.GetCapabilities(), 1)
		assert.Equal(t, "network.dns.lookup", update.GetCapabilities()[0].GetName())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for effective config handler")
	}
}

func TestClientVersionGatingIgnoresStaleUpdateConfig(t *testing.T) {
	t.Parallel()

	var mock *mockControlPlaneServer
	updateCh := make(chan *marvinpb.UpdateConfig, 2)
	server, listener, mock := newMockGRPCServer(t, func(stream marvinpb.MarvinService_ConnectServer) error {
		msg, err := mock.recvAgentMessage(stream)
		require.NoError(t, err)
		require.NotNil(t, msg.GetRegister())

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_Registered{Registered: &marvinpb.Registered{
				HeartbeatIntervalSeconds: 1,
				EffectiveConfig: &marvinpb.UpdateConfig{
					ConfigVersion: 10,
					Capabilities: []*marvinpb.CapabilityManifest{
						{Name: "network.dns.lookup", Config: newStruct(t, map[string]any{"ttl": 60})},
					},
				},
			}},
		}))

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_UpdateConfig{UpdateConfig: &marvinpb.UpdateConfig{
				ConfigVersion: 5,
				Capabilities: []*marvinpb.CapabilityManifest{
					{Name: "network.dns.lookup", Config: newStruct(t, map[string]any{"ttl": 999})},
				},
			}},
		}))

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_UpdateConfig{UpdateConfig: &marvinpb.UpdateConfig{
				ConfigVersion: 15,
				Capabilities: []*marvinpb.CapabilityManifest{
					{Name: "network.dns.lookup", Config: newStruct(t, map[string]any{"ttl": 120})},
				},
			}},
		}))

		<-stream.Context().Done()
		return stream.Context().Err()
	})
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "test-key"},
		WithAgentID("agent-ver"),
		WithUpdateConfigHandler(func(_ context.Context, update *marvinpb.UpdateConfig) {
			updateCh <- update
		}),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, client.Register(ctx, agentmetadata.Metadata{Name: "agent-ver", Cluster: "c", Region: "r", Environment: "e"}, nil))

	var received []*marvinpb.UpdateConfig
	for i := 0; i < 2; i++ {
		select {
		case update := <-updateCh:
			received = append(received, update)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for update config %d", i)
		}
	}

	assert.Equal(t, int64(10), received[0].GetConfigVersion())
	assert.Equal(t, int64(15), received[1].GetConfigVersion())

	select {
	case <-updateCh:
		t.Fatal("expected no more updates (stale version should be dropped)")
	case <-time.After(500 * time.Millisecond):
	}
}

func newStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	require.NoError(t, err)
	return s
}

func TestClientHasNoKeepaliveParamsField(t *testing.T) {
	t.Parallel()

	ty := reflect.TypeOf(Client{})
	for i := range ty.NumField() {
		name := ty.Field(i).Name
		if strings.EqualFold(name, "keepaliveParams") {
			t.Fatalf("Client struct must not contain a keepaliveParams field, found: %s", name)
		}
	}
}

func boolPtr(v bool) *bool { return &v }

func stubBufDialer(t *testing.T, listener *bufconn.Listener) func() {
	t.Helper()

	grpcNewClientMu.Lock()
	originalNewClient := grpcNewClient
	grpcNewClient = func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}))
		return grpc.NewClient("passthrough:///bufnet", opts...)
	}
	grpcNewClientMu.Unlock()

	return func() {
		grpcNewClientMu.Lock()
		grpcNewClient = originalNewClient
		grpcNewClientMu.Unlock()
	}
}

func TestClientUpdateConfigHandlerDoesNotDeadlock(t *testing.T) {
	t.Parallel()

	updateCh := make(chan struct{}, 1)
	var mock *mockControlPlaneServer
	server, listener, mock := newMockGRPCServer(t, func(stream marvinpb.MarvinService_ConnectServer) error {
		msg, err := mock.recvAgentMessage(stream)
		require.NoError(t, err)
		require.NotNil(t, msg.GetRegister())

		require.NoError(t, mock.sendControlMessage(stream, &marvinpb.ControlMessage{
			Payload: &marvinpb.ControlMessage_Registered{Registered: &marvinpb.Registered{
				HeartbeatIntervalSeconds: 1,
				EffectiveConfig: &marvinpb.UpdateConfig{
					ConfigVersion: 1,
					Capabilities: []*marvinpb.CapabilityManifest{
						{Name: "network.dns.lookup", Config: newStruct(t, map[string]any{"ttl": 60})},
					},
				},
			}},
		}))

		_, err = mock.recvAgentMessage(stream)
		if err != nil {
			return err
		}

		return nil
	})
	defer server.Stop()

	restore := stubBufDialer(t, listener)
	defer restore()

	var testClient *Client
	client, err := NewClient(
		config.ControlPlane{Address: "bufnet", TLS: config.TLSConfig{Enabled: boolPtr(false)}},
		SecretKeyAuthenticator{Key: "test-key"},
		WithAgentID("agent-deadlock"),
		WithUpdateConfigHandler(func(_ context.Context, update *marvinpb.UpdateConfig) {
			testClient.UpdateCapabilities(nil)
			close(updateCh)
		}),
	)
	testClient = client
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, client.Register(ctx, agentmetadata.Metadata{Name: "agent-deadlock", Cluster: "c", Region: "r", Environment: "e"}, nil))

	select {
	case <-updateCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UpdateConfig handler — deadlock suspected")
	}

	select {
	case hb := <-mock.heartbeatCh:
		assert.NotZero(t, hb.GetTimestampUnixMs())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for heartbeat after UpdateConfig")
	}
}

func mustCreateControlPlaneTestCACertPEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "marvin-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}
