package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestKubernetesTaskSchemasAreValid(t *testing.T) {
	t.Parallel()

	for name, schemaText := range map[string]string{
		"pod_list":                podListSchema,
		"pod_get":                 podGetSchema,
		"pod_logs":                podLogsSchema,
		"deployment_list":         deploymentListSchema,
		"deployment_get":          deploymentGetSchema,
		"replicaset_list":         replicaSetListSchema,
		"replicaset_get":          replicaSetGetSchema,
		"statefulset_list":        statefulSetListSchema,
		"statefulset_get":         statefulSetGetSchema,
		"daemonset_list":          daemonSetListSchema,
		"daemonset_get":           daemonSetGetSchema,
		"job_list":                jobListSchema,
		"job_get":                 jobGetSchema,
		"cronjob_list":            cronJobListSchema,
		"cronjob_get":             cronJobGetSchema,
		"event_list":              eventListSchema,
		"event_get":               eventGetSchema,
		"service_list":            serviceListSchema,
		"service_get":             serviceGetSchema,
		"endpoints_list":          endpointsListSchema,
		"endpoints_get":           endpointsGetSchema,
		"endpointslice_list":      endpointSliceListSchema,
		"endpointslice_get":       endpointSliceGetSchema,
		"networkpolicy_list":      networkPolicyListSchema,
		"networkpolicy_get":       networkPolicyGetSchema,
		"ingress_list":            ingressListSchema,
		"ingress_get":             ingressGetSchema,
		"node_list":               nodeListSchema,
		"node_get":                nodeGetSchema,
		"namespace_list":          namespaceListSchema,
		"namespace_get":           namespaceGetSchema,
		"version_get":             versionGetSchema,
		"api_resources_list":      apiResourcesListSchema,
		"configmap_list":          configMapListSchema,
		"configmap_get":           configMapGetSchema,
		"serviceaccount_list":     serviceAccountListSchema,
		"serviceaccount_get":      serviceAccountGetSchema,
		"resourcequota_list":      resourceQuotaListSchema,
		"resourcequota_get":       resourceQuotaGetSchema,
		"limitrange_list":         limitRangeListSchema,
		"limitrange_get":          limitRangeGetSchema,
		"role_list":               roleListSchema,
		"role_get":                roleGetSchema,
		"rolebinding_list":        roleBindingListSchema,
		"rolebinding_get":         roleBindingGetSchema,
		"clusterrole_list":        clusterRoleListSchema,
		"clusterrole_get":         clusterRoleGetSchema,
		"clusterrolebinding_list": clusterRoleBindingListSchema,
		"clusterrolebinding_get":  clusterRoleBindingGetSchema,
		"authorization_check":     authorizationCheckSchema,
	} {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(schemaText)))
		require.NoError(t, err, name)
		require.NotNil(t, schema, name)
	}
}

func TestKubernetesTaskSchemasValidateExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		schema  string
		valid   map[string]any
		invalid map[string]any
	}{
		{podListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{podGetSchema, map[string]any{"namespace": "default", "name": "nginx"}, map[string]any{"namespace": "default"}},
		{podLogsSchema, map[string]any{"namespace": "default", "name": "nginx", "tail_lines": 50, "previous": true, "search": "error"}, map[string]any{"namespace": "default"}},
		{deploymentListSchema, map[string]any{"namespace": "default", "limit": 50}, map[string]any{"limit": 0}},
		{deploymentGetSchema, map[string]any{"namespace": "default", "name": "nginx"}, map[string]any{"name": "nginx"}},
		{replicaSetListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{replicaSetGetSchema, map[string]any{"namespace": "default", "name": "rs1"}, map[string]any{"namespace": "default"}},
		{statefulSetListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{statefulSetGetSchema, map[string]any{"namespace": "default", "name": "web"}, map[string]any{"name": "web"}},
		{daemonSetListSchema, map[string]any{"namespace": "kube-system"}, map[string]any{"limit": 0}},
		{daemonSetGetSchema, map[string]any{"namespace": "kube-system", "name": "fluentd"}, map[string]any{"namespace": "kube-system"}},
		{jobListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{jobGetSchema, map[string]any{"namespace": "default", "name": "backup"}, map[string]any{"namespace": "default"}},
		{cronJobListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{cronJobGetSchema, map[string]any{"namespace": "default", "name": "backup"}, map[string]any{"namespace": "default"}},
		{eventListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{eventGetSchema, map[string]any{"namespace": "default", "name": "ev1"}, map[string]any{"namespace": "default"}},
		{serviceListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{serviceGetSchema, map[string]any{"namespace": "default", "name": "nginx"}, map[string]any{"namespace": "default"}},
		{endpointsListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{endpointsGetSchema, map[string]any{"namespace": "default", "name": "nginx"}, map[string]any{"namespace": "default"}},
		{endpointSliceListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{endpointSliceGetSchema, map[string]any{"namespace": "default", "name": "nginx-abc"}, map[string]any{"namespace": "default"}},
		{networkPolicyListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{networkPolicyGetSchema, map[string]any{"namespace": "default", "name": "deny-all"}, map[string]any{"namespace": "default"}},
		{nodeListSchema, map[string]any{"label_selector": "app=nginx", "limit": 10}, map[string]any{"limit": 0}},
		{nodeGetSchema, map[string]any{"name": "node-1"}, map[string]any{}},
		{namespaceListSchema, map[string]any{"label_selector": "env=prod"}, map[string]any{"limit": 0}},
		{namespaceGetSchema, map[string]any{"name": "default"}, map[string]any{}},
		{versionGetSchema, map[string]any{}, map[string]any{"extra": "bad"}},
		{apiResourcesListSchema, map[string]any{"api_group": "apps"}, map[string]any{"namespaced": "yes"}},
		{configMapListSchema, map[string]any{"namespace": "default", "limit": 50}, map[string]any{"limit": 0}},
		{configMapGetSchema, map[string]any{"namespace": "default", "name": "nginx-config"}, map[string]any{"namespace": "default"}},
		{serviceAccountListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{serviceAccountGetSchema, map[string]any{"namespace": "default", "name": "default"}, map[string]any{"namespace": "default"}},
		{resourceQuotaListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{resourceQuotaGetSchema, map[string]any{"namespace": "default", "name": "compute-quota"}, map[string]any{"namespace": "default"}},
		{limitRangeListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{limitRangeGetSchema, map[string]any{"namespace": "default", "name": "cpu-memory-limits"}, map[string]any{"namespace": "default"}},
		{ingressListSchema, map[string]any{"namespace": "default", "limit": 10}, map[string]any{"limit": 0}},
		{ingressGetSchema, map[string]any{"namespace": "default", "name": "web"}, map[string]any{"namespace": "default"}},
		{roleListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{roleGetSchema, map[string]any{"namespace": "default", "name": "pod-reader"}, map[string]any{"namespace": "default"}},
		{roleBindingListSchema, map[string]any{"namespace": "default"}, map[string]any{"limit": 0}},
		{roleBindingGetSchema, map[string]any{"namespace": "default", "name": "read-pods"}, map[string]any{"namespace": "default"}},
		{clusterRoleListSchema, map[string]any{"limit": 50}, map[string]any{"limit": 0}},
		{clusterRoleGetSchema, map[string]any{"name": "cluster-admin"}, map[string]any{}},
		{clusterRoleBindingListSchema, map[string]any{"limit": 50}, map[string]any{"limit": 0}},
		{clusterRoleBindingGetSchema, map[string]any{"name": "cluster-admin-binding"}, map[string]any{}},
		{authorizationCheckSchema, map[string]any{"verb": "get", "resource": "pods", "namespace": "default"}, map[string]any{"verb": "get"}},
	}

	for _, tt := range tests {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(tt.schema)))
		require.NoError(t, err)

		validRes, err := schema.Validate(gojsonschema.NewGoLoader(tt.valid))
		require.NoError(t, err)
		require.True(t, validRes.Valid(), validRes.Errors())

		invalidRes, err := schema.Validate(gojsonschema.NewGoLoader(tt.invalid))
		require.NoError(t, err)
		require.False(t, invalidRes.Valid())
	}
}
