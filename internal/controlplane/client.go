package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	agentmetadata "github.com/marvin-agent/marvin/internal/metadata"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	defaultAgentVersion    = "marvin-agent/v1.0.0"
	defaultHeartbeatPeriod = 30 * time.Second
	defaultBackoffBase     = time.Second
	defaultBackoffMax      = 30 * time.Second
)

var grpcNewClientMu sync.RWMutex
var grpcNewClient = grpc.NewClient

type TaskExecutor interface {
	Execute(ctx context.Context, capabilityName string, params map[string]any) (task.Result, error)
}

type UpdateConfigHandler func(context.Context, *marvinpb.UpdateConfig)

type DisconnectHandler func(context.Context, *marvinpb.Disconnect)

type ClientOption func(*Client)

type Client struct {
	conn    *grpc.ClientConn
	service marvinpb.MarvinServiceClient
	auth    Authenticator

	executor              TaskExecutor
	updateConfigHandler   UpdateConfigHandler
	disconnectHandler     DisconnectHandler
	configuredAgentID     string
	configuredLabels      []string
	configuredRegisterKey string
	configuredNonce       string
	backoffBase           time.Duration
	backoffMax            time.Duration
	logger                *slog.Logger

	sendMu   sync.Mutex
	stateMu  sync.RWMutex
	stream   grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage]
	runDone  chan struct{}
	cancel   context.CancelFunc
	started  bool
	closed   bool
	regState registrationState

	lastAppliedVersion int64

	inFlight  sync.WaitGroup
	closeOnce sync.Once

	heartbeatAckMu   sync.Mutex
	lastHeartbeatAck time.Time
}

type registrationState struct {
	agentID         string
	metadata        agentmetadata.Metadata
	capabilities    []*marvinpb.CapabilityManifest
	labels          []string
	registrationKey string
	nonce           string
}

type permanentRegistrationError struct {
	reason string
}

func (e *permanentRegistrationError) Error() string {
	return fmt.Sprintf("registration rejected permanently: %s", e.reason)
}

type temporaryRegistrationError struct {
	reason string
}

func (e *temporaryRegistrationError) Error() string {
	return fmt.Sprintf("registration rejected temporarily: %s", e.reason)
}

func WithTaskExecutor(executor TaskExecutor) ClientOption {
	return func(c *Client) {
		c.executor = executor
	}
}

func WithAgentID(agentID string) ClientOption {
	return func(c *Client) {
		c.configuredAgentID = agentID
	}
}

func WithLabels(labels []string) ClientOption {
	return func(c *Client) {
		c.configuredLabels = append([]string(nil), labels...)
	}
}

func WithRegistrationKey(key string) ClientOption {
	return func(c *Client) {
		c.configuredRegisterKey = key
	}
}

func WithRegistrationNonce(nonce string) ClientOption {
	return func(c *Client) {
		c.configuredNonce = nonce
	}
}

func WithUpdateConfigHandler(handler UpdateConfigHandler) ClientOption {
	return func(c *Client) {
		c.updateConfigHandler = handler
	}
}

func WithDisconnectHandler(handler DisconnectHandler) ClientOption {
	return func(c *Client) {
		c.disconnectHandler = handler
	}
}

func WithReconnectBackoff(base, max time.Duration) ClientOption {
	return func(c *Client) {
		if base > 0 {
			c.backoffBase = base
		}
		if max > 0 {
			c.backoffMax = max
		}
	}
}

func (c *Client) log() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}
	return slog.Default()
}

func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

func NewClient(cfg config.ControlPlane, auth Authenticator, opts ...ClientOption) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("control plane address is required")
	}

	transportCreds, err := transportCredentials(cfg)
	if err != nil {
		return nil, err
	}

	client := &Client{
		auth:        auth,
		backoffBase: defaultBackoffBase,
		backoffMax:  defaultBackoffMax,
	}
	for _, opt := range opts {
		opt(client)
	}

	if client.backoffMax < client.backoffBase {
		client.backoffMax = client.backoffBase
	}

	if client.configuredRegisterKey == "" && auth != nil {
		registrationKey, err := auth.RegistrationKey()
		if err != nil {
			return nil, fmt.Errorf("resolve registration key: %w", err)
		}
		client.configuredRegisterKey = registrationKey
	}

	grpcNewClientMu.RLock()
	conn, err := grpcNewClient(
		cfg.Address,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	grpcNewClientMu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("dial control plane %q: %w", cfg.Address, err)
	}

	client.conn = conn
	client.service = marvinpb.NewMarvinServiceClient(conn)
	client.log().Debug("gRPC client connection initialized", "address", cfg.Address)

	return client, nil
}

