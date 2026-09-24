package kubernetes

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ provider.Provider = provider.BaseProvider{}

func TestProviderCurrentClient(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.KubernetesConfig{})
	require.NotNil(t, p)

	client := p.CurrentClient()
	require.NotNil(t, client)
}

func TestProviderUpdateConfig(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.KubernetesConfig{})
	require.NotNil(t, p)

	// Initial client should exist (possibly with initErr if no kubeconfig).
	initialClient := p.CurrentClient()
	require.NotNil(t, initialClient)

	// UpdateConfig with namespace/max_pod_log_lines should not recreate client.
	p.UpdateConfig("kubernetes.pod.logs", map[string]any{
		"namespace":         "default",
		"max_pod_log_lines": 500,
	})

	clientAfterNonCritical := p.CurrentClient()
	require.NotNil(t, clientAfterNonCritical)
	// Same underlying client since kubeconfig_path/context didn't change.
	assert.Equal(t, initialClient, clientAfterNonCritical)

	// UpdateConfig with kubeconfig_path change should recreate client.
	p.UpdateConfig("kubernetes.pod.list", map[string]any{
		"kubeconfig_path": "/tmp/nonexistent-kubeconfig",
	})

	clientAfterCritical := p.CurrentClient()
	require.NotNil(t, clientAfterCritical)
	// Client should be a different pointer because critical field changed.
	assert.NotEqual(t, initialClient, clientAfterCritical)
}

func TestNewProviderWiresAllCapabilities(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.KubernetesConfig{})
	require.NotNil(t, p)
	assert.Equal(t, "kubernetes", p.Name())

	capabilities := p.Capabilities()
	require.Len(t, capabilities, 54)

	names := map[string]bool{}
	for _, capability := range capabilities {
		names[capability.Name] = true
		assert.Equal(t, "kubernetes", capability.Provider)
		assert.NotEmpty(t, capability.ParametersJSONSchema)
	}

	for _, name := range []string{
		"kubernetes.pod.list",
		"kubernetes.pod.get",
		"kubernetes.pod.describe",
		"kubernetes.pod.get_full",
		"kubernetes.pod.logs",
		"kubernetes.deployment.list",
		"kubernetes.deployment.get",
		"kubernetes.replicaset.list",
		"kubernetes.replicaset.get",
		"kubernetes.statefulset.list",
		"kubernetes.statefulset.get",
		"kubernetes.daemonset.list",
		"kubernetes.daemonset.get",
		"kubernetes.job.list",
		"kubernetes.job.get",
		"kubernetes.cronjob.list",
		"kubernetes.cronjob.get",
		"kubernetes.event.list",
		"kubernetes.event.get",
		"kubernetes.service.list",
		"kubernetes.service.get",
		"kubernetes.endpoints.list",
		"kubernetes.endpoints.get",
		"kubernetes.endpointslice.list",
		"kubernetes.endpointslice.get",
		"kubernetes.networkpolicy.list",
		"kubernetes.networkpolicy.get",
		"kubernetes.ingress.list",
		"kubernetes.ingress.get",
		"kubernetes.node.list",
		"kubernetes.node.get",
		"kubernetes.namespace.list",
		"kubernetes.namespace.get",
		"kubernetes.version.get",
		"kubernetes.api_resources.list",
		"kubernetes.configmap.list",
		"kubernetes.configmap.get",
		"kubernetes.serviceaccount.list",
		"kubernetes.serviceaccount.get",
		"kubernetes.resourcequota.list",
		"kubernetes.resourcequota.get",
		"kubernetes.limitrange.list",
		"kubernetes.limitrange.get",
		"kubernetes.role.list",
		"kubernetes.role.get",
		"kubernetes.rolebinding.list",
		"kubernetes.rolebinding.get",
		"kubernetes.clusterrole.list",
		"kubernetes.clusterrole.get",
		"kubernetes.clusterrolebinding.list",
		"kubernetes.clusterrolebinding.get",
		"kubernetes.authorization.check",
		"kubernetes.custom_resource.list",
		"kubernetes.custom_resource.get",
	} {
		assert.True(t, names[name], name)
		_, ok := p.GetTask(name)
		assert.True(t, ok, name)
	}
}

// Compile-time interface assertions for all task types.
var (
	_ task.Task = (*podListTask)(nil)
	_ task.Task = (*podGetTask)(nil)
	_ task.Task = (*podDescribeTask)(nil)
	_ task.Task = (*podGetFullTask)(nil)
	_ task.Task = (*podLogsTask)(nil)
	_ task.Task = (*deploymentListTask)(nil)
	_ task.Task = (*deploymentGetTask)(nil)
	_ task.Task = (*replicaSetListTask)(nil)
	_ task.Task = (*replicaSetGetTask)(nil)
	_ task.Task = (*statefulSetListTask)(nil)
	_ task.Task = (*statefulSetGetTask)(nil)
	_ task.Task = (*daemonSetListTask)(nil)
	_ task.Task = (*daemonSetGetTask)(nil)
	_ task.Task = (*jobListTask)(nil)
	_ task.Task = (*jobGetTask)(nil)
	_ task.Task = (*cronJobListTask)(nil)
	_ task.Task = (*cronJobGetTask)(nil)
	_ task.Task = (*eventListTask)(nil)
	_ task.Task = (*eventGetTask)(nil)
	_ task.Task = (*serviceListTask)(nil)
	_ task.Task = (*serviceGetTask)(nil)
	_ task.Task = (*endpointsListTask)(nil)
	_ task.Task = (*endpointsGetTask)(nil)
	_ task.Task = (*endpointSliceListTask)(nil)
	_ task.Task = (*endpointSliceGetTask)(nil)
	_ task.Task = (*networkPolicyListTask)(nil)
	_ task.Task = (*networkPolicyGetTask)(nil)
	_ task.Task = (*ingressListTask)(nil)
	_ task.Task = (*ingressGetTask)(nil)
	_ task.Task = (*nodeListTask)(nil)
	_ task.Task = (*nodeGetTask)(nil)
	_ task.Task = (*namespaceListTask)(nil)
	_ task.Task = (*namespaceGetTask)(nil)
	_ task.Task = (*versionGetTask)(nil)
	_ task.Task = (*apiResourcesListTask)(nil)
	_ task.Task = (*configMapListTask)(nil)
	_ task.Task = (*configMapGetTask)(nil)
	_ task.Task = (*serviceAccountListTask)(nil)
	_ task.Task = (*serviceAccountGetTask)(nil)
	_ task.Task = (*resourceQuotaListTask)(nil)
	_ task.Task = (*resourceQuotaGetTask)(nil)
	_ task.Task = (*limitRangeListTask)(nil)
	_ task.Task = (*limitRangeGetTask)(nil)
	_ task.Task = (*roleListTask)(nil)
	_ task.Task = (*roleGetTask)(nil)
	_ task.Task = (*roleBindingListTask)(nil)
	_ task.Task = (*roleBindingGetTask)(nil)
	_ task.Task = (*clusterRoleListTask)(nil)
	_ task.Task = (*clusterRoleGetTask)(nil)
	_ task.Task = (*clusterRoleBindingListTask)(nil)
	_ task.Task = (*clusterRoleBindingGetTask)(nil)
	_ task.Task = (*authorizationCheckTask)(nil)
	_ task.Task = (*customResourceListTask)(nil)
	_ task.Task = (*customResourceGetTask)(nil)
)
