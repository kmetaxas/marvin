package kubernetes

import (
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
)

// newTestProvider creates a Provider with the given k8sClient for use in tests.
func newTestProvider(client *k8sClient) *Provider {
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, config.KubernetesConfig{}, client)
	return p
}
