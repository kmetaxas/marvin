package kubernetes

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider implements the provider.Provider interface for Kubernetes.
type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.KubernetesConfig, *k8sClient]

	provider.BaseProvider
}

// NewProvider creates a new Kubernetes provider with the given configuration.
func NewProvider(cfg config.KubernetesConfig) *Provider {
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, newK8sClient(cfg))

	podListTask := &podListTask{provider: p}
	podGetTask := &podGetTask{provider: p}
	podDescribeTask := &podDescribeTask{provider: p}
	podGetFullTask := &podGetFullTask{provider: p}
	podLogsTask := &podLogsTask{provider: p, maxLogLines: cfg.MaxPodLogLines}
	deploymentListTask := &deploymentListTask{provider: p}
	deploymentGetTask := &deploymentGetTask{provider: p}
	replicaSetListTask := &replicaSetListTask{provider: p}
	replicaSetGetTask := &replicaSetGetTask{provider: p}
	statefulSetListTask := &statefulSetListTask{provider: p}
	statefulSetGetTask := &statefulSetGetTask{provider: p}
	daemonSetListTask := &daemonSetListTask{provider: p}
	daemonSetGetTask := &daemonSetGetTask{provider: p}
	jobListTask := &jobListTask{provider: p}
	jobGetTask := &jobGetTask{provider: p}
	cronJobListTask := &cronJobListTask{provider: p}
	cronJobGetTask := &cronJobGetTask{provider: p}
	eventListTask := &eventListTask{provider: p}
	eventGetTask := &eventGetTask{provider: p}
	serviceListTask := &serviceListTask{provider: p}
	serviceGetTask := &serviceGetTask{provider: p}
	endpointsListTask := &endpointsListTask{provider: p}
	endpointsGetTask := &endpointsGetTask{provider: p}
	endpointSliceListTask := &endpointSliceListTask{provider: p}
	endpointSliceGetTask := &endpointSliceGetTask{provider: p}
	networkPolicyListTask := &networkPolicyListTask{provider: p}
	networkPolicyGetTask := &networkPolicyGetTask{provider: p}
	ingressListTask := &ingressListTask{provider: p}
	ingressGetTask := &ingressGetTask{provider: p}
	nodeListTask := &nodeListTask{provider: p}
	nodeGetTask := &nodeGetTask{provider: p}
	namespaceListTask := &namespaceListTask{provider: p}
	namespaceGetTask := &namespaceGetTask{provider: p}
	versionGetTask := &versionGetTask{provider: p}
	apiResourcesListTask := &apiResourcesListTask{provider: p}
	configMapListTask := &configMapListTask{provider: p}
	configMapGetTask := &configMapGetTask{provider: p}
	serviceAccountListTask := &serviceAccountListTask{provider: p}
	serviceAccountGetTask := &serviceAccountGetTask{provider: p}
	resourceQuotaListTask := &resourceQuotaListTask{provider: p}
	resourceQuotaGetTask := &resourceQuotaGetTask{provider: p}
	limitRangeListTask := &limitRangeListTask{provider: p}
	limitRangeGetTask := &limitRangeGetTask{provider: p}
	roleListTask := &roleListTask{provider: p}
	roleGetTask := &roleGetTask{provider: p}
	roleBindingListTask := &roleBindingListTask{provider: p}
	roleBindingGetTask := &roleBindingGetTask{provider: p}
	clusterRoleListTask := &clusterRoleListTask{provider: p}
	clusterRoleGetTask := &clusterRoleGetTask{provider: p}
	clusterRoleBindingListTask := &clusterRoleBindingListTask{provider: p}
	clusterRoleBindingGetTask := &clusterRoleBindingGetTask{provider: p}
	authorizationCheckTask := &authorizationCheckTask{provider: p}
	customResourceListTask := &customResourceListTask{provider: p}
	customResourceGetTask := &customResourceGetTask{provider: p}

	tasks := []task.Task{
		podListTask, podGetTask, podDescribeTask, podGetFullTask, podLogsTask,
		deploymentListTask, deploymentGetTask,
		replicaSetListTask, replicaSetGetTask,
		statefulSetListTask, statefulSetGetTask,
		daemonSetListTask, daemonSetGetTask,
		jobListTask, jobGetTask,
		cronJobListTask, cronJobGetTask,
		eventListTask, eventGetTask,
		serviceListTask, serviceGetTask,
		endpointsListTask, endpointsGetTask,
		endpointSliceListTask, endpointSliceGetTask,
		networkPolicyListTask, networkPolicyGetTask,
		ingressListTask, ingressGetTask,
		nodeListTask, nodeGetTask,
		namespaceListTask, namespaceGetTask,
		versionGetTask,
		apiResourcesListTask,
		configMapListTask, configMapGetTask,
		serviceAccountListTask, serviceAccountGetTask,
		resourceQuotaListTask, resourceQuotaGetTask,
		limitRangeListTask, limitRangeGetTask,
		roleListTask, roleGetTask,
		roleBindingListTask, roleBindingGetTask,
		clusterRoleListTask, clusterRoleGetTask,
		clusterRoleBindingListTask, clusterRoleBindingGetTask,
		authorizationCheckTask,
		customResourceListTask, customResourceGetTask,
	}
	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "kubernetes",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.BaseProvider = provider.BaseProvider{
		ProviderName:         "kubernetes",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	return p
}

// CurrentClient returns the current Kubernetes client safely for concurrent use.
func (p *Provider) CurrentClient() *k8sClient {
	return p.state.Client()
}

func (p *Provider) IsConfigured() bool { return true }

