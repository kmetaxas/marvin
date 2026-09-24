package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*authorizationCheckTask)(nil)

type authorizationCheckTask struct{ provider *Provider }

func (t *authorizationCheckTask) Name() string { return "kubernetes.authorization.check" }

func (t *authorizationCheckTask) JSONSchema() string { return authorizationCheckSchema }

func (t *authorizationCheckTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	verb, err := common.RequireString(params, "verb")
	if err != nil {
		return common.TaskFailure(err)
	}
	resource, err := common.RequireString(params, "resource")
	if err != nil {
		return common.TaskFailure(err)
	}
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.OptionalString(params, "name", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	apiGroup, err := common.OptionalString(params, "api_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	subresource, err := common.OptionalString(params, "subresource", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	user, err := common.OptionalString(params, "user", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	groups, err := optionalStringSlice(params, "groups")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	attrs := &authorizationv1.ResourceAttributes{
		Namespace:   namespace,
		Verb:        verb,
		Group:       apiGroup,
		Resource:    resource,
		Subresource: subresource,
		Name:        name,
	}

	result := map[string]any{
		"verb":         verb,
		"resource":     resource,
		"namespace":    namespace,
		"name":         name,
		"evaluated_as": "SelfSubjectAccessReview",
	}

	if user != "" {
		sar, err := client.clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User:               user,
				Groups:             groups,
				ResourceAttributes: attrs,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return common.TaskFailure(err)
		}

		result["allowed"] = sar.Status.Allowed
		result["reason"] = sar.Status.Reason
		result["evaluated_as"] = "SubjectAccessReview"
		return common.SuccessResult(result), nil
	}

	ssar, err := client.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attrs,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	result["allowed"] = ssar.Status.Allowed
	result["reason"] = ssar.Status.Reason
	return common.SuccessResult(result), nil
}

func optionalStringSlice(params map[string]any, key string) ([]string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, nil
	}

	switch values := v.(type) {
	case []string:
		result := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			result = append(result, value)
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			group, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %s must be a string list", key)
			}
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			result = append(result, group)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("parameter %s must be a string list", key)
	}
}

const authorizationCheckSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Authorization Check Parameters",
  "description": "Tests whether a Kubernetes identity is authorized to perform an action. Does not perform that action.",
  "properties": {
    "verb": {
      "type": "string",
      "description": "The verb to check (e.g., 'get', 'list', 'create', 'delete')."
    },
    "resource": {
      "type": "string",
      "description": "The resource type (e.g., 'pods', 'deployments')."
    },
    "namespace": {
      "type": "string",
      "description": "Namespace scope. Omit for cluster-scoped resources."
    },
    "name": {
      "type": "string",
      "description": "Specific resource name. Omit for collection operations."
    },
    "api_group": {
      "type": "string",
      "description": "API group of the resource. Omit for core resources."
    },
    "subresource": {
      "type": "string",
      "description": "Subresource to check (e.g., 'log', 'status')."
    },
    "user": {
      "type": "string",
      "description": "User to impersonate. Omit to check the Marvin service account."
    },
    "groups": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Groups to impersonate. Omit to check the Marvin service account."
    }
  },
  "required": ["verb", "resource"]
}`
