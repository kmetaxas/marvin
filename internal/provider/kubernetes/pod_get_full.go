package kubernetes

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*podGetFullTask)(nil)

type podGetFullTask struct{ provider *Provider }

func (t *podGetFullTask) Name() string { return "kubernetes.pod.get_full" }

func (t *podGetFullTask) JSONSchema() string { return podGetFullSchema }

func (t *podGetFullTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	pod, err := client.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		return common.TaskFailure(err)
	}
	rawJSONMap := map[string]any{}
	if err := json.Unmarshal(raw, &rawJSONMap); err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"pod": rawJSONMap}), nil
}

const podGetFullSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Pod Get Full Parameters",
  "description": "Parameters for retrieving the complete raw JSON representation of a pod.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the pod."
    },
    "name": {
      "type": "string",
      "description": "Name of the pod."
    }
  },
  "required": ["namespace", "name"]
}`