func (c *Client) Register(ctx context.Context, metadata agentmetadata.Metadata, capabilities []capability.Capability) error {
	if c == nil {
		return fmt.Errorf("client is nil")
	}

	regState, err := c.buildRegistrationState(metadata, capabilities)
	if err != nil {
		return err
	}

	firstRegistration := make(chan error, 1)
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	runDone := make(chan struct{})

	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		cancel()
		close(runDone)
		return fmt.Errorf("client is closed")
	}
	if c.started {
		c.stateMu.Unlock()
		cancel()
		close(runDone)
		return fmt.Errorf("client already registered")
	}
	c.regState = regState
	c.cancel = cancel
	c.runDone = runDone
	c.started = true
	c.stateMu.Unlock()

	go c.run(runCtx, firstRegistration, runDone)
	c.log().Debug("started background registration goroutine")

	select {
	case err := <-firstRegistration:
		if err != nil {
			_ = c.Close()
			c.log().Error("initial registration failed", "error", err)
			return err
		}
		c.log().Debug("registration succeeded")
		return nil
	case <-ctx.Done():
		_ = c.Close()
		c.log().Debug("registration interrupted by context", "error", ctx.Err())
		return ctx.Err()
	}
}

func (c *Client) SendHealthPing(ctx context.Context) error {
	stream := c.currentStream()
	if stream == nil {
		return fmt.Errorf("connect stream is not established")
	}

	return c.safeSend(ctx, stream, c.newHeartbeatMessage())
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	var closeErr error
	c.closeOnce.Do(func() {
		c.stateMu.Lock()
		c.closed = true
		cancel := c.cancel
		runDone := c.runDone
		stream := c.stream
		conn := c.conn
		c.conn = nil
		c.stateMu.Unlock()

		if cancel != nil {
			cancel()
		}
		if stream != nil {
			c.sendMu.Lock()
			_ = stream.CloseSend()
			c.sendMu.Unlock()
		}
		if runDone != nil {
			<-runDone
		}
		if conn != nil {
			closeErr = conn.Close()
		}
	})

	return closeErr
}

