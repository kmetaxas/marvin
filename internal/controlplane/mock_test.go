package controlplane

import (
	"sync"
	"testing"

	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type connectScript func(marvinpb.MarvinService_ConnectServer) error

type mockControlPlaneServer struct {
	marvinpb.UnimplementedMarvinServiceServer

	t *testing.T

	mu           sync.Mutex
	connectCalls int
	scripts      []connectScript
	registers    []*marvinpb.AgentMessage
	heartbeats   []*marvinpb.Heartbeat
	results      []*marvinpb.CapabilityResult
	updates      []*marvinpb.UpdateConfig
	disconnects  []*marvinpb.Disconnect
	commandsSent []*marvinpb.ControlMessage
	resultsCh    chan *marvinpb.CapabilityResult
	heartbeatCh  chan *marvinpb.Heartbeat
	registerCh   chan *marvinpb.Register
	connectCh    chan int
}

func newMockGRPCServer(t *testing.T, scripts ...connectScript) (*grpc.Server, *bufconn.Listener, *mockControlPlaneServer) {
	t.Helper()

	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	mock := &mockControlPlaneServer{
		t:           t,
		scripts:     scripts,
		resultsCh:   make(chan *marvinpb.CapabilityResult, 8),
		heartbeatCh: make(chan *marvinpb.Heartbeat, 8),
		registerCh:  make(chan *marvinpb.Register, 8),
		connectCh:   make(chan int, 8),
	}

	marvinpb.RegisterMarvinServiceServer(server, mock)

	go func() {
		_ = server.Serve(listener)
	}()

	return server, listener, mock
}

func (m *mockControlPlaneServer) Connect(stream marvinpb.MarvinService_ConnectServer) error {
	m.mu.Lock()
	callIndex := m.connectCalls
	m.connectCalls++
	m.mu.Unlock()

	select {
	case m.connectCh <- callIndex + 1:
	default:
	}

	if callIndex < len(m.scripts) {
		return m.scripts[callIndex](stream)
	}

	<-stream.Context().Done()
	return stream.Context().Err()
}

func (m *mockControlPlaneServer) recvAgentMessage(stream marvinpb.MarvinService_ConnectServer) (*marvinpb.AgentMessage, error) {
	msg, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if register := msg.GetRegister(); register != nil {
		m.registers = append(m.registers, msg)
		select {
		case m.registerCh <- register:
		default:
		}
	}
	if heartbeat := msg.GetHeartbeat(); heartbeat != nil {
		m.heartbeats = append(m.heartbeats, heartbeat)
		select {
		case m.heartbeatCh <- heartbeat:
		default:
		}
	}
	if result := msg.GetCapabilityResult(); result != nil {
		m.results = append(m.results, result)
		select {
		case m.resultsCh <- result:
		default:
		}
	}

	return msg, nil
}

func (m *mockControlPlaneServer) sendControlMessage(stream marvinpb.MarvinService_ConnectServer, msg *marvinpb.ControlMessage) error {
	m.mu.Lock()
	m.commandsSent = append(m.commandsSent, msg)
	if update := msg.GetUpdateConfig(); update != nil {
		m.updates = append(m.updates, update)
	}
	if disconnect := msg.GetDisconnect(); disconnect != nil {
		m.disconnects = append(m.disconnects, disconnect)
	}
	m.mu.Unlock()

	return stream.Send(msg)
}
