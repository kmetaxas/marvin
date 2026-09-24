package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/controlplane"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/azure"
	"github.com/marvin-agent/marvin/internal/provider/kafka"
	"github.com/marvin-agent/marvin/internal/provider/kubernetes"
	"github.com/marvin-agent/marvin/internal/provider/linux"
	logprovider "github.com/marvin-agent/marvin/internal/provider/log"
	"github.com/marvin-agent/marvin/internal/provider/network"
	"github.com/marvin-agent/marvin/internal/provider/prometheus"
	"github.com/marvin-agent/marvin/internal/registry"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	marvinlog "github.com/marvin-agent/marvin/pkg/log"
	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
)

const defaultConfigPath = "/etc/marvin/config.yaml"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configPath := flag.String("config", defaultConfigPath, "path to Marvin config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	marvinlog.Configure(cfg.Logging.Level)
	log := marvinlog.Logger()
	log.Info("marvin starting", "config", *configPath, "agent_id", cfg.AgentID)

	if cfg.ControlPlane.TLS.IsEnabled() {
		log.Debug("preflight tls config", "enabled", true)
		if _, err := cfg.ControlPlane.TLS.BuildTLSConfig(); err != nil {
			return fmt.Errorf("preflight tls config: %w", err)
		}
	}

	for _, warn := range cfg.ValidateProviderConfigs() {
		log.Warn("provider config issue, capabilities will be unavailable until fixed", "warning", warn)
	}

	auth, err := buildAuthenticator(cfg.Authentication)
	if err != nil {
		return err
	}
	log.Debug("authenticator built", "method", auth.Method())

	reg := registry.New()
	enabledConfig := map[string]any{"enabled": cfg.Capabilities.Enabled}
	for _, p := range exampleProviders(*cfg) {
		if err := reg.RegisterProvider(p, enabledConfig); err != nil {
			return fmt.Errorf("register provider %q: %w", p.Name(), err)
		}
		if !p.IsConfigured() {
			log.Warn("provider not configured, capabilities unavailable until config is provided", "provider", p.Name())
		}
	}
	enabledCaps := reg.ListEnabled()
	log.Debug("providers registered", "enabled_capabilities", len(enabledCaps))

	client, err := controlplane.NewClient(
		cfg.ControlPlane, auth,
		controlplane.WithAgentID(cfg.AgentID),
		controlplane.WithRegistrationKey(cfg.RegistrationKey),
		controlplane.WithLabels(cfg.Labels),
		controlplane.WithTaskExecutor(&registryExecutor{reg: reg, logger: log}),
		controlplane.WithUpdateConfigHandler(func(ctx context.Context, upd *marvinpb.UpdateConfig) {
			reg.ApplyOnlineConfig(upd)
		}),
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Warn("close control plane client error", "error", closeErr)
		}
	}()
	log.Info("gRPC client created", "address", cfg.ControlPlane.Address)

	reg.SetOnCapabilitiesChanged(func(caps []capability.Capability) {
		log.Info("capabilities changed, triggering re-registration", "count", len(caps))
		client.UpdateCapabilities(caps)
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	registerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	log.Info("registering with control plane", "address", cfg.ControlPlane.Address, "timeout", "30s")
	if err := client.Register(registerCtx, cfg.Metadata, reg.ListEnabled()); err != nil {
		if registerCtx.Err() != nil && ctx.Err() == nil {
			log.Error("registration timed out", "error", err, "address", cfg.ControlPlane.Address)
			return fmt.Errorf("registration timed out connecting to %s: %w", cfg.ControlPlane.Address, err)
		}
		return err
	}
	log.Info("registered with control plane", "address", cfg.ControlPlane.Address)

	<-ctx.Done()
	log.Info("shutting down", "reason", ctx.Err())
	return nil
}

// registryExecutor adapts the registry to the controlplane.TaskExecutor interface.
type registryExecutor struct {
	reg    *registry.Registry
	logger *slog.Logger
}

func (r *registryExecutor) Execute(ctx context.Context, capabilityName string, params map[string]any) (task.Result, error) {
	log := r.logger
	if log == nil {
		log = slog.Default()
	}

	log.Info("executing capability", "capability", capabilityName)
	log.Debug("capability parameters", "capability", capabilityName, "params", params)

	t, ok := r.reg.GetTask(capabilityName)
	if !ok {
		log.Info("capability not found", "capability", capabilityName)
		return task.Result{Success: false, Error: fmt.Sprintf("capability %q not found", capabilityName)}, nil
	}

	log.Debug("capability task found, invoking execute", "capability", capabilityName)
	result, err := t.Execute(ctx, params)

	if err != nil {
		log.Info("capability execution failed with error", "capability", capabilityName, "error", err)
	} else if !result.Success {
		log.Info("capability execution returned failure", "capability", capabilityName, "error", result.Error)
	} else {
		log.Info("capability execution succeeded", "capability", capabilityName)
	}
	log.Debug("capability execution result", "capability", capabilityName, "result", result)

	return result, err
}

func buildAuthenticator(authCfg config.Authentication) (controlplane.Authenticator, error) {
	method, rawCfg, err := authCfg.AuthMethod()
	if err != nil {
		return nil, fmt.Errorf("resolve auth method: %w", err)
	}

	switch method {
	case "secret_key":
		secretCfg, ok := rawCfg.(config.SecretKeyAuth)
		if !ok {
			return nil, fmt.Errorf("unexpected secret_key config type %T", rawCfg)
		}
		return controlplane.SecretKeyAuthenticator{Key: secretCfg.Key}, nil
	default:
		return nil, fmt.Errorf("unsupported auth method %q", method)
	}
}

func exampleProviders(cfg config.Config) []provider.Provider {
	return []provider.Provider{
		network.NewProvider(),
		azure.NewProvider(cfg.Azure),
		kubernetes.NewProvider(cfg.Kubernetes),
		prometheus.NewProvider(cfg.Prometheus),
		kafka.NewProvider(cfg.Kafka),
		linux.NewProvider(cfg.Linux),
		logprovider.NewProvider(cfg.Graylog),
		provider.BaseProvider{
			ProviderName: "os",
			ProviderCapabilities: []capability.Capability{
				{Name: "os.process.list", Version: "v1", Description: "Lists processes", Provider: "os"},
				{Name: "os.file.read", Version: "v1", Description: "Reads files", Provider: "os"},
			},
		},
	}
}