func (c *Client) run(ctx context.Context, firstRegistration chan<- error, runDone chan struct{}) {
	defer close(runDone)

	firstReported := false
	attempt := 0
	reportFirst := func(err error) {
		if firstReported {
			return
		}
		firstReported = true
		firstRegistration <- err
	}

	for {
		if ctx.Err() != nil {
			reportFirst(ctx.Err())
			return
		}

		c.log().Debug("attempting control plane connection", "attempt", attempt)
		registered, err := c.connectOnce(ctx, reportFirst)
		if registered {
			attempt = 0
		}

		if err == nil {
			if ctx.Err() != nil {
				return
			}
		} else {
			var permanentErr *permanentRegistrationError
			if errors.As(err, &permanentErr) {
				reportFirst(err)
				return
			}
			c.log().Debug("connection lost, will retry", "error", err, "attempt", attempt)
		}

		if ctx.Err() != nil {
			return
		}

		delay := c.nextBackoff(attempt)
		attempt++
		c.log().Debug("scheduling reconnect", "delay", delay.String())

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (c *Client) connectOnce(ctx context.Context, reportFirst func(error)) (bool, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.service.Connect(streamCtx)
	if err != nil {
		return false, fmt.Errorf("open connect stream: %w", err)
	}

	if err := c.safeSend(ctx, stream, c.newRegisterMessage()); err != nil {
		_ = stream.CloseSend()
		return false, fmt.Errorf("send register message: %w", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		_ = stream.CloseSend()
		return false, fmt.Errorf("wait for registration response: %w", err)
	}

	if rejected := msg.GetRegistrationRejected(); rejected != nil {
		_ = stream.CloseSend()
		if rejected.GetPermanent() {
			return false, &permanentRegistrationError{reason: rejected.GetReason()}
		}
		return false, &temporaryRegistrationError{reason: rejected.GetReason()}
	}

	registered := msg.GetRegistered()
	if registered == nil {
		_ = stream.CloseSend()
		return false, fmt.Errorf("expected registration acknowledgement, received %T", msg.GetPayload())
	}
	reportFirst(nil)

	if eff := registered.GetEffectiveConfig(); eff != nil {
		c.applyUpdateConfig(eff)
	}

	c.heartbeatAckMu.Lock()
	c.lastHeartbeatAck = time.Now()
	c.heartbeatAckMu.Unlock()

	c.setStream(stream)
	defer func() {
		c.clearStream(stream)
	}()

	heartbeatErr := make(chan error, 1)
	go func() {
		heartbeatErr <- c.heartbeatLoop(streamCtx, stream, registered.GetHeartbeatIntervalSeconds())
	}()

	recvErr := c.recvLoop(streamCtx, stream)
	cancel()
	hbErr := <-heartbeatErr

	if recvErr != nil && !errors.Is(recvErr, context.Canceled) {
		return true, recvErr
	}
	if hbErr != nil && !errors.Is(hbErr, context.Canceled) {
		return true, hbErr
	}

	return true, nil
}

func (c *Client) recvLoop(ctx context.Context, stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage]) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("receive control message: %w", err)
		}

		switch payload := msg.GetPayload().(type) {
		case *marvinpb.ControlMessage_ExecuteCapability:
			c.dispatchCapabilityExecution(ctx, stream, msg.GetCommandId(), payload.ExecuteCapability)
		case *marvinpb.ControlMessage_UpdateConfig:
			c.applyUpdateConfig(payload.UpdateConfig)
		case *marvinpb.ControlMessage_Disconnect:
			if c.disconnectHandler != nil {
				c.disconnectHandler(ctx, payload.Disconnect)
			}
			if payload.Disconnect.GetAllowGraceful() {
				c.inFlight.Wait()
			}
			return nil
		case *marvinpb.ControlMessage_Registered:
			continue
		case *marvinpb.ControlMessage_RegistrationRejected:
			if payload.RegistrationRejected.GetPermanent() {
				return &permanentRegistrationError{reason: payload.RegistrationRejected.GetReason()}
			}
			return &temporaryRegistrationError{reason: payload.RegistrationRejected.GetReason()}
		case *marvinpb.ControlMessage_HeartbeatAck:
			c.log().Debug("received heartbeat ack", "received_timestamp_ms", payload.HeartbeatAck.GetReceivedTimestampUnixMs())
			c.heartbeatAckMu.Lock()
			c.lastHeartbeatAck = time.Now()
			c.heartbeatAckMu.Unlock()
			continue
		default:
			return fmt.Errorf("received unsupported control message payload %T", msg.GetPayload())
		}
	}
}

func (c *Client) heartbeatLoop(ctx context.Context, stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage], intervalSeconds int32) error {
	interval := defaultHeartbeatPeriod
	if intervalSeconds > 0 {
		interval = time.Duration(intervalSeconds) * time.Second
	}

	// Send an immediate heartbeat so the server has activity to ACK.
	if err := c.safeSend(ctx, stream, c.newHeartbeatMessage()); err != nil {
		return fmt.Errorf("send initial heartbeat: %w", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			// Verify the server ACKed the previous heartbeat.
			c.heartbeatAckMu.Lock()
			lastAck := c.lastHeartbeatAck
			c.heartbeatAckMu.Unlock()
			if !lastAck.IsZero() && time.Since(lastAck) > interval*2+5*time.Second {
				return fmt.Errorf("heartbeat ack missing for %s", time.Since(lastAck).Round(time.Second))
			}
			if err := c.safeSend(ctx, stream, c.newHeartbeatMessage()); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}
		}
	}
}

func (c *Client) dispatchCapabilityExecution(ctx context.Context, stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage], commandID string, req *marvinpb.ExecuteCapability) {
	if req == nil {
		return
	}

	c.inFlight.Go(func() {
		result := c.executeCapability(ctx, commandID, req)
		if err := c.safeSend(ctx, stream, &marvinpb.AgentMessage{
			AgentId: c.regState.agentID,
			Payload: &marvinpb.AgentMessage_CapabilityResult{CapabilityResult: result},
		}); err != nil {
			return
		}
	})
}