// UpdateConfig updates the provider configuration and recreates the Kubernetes
// client if kubeconfig_path or context changed.
func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName
	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.KubernetesConfig, *k8sClient]{
		Parse: func(base config.KubernetesConfig, raw map[string]any) (config.KubernetesConfig, error) {
			out := base
			if v, ok := autoconfig.StringOK(raw, "kubeconfig_path"); ok {
				out.KubeconfigPath = v
			}
			if v, ok := autoconfig.StringOK(raw, "context"); ok {
				out.Context = v
			}
			if v, ok := autoconfig.StringOK(raw, "namespace"); ok {
				out.Namespace = v
			}
			if v, ok := autoconfig.IntOK(raw, "max_pod_log_lines"); ok {
				out.MaxPodLogLines = v
			}
			return out, nil
		},
		Equal: func(a, b config.KubernetesConfig) bool { return a == b },
		Recreate: func(oldCfg, newCfg config.KubernetesConfig) bool {
			return oldCfg.KubeconfigPath != newCfg.KubeconfigPath || oldCfg.Context != newCfg.Context
		},
		Build: func(cfg config.KubernetesConfig) (*k8sClient, error) {
			return newK8sClient(cfg), nil
		},
	})
}

func capabilityDescription(name string) string {
	descriptions := map[string]string{
		"kubernetes.pod.list":                "Lists pods and their basic status",
		"kubernetes.pod.get":                 "Retrieves a specific pod",
		"kubernetes.pod.describe":            "Describes a pod in detail including status, conditions, container states, and recent Kubernetes events for troubleshooting",
		"kubernetes.pod.get_full":            "Retrieves the complete raw pod object including all spec, status, and metadata fields as JSON for deep debugging",
		"kubernetes.pod.logs":                "Retrieves container logs for a specific pod with optional keyword search",
		"kubernetes.deployment.list":         "Lists Deployments",
		"kubernetes.deployment.get":          "Retrieves a specific Deployment",
		"kubernetes.replicaset.list":         "Lists ReplicaSets",
		"kubernetes.replicaset.get":          "Retrieves a specific ReplicaSet",
		"kubernetes.statefulset.list":        "Lists StatefulSets",
		"kubernetes.statefulset.get":         "Retrieves a specific StatefulSet",
		"kubernetes.daemonset.list":          "Lists DaemonSets",
		"kubernetes.daemonset.get":           "Retrieves a specific DaemonSet",
		"kubernetes.job.list":                "Lists Jobs",
		"kubernetes.job.get":                 "Retrieves a specific Job",
		"kubernetes.cronjob.list":            "Lists CronJobs",
		"kubernetes.cronjob.get":             "Retrieves a specific CronJob",
		"kubernetes.event.list":              "Lists Kubernetes events",
		"kubernetes.event.get":               "Retrieves a specific Kubernetes event",
		"kubernetes.service.list":            "Lists Services with optional filtering by service type such as LoadBalancer, ClusterIP, NodePort, or ExternalName",
		"kubernetes.service.get":             "Retrieves a specific Service",
		"kubernetes.endpoints.list":          "Lists Endpoints",
		"kubernetes.endpoints.get":           "Retrieves a specific Endpoints resource",
		"kubernetes.endpointslice.list":      "Lists EndpointSlices",
		"kubernetes.endpointslice.get":       "Retrieves a specific EndpointSlice",
		"kubernetes.networkpolicy.list":      "Lists NetworkPolicies",
		"kubernetes.networkpolicy.get":       "Retrieves a specific NetworkPolicy",
		"kubernetes.ingress.list":            "Lists Ingresses",
		"kubernetes.ingress.get":             "Retrieves a specific Ingress",
		"kubernetes.node.list":               "Lists nodes including basic status, roles and versions",
		"kubernetes.node.get":                "Retrieves complete Node spec/status",
		"kubernetes.namespace.list":          "Lists namespaces and their status",
		"kubernetes.namespace.get":           "Retrieves a specific Namespace",
		"kubernetes.version.get":             "Retrieves Kubernetes server version information",
		"kubernetes.api_resources.list":      "Lists resource kinds/APIs supported by the cluster",
		"kubernetes.configmap.list":          "Lists ConfigMaps",
		"kubernetes.configmap.get":           "Retrieves a ConfigMap including its data",
		"kubernetes.serviceaccount.list":     "Lists ServiceAccounts",
		"kubernetes.serviceaccount.get":      "Retrieves a specific ServiceAccount",
		"kubernetes.resourcequota.list":      "Lists ResourceQuotas",
		"kubernetes.resourcequota.get":       "Retrieves quota limits and current usage",
		"kubernetes.limitrange.list":         "Lists LimitRanges",
		"kubernetes.limitrange.get":          "Retrieves namespace resource default/min/max constraints",
		"kubernetes.role.list":               "Lists namespaced Roles",
		"kubernetes.role.get":                "Retrieves Role rules",
		"kubernetes.rolebinding.list":        "Lists RoleBindings",
		"kubernetes.rolebinding.get":         "Retrieves RoleBinding subjects and referenced role",
		"kubernetes.clusterrole.list":        "Lists ClusterRoles",
		"kubernetes.clusterrole.get":         "Retrieves ClusterRole rules",
		"kubernetes.clusterrolebinding.list": "Lists ClusterRoleBindings",
		"kubernetes.clusterrolebinding.get":  "Retrieves ClusterRoleBinding subjects and role",
		"kubernetes.authorization.check":     "Tests whether a Kubernetes identity is authorized to perform an action",
		"kubernetes.custom_resource.list":    "Lists arbitrary Kubernetes Custom Resources by API group, version, and resource name for flexible cluster introspection",
		"kubernetes.custom_resource.get":     "Retrieves a specific Kubernetes Custom Resource by API group, version, resource name, and object name",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Kubernetes capability"
}
