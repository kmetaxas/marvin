package kubernetes

import (
	"context"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*configMapGetTask)(nil)

type configMapGetTask struct{ provider *Provider }

func (t *configMapGetTask) Name() string { return "kubernetes.configmap.get" }

func (t *configMapGetTask) JSONSchema() string { return configMapGetSchema }

func (t *configMapGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.RequireString(params, "namespace")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	cm, err := client.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"config_map": flattenConfigMapDetail(*cm)}), nil
}

const maxConfigMapDataValue = 4096

func flattenConfigMapDetail(cm corev1.ConfigMap) configMapDetail {
	data := make(map[string]any, len(cm.Data))
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := cm.Data[k]
		if len(v) > maxConfigMapDataValue {
			data[k] = v[:maxConfigMapDataValue]
			data[k+"_truncated"] = true
		} else {
			data[k] = v
		}
	}

	binaryKeys := make([]string, 0, len(cm.BinaryData))
	for k := range cm.BinaryData {
		binaryKeys = append(binaryKeys, k)
	}
	sort.Strings(binaryKeys)

	return configMapDetail{
		Name:           cm.Name,
		Namespace:      cm.Namespace,
		Data:           data,
		BinaryDataKeys: binaryKeys,
		Age:            formatAge(cm.CreationTimestamp.Time),
	}
}

const configMapGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ConfigMap Get Parameters",
  "description": "Parameters for retrieving a specific ConfigMap.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the ConfigMap."
    },
    "name": {
      "type": "string",
      "description": "Name of the ConfigMap."
    }
  },
  "required": ["namespace", "name"]
}`

type configMapDetail struct {
	Name           string         `json:"name,omitempty"`
	Namespace      string         `json:"namespace,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	BinaryDataKeys []string       `json:"binary_data_keys,omitempty"`
	Age            string         `json:"age,omitempty"`
}
