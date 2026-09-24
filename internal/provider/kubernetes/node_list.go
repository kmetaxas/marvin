package kubernetes

import (
	"context"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*nodeListTask)(nil)

type nodeListTask struct{ provider *Provider }

func (t *nodeListTask) Name() string { return "kubernetes.node.list" }

func (t *nodeListTask) JSONSchema() string { return nodeListSchema }

func (t *nodeListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	labelSelector, err := common.OptionalString(params, "label_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	fieldSelector, err := common.OptionalString(params, "field_selector", "")
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
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []nodeSummary
	for _, node := range list.Items {
		items = append(items, flattenNode(node))
	}

	return common.SuccessResult(map[string]any{"nodes": items, "count": len(items)}), nil
}

func flattenNode(node corev1.Node) nodeSummary {
	return nodeSummary{
		Name:             node.Name,
		Roles:            extractNodeRoles(node.Labels),
		Status:           nodeReadyStatus(node.Status.Conditions),
		Version:          node.Status.NodeInfo.KubeletVersion,
		OSImage:          node.Status.NodeInfo.OSImage,
		KernelVersion:    node.Status.NodeInfo.KernelVersion,
		ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
		KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
		Age:              formatAge(node.CreationTimestamp.Time),
		InternalIP:       nodeInternalIP(node.Status.Addresses),
		ExternalIP:       nodeExternalIP(node.Status.Addresses),
	}
}

func extractNodeRoles(labels map[string]string) []string {
	var roles []string
	for k, v := range labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			} else if v != "" {
				roles = append(roles, v)
			}
		}
	}
	sort.Strings(roles)
	return roles
}

func nodeReadyStatus(conditions []corev1.NodeCondition) string {
	for _, c := range conditions {
		if c.Type == corev1.NodeReady {
			switch c.Status {
			case corev1.ConditionTrue:
				return "Ready"
			case corev1.ConditionFalse:
				return "NotReady"
			case corev1.ConditionUnknown:
				return "Unknown"
			}
		}
	}
	return "Unknown"
}

func nodeInternalIP(addresses []corev1.NodeAddress) string {
	for _, a := range addresses {
		if a.Type == corev1.NodeInternalIP {
			return a.Address
		}
	}
	return ""
}

func nodeExternalIP(addresses []corev1.NodeAddress) string {
	for _, a := range addresses {
		if a.Type == corev1.NodeExternalIP {
			return a.Address
		}
	}
	return ""
}

const nodeListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Node List Parameters",
  "description": "Parameters for listing nodes.",
  "properties": {
    "label_selector": {
      "type": "string",
      "description": "Label selector expression."
    },
    "field_selector": {
      "type": "string",
      "description": "Field selector expression."
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

type nodeSummary struct {
	Name             string   `json:"name,omitempty"`
	Roles            []string `json:"roles,omitempty"`
	Status           string   `json:"status,omitempty"`
	Version          string   `json:"version,omitempty"`
	OSImage          string   `json:"os_image,omitempty"`
	KernelVersion    string   `json:"kernel_version,omitempty"`
	ContainerRuntime string   `json:"container_runtime,omitempty"`
	KubeletVersion   string   `json:"kubelet_version,omitempty"`
	Age              string   `json:"age,omitempty"`
	InternalIP       string   `json:"internal_ip,omitempty"`
	ExternalIP       string   `json:"external_ip,omitempty"`
}
