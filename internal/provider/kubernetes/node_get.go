package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*nodeGetTask)(nil)

type nodeGetTask struct{ provider *Provider }

func (t *nodeGetTask) Name() string { return "kubernetes.node.get" }

func (t *nodeGetTask) JSONSchema() string { return nodeGetSchema }

func (t *nodeGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	node, err := client.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"node": flattenNodeDetail(*node)}), nil
}

func flattenNodeDetail(node corev1.Node) nodeDetail {
	return nodeDetail{
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
		Conditions:       flattenNodeConditions(node.Status.Conditions),
		Taints:           flattenNodeTaints(node.Spec.Taints),
		Capacity:         flattenResourceList(node.Status.Capacity),
		Allocatable:      flattenResourceList(node.Status.Allocatable),
	}
}

func flattenNodeConditions(conditions []corev1.NodeCondition) []conditionSummary {
	var result []conditionSummary
	for _, c := range conditions {
		result = append(result, conditionSummary{
			Type:   string(c.Type),
			Status: string(c.Status),
			Reason: c.Reason,
		})
	}
	return result
}

func flattenNodeTaints(taints []corev1.Taint) []taintSummary {
	var result []taintSummary
	for _, t := range taints {
		result = append(result, taintSummary{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}
	return result
}

const nodeGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Node Get Parameters",
  "description": "Parameters for retrieving a specific node.",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name of the node."
    }
  },
  "required": ["name"]
}`

type nodeDetail struct {
	Name             string             `json:"name,omitempty"`
	Roles            []string           `json:"roles,omitempty"`
	Status           string             `json:"status,omitempty"`
	Version          string             `json:"version,omitempty"`
	OSImage          string             `json:"os_image,omitempty"`
	KernelVersion    string             `json:"kernel_version,omitempty"`
	ContainerRuntime string             `json:"container_runtime,omitempty"`
	KubeletVersion   string             `json:"kubelet_version,omitempty"`
	Age              string             `json:"age,omitempty"`
	InternalIP       string             `json:"internal_ip,omitempty"`
	ExternalIP       string             `json:"external_ip,omitempty"`
	Conditions       []conditionSummary `json:"conditions,omitempty"`
	Taints           []taintSummary     `json:"taints,omitempty"`
	Capacity         map[string]string  `json:"capacity,omitempty"`
	Allocatable      map[string]string  `json:"allocatable,omitempty"`
}

type taintSummary struct {
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect,omitempty"`
}