func (c *Client) executeCapability(ctx context.Context, commandID string, req *marvinpb.ExecuteCapability) *marvinpb.CapabilityResult {
	startedAt := time.Now()
	capabilityName := req.GetCapabilityName()
	result := &marvinpb.CapabilityResult{
		CommandId:      commandID,
		SessionId:      req.GetSessionId(),
		ThreadId:       req.GetThreadId(),
		CapabilityName: capabilityName,
	}

	c.log().Info("control plane received capability execution request", "capability", capabilityName, "command_id", commandID, "session_id", req.GetSessionId())
	c.log().Debug("capability execution request details", "capability", capabilityName, "command_id", commandID, "parameters_json", req.GetParametersJson())

	params := map[string]any{}
	if req.GetParametersJson() != "" {
		if err := json.Unmarshal([]byte(req.GetParametersJson()), &params); err != nil {
			c.log().Info("capability parameter decode failed", "capability", capabilityName, "error", err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("decode parameters_json: %v", err)
			result.ExecutionDuration = durationpb.New(time.Since(startedAt))
			return result
		}
		c.log().Debug("capability parameters decoded", "capability", capabilityName, "params", params)
	}

	if c.executor == nil {
		c.log().Info("capability execution failed: no executor configured", "capability", capabilityName)
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("capability %q is not executable: no task executor configured", capabilityName)
		result.ExecutionDuration = durationpb.New(time.Since(startedAt))
		return result
	}

	execCtx := ctx
	var cancel context.CancelFunc
	if deadlineUnixMs := req.GetDeadlineUnixMs(); deadlineUnixMs > 0 {
		c.log().Debug("capability deadline configured", "capability", capabilityName, "deadline_ms", deadlineUnixMs)
		execCtx, cancel = context.WithDeadline(ctx, time.UnixMilli(deadlineUnixMs))
		defer cancel()
	}

	c.log().Debug("capability execution starting", "capability", capabilityName)
	execResult, err := c.executor.Execute(execCtx, capabilityName, params)
	c.log().Debug("capability execution completed", "capability", capabilityName, "duration_ms", time.Since(startedAt).Milliseconds())

	result.Success = err == nil && execResult.Success
	result.ErrorMessage = execResult.Error
	if err != nil {
		result.ErrorMessage = err.Error()
	}

	if execResult.Data != nil {
		encoded, marshalErr := json.Marshal(execResult.Data)
		if marshalErr != nil {
			c.log().Info("capability result marshalling failed", "capability", capabilityName, "error", marshalErr)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("marshal task result: %v", marshalErr)
		} else {
			result.ResultJson = string(encoded)
			c.log().Debug("capability result marshalled", "capability", capabilityName, "result_json", string(encoded))
		}
	}

	if !result.Success {
		c.log().Info("capability execution failed", "capability", capabilityName, "command_id", commandID, "error", result.ErrorMessage)
	} else {
		c.log().Info("capability execution succeeded", "capability", capabilityName, "command_id", commandID)
	}

	result.ExecutionDuration = durationpb.New(time.Since(startedAt))
	return result
}

func (c *Client) buildRegistrationState(metadata agentmetadata.Metadata, capabilities []capability.Capability) (registrationState, error) {
	manifests := make([]*marvinpb.CapabilityManifest, 0, len(capabilities))
	for _, capability := range capabilities {
		manifests = append(manifests, capability.ToProto())
	}

	agentID := c.configuredAgentID
	if agentID == "" {
		agentID = metadata.Name
	}
	if agentID == "" {
		return registrationState{}, fmt.Errorf("agent id is required")
	}

	registrationKey := c.configuredRegisterKey
	if registrationKey == "" {
		return registrationState{}, fmt.Errorf("registration key is required")
	}

	labels := append([]string(nil), c.configuredLabels...)
	if len(labels) == 0 {
		labels = metadata.ToLabels()
	}

	return registrationState{
		agentID:         agentID,
		metadata:        metadata,
		capabilities:    manifests,
		labels:          labels,
		registrationKey: registrationKey,
		nonce:           c.configuredNonce,
	}, nil
}

func (c *Client) UpdateCapabilities(capabilities []capability.Capability) {
	manifests := make([]*marvinpb.CapabilityManifest, 0, len(capabilities))
	for _, capability := range capabilities {
		manifests = append(manifests, capability.ToProto())
	}

	c.stateMu.Lock()
	c.regState.capabilities = manifests
	stream := c.stream
	c.stateMu.Unlock()

	c.log().Info("capabilities updated, forcing reconnect to re-register")
	if stream != nil {
		_ = stream.CloseSend()
	}
}

func (c *Client) newRegisterMessage() *marvinpb.AgentMessage {
	regState := c.regState
	return &marvinpb.AgentMessage{
		AgentId: regState.agentID,
		Payload: &marvinpb.AgentMessage_Register{Register: &marvinpb.Register{
			AgentVersion:      defaultAgentVersion,
			Host:              regState.metadata.ToProto(),
			Capabilities:      regState.capabilities,
			Labels:            regState.labels,
			RegistrationNonce: regState.nonce,
			RegistrationKey:   regState.registrationKey,
		}},
	}
}

func (c *Client) newHeartbeatMessage() *marvinpb.AgentMessage {
	return &marvinpb.AgentMessage{
		AgentId: c.regState.agentID,
		Payload: &marvinpb.AgentMessage_Heartbeat{Heartbeat: &marvinpb.Heartbeat{
			TimestampUnixMs: time.Now().UTC().UnixMilli(),
		}},
	}
}

func (c *Client) safeSend(ctx context.Context, stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage], msg *marvinpb.AgentMessage) error {
	if stream == nil {
		return fmt.Errorf("connect stream is nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if err := stream.Send(msg); err != nil {
		return err
	}

	return nil
}

func (c *Client) nextBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := c.backoffBase
	for i := 0; i < attempt; i++ {
		if delay >= c.backoffMax/2 {
			delay = c.backoffMax
			break
		}
		delay *= 2
	}
	if delay <= 0 {
		delay = c.backoffBase
	}
	if delay > c.backoffMax {
		delay = c.backoffMax
	}

	maxJitter := int64(delay / 2)
	if maxJitter <= 0 {
		return delay
	}

	jitter := time.Duration(rand.Int63n(int64(math.Max(1, float64(maxJitter)))))
	return delay + jitter
}

