package kubernetes

import (
	"context"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*configMapListTask)(nil)

type configMapListTask struct{ provider *Provider }

func (t *configMapListTask) Name() string { return "kubernetes.configmap.list" }

func (t *configMapListTask) JSONSchema() string { return configMapListSchema }

func (t *configMapListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	labelSelector, err := common.OptionalString(params, "label_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.CoreV1().ConfigMaps(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []configMapSummary
	for _, cm := range list.Items {
		items = append(items, flattenConfigMap(cm))
	}

	return common.SuccessResult(map[string]any{"config_maps": items, "count": len(items)}), nil
}

func flattenConfigMap(cm corev1.ConfigMap) configMapSummary {
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return configMapSummary{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		DataKeys:  keys,
		Age:       formatAge(cm.CreationTimestamp.Time),
	}
}

const configMapListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ConfigMap List Parameters",
  "description": "Parameters for listing ConfigMaps.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of items to return.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`

type configMapSummary struct {
	Name      string   `json:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	DataKeys  []string `json:"data_keys,omitempty"`
	Age       string   `json:"age,omitempty"`
}
