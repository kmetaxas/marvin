package kubernetes

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*podGetTask)(nil)

type podGetTask struct{ provider *Provider }

func (t *podGetTask) Name() string { return "kubernetes.pod.get" }

func (t *podGetTask) JSONSchema() string { return podGetSchema }

func (t *podGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	return common.SuccessResult(map[string]any{"pod": flattenPodDetail(*pod)}), nil
}

func flattenPodDetail(pod corev1.Pod) podDetail {
	var containers []containerSummary
	for _, c := range pod.Spec.Containers {
		containers = append(containers, containerSummary{
			Name:  c.Name,
			Image: c.Image,
		})
	}
	for idx, cs := range pod.Status.ContainerStatuses {
		if idx < len(containers) {
			containers[idx].Ready = cs.Ready
			containers[idx].RestartCount = int(cs.RestartCount)
			if cs.State.Running != nil {
				containers[idx].State = "running"
			} else if cs.State.Waiting != nil {
				containers[idx].State = "waiting"
			} else if cs.State.Terminated != nil {
				containers[idx].State = "terminated"
			}
		}
	}
	var conditions []conditionSummary
	for _, c := range pod.Status.Conditions {
		conditions = append(conditions, conditionSummary{
			Type:   string(c.Type),
			Status: string(c.Status),
			Reason: c.Reason,
		})
	}
	startTime := ""
	if pod.Status.StartTime != nil {
		startTime = pod.Status.StartTime.Format(time.RFC3339)
	}
	return podDetail{
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		UID:               string(pod.UID),
		CreationTimestamp: pod.CreationTimestamp.Format(time.RFC3339),
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
		NodeName:          pod.Spec.NodeName,
		ServiceAccount:    pod.Spec.ServiceAccountName,
		RestartPolicy:     string(pod.Spec.RestartPolicy),
		Containers:        containers,
		Phase:             string(pod.Status.Phase),
		Conditions:        conditions,
		PodIP:             pod.Status.PodIP,
		StartTime:         startTime,
	}
}

const podGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Pod Get Parameters",
  "description": "Parameters for retrieving a specific pod.",
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

type podDetail struct {
	Name              string             `json:"name,omitempty"`
	Namespace         string             `json:"namespace,omitempty"`
	UID               string             `json:"uid,omitempty"`
	CreationTimestamp string             `json:"creation_timestamp,omitempty"`
	Labels            map[string]string  `json:"labels,omitempty"`
	Annotations       map[string]string  `json:"annotations,omitempty"`
	NodeName          string             `json:"node_name,omitempty"`
	ServiceAccount    string             `json:"service_account,omitempty"`
	RestartPolicy     string             `json:"restart_policy,omitempty"`
	Containers        []containerSummary `json:"containers,omitempty"`
	Phase             string             `json:"phase,omitempty"`
	Conditions        []conditionSummary `json:"conditions,omitempty"`
	PodIP             string             `json:"pod_ip,omitempty"`
	StartTime         string             `json:"start_time,omitempty"`
}

type containerSummary struct {
	Name         string `json:"name,omitempty"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready,omitempty"`
	RestartCount int    `json:"restart_count,omitempty"`
	State        string `json:"state,omitempty"`
}