func (c *Client) applyUpdateConfig(upd *marvinpb.UpdateConfig) {
	if upd == nil {
		return
	}

	c.stateMu.Lock()
	version := upd.GetConfigVersion()
	if version > 0 && version <= c.lastAppliedVersion {
		c.stateMu.Unlock()
		c.log().Debug("ignoring stale UpdateConfig", "version", version, "last_applied", c.lastAppliedVersion)
		return
	}
	if version > 0 {
		c.lastAppliedVersion = version
	}
	c.stateMu.Unlock()

	caps := upd.GetCapabilities()
	c.log().Info("received config update from control plane", "version", version, "capabilities", len(caps))
	for _, cap := range caps {
		c.log().Debug("config update capability", "name", cap.GetName(), "enabled", cap.GetEnabled(), "resource_ids", cap.GetResourceIds())
		if cfg := cap.GetConfig(); cfg != nil {
			c.log().Debug("config update capability config", "name", cap.GetName(), "config", cfg.AsMap())
		}
	}
	if c.updateConfigHandler != nil {
		c.updateConfigHandler(context.Background(), upd)
	}
}

func (c *Client) currentStream() grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage] {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.stream
}

func (c *Client) setStream(stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage]) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.stream = stream
}

func (c *Client) clearStream(stream grpc.BidiStreamingClient[marvinpb.AgentMessage, marvinpb.ControlMessage]) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stream == stream {
		c.stream = nil
	}
}

func transportCredentials(cfg config.ControlPlane) (credentials.TransportCredentials, error) {
	if !cfg.TLS.IsEnabled() {
		return insecure.NewCredentials(), nil
	}

	tlsCfg, err := BuildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("build tls config: %w", err)
	}

	return credentials.NewTLS(tlsCfg), nil
}
