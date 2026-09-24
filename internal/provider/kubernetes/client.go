package kubernetes

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/marvin-agent/marvin/internal/config"
)

var (
	newRESTConfig = func(cfg config.KubernetesConfig) (*rest.Config, error) {
		if cfg.KubeconfigPath == "" {
			if c, err := rest.InClusterConfig(); err == nil {
				return c, nil
			}
			return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{CurrentContext: cfg.Context},
			).ClientConfig()
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.KubeconfigPath},
			&clientcmd.ConfigOverrides{CurrentContext: cfg.Context},
		).ClientConfig()
	}
	newKubernetesClientset = func(restCfg *rest.Config) (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(restCfg)
	}
	newDynamicClient = func(restCfg *rest.Config) (dynamic.Interface, error) {
		return dynamic.NewForConfig(restCfg)
	}
)

type k8sClient struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	initErr       error
}

func newK8sClient(cfg config.KubernetesConfig) *k8sClient {
	c := &k8sClient{}
	restCfg, err := newRESTConfig(cfg)
	if err != nil {
		c.initErr = fmt.Errorf("create kubernetes rest config: %w", err)
		return c
	}
	clientset, err := newKubernetesClientset(restCfg)
	if err != nil {
		c.initErr = fmt.Errorf("create kubernetes clientset: %w", err)
		return c
	}
	c.clientset = clientset
	dynamicClient, err := newDynamicClient(restCfg)
	if err != nil {
		c.initErr = fmt.Errorf("create kubernetes dynamic client: %w", err)
		return c
	}
	c.dynamicClient = dynamicClient
	return c
}
