package kubernetes

import (
	"errors"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestNewK8sClientCapturesInitializationError(t *testing.T) {
	t.Parallel()

	orig := newRESTConfig
	newRESTConfig = func(_ config.KubernetesConfig) (*rest.Config, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { newRESTConfig = orig })

	client := newK8sClient(config.KubernetesConfig{})
	require.NotNil(t, client)
	require.Error(t, client.initErr)
	assert.Contains(t, client.initErr.Error(), "create kubernetes rest config")
}

func TestNewK8sClientCapturesClientsetError(t *testing.T) {
	t.Parallel()

	origRest := newRESTConfig
	origClientset := newKubernetesClientset
	newRESTConfig = func(_ config.KubernetesConfig) (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	newKubernetesClientset = func(_ *rest.Config) (kubernetes.Interface, error) {
		return nil, errors.New("clientset boom")
	}
	t.Cleanup(func() {
		newRESTConfig = origRest
		newKubernetesClientset = origClientset
	})

	client := newK8sClient(config.KubernetesConfig{})
	require.NotNil(t, client)
	require.Error(t, client.initErr)
	assert.Contains(t, client.initErr.Error(), "create kubernetes clientset")
}
