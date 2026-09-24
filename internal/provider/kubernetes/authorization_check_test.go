package kubernetes

import (
	"context"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationCheckTaskAllowed(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		require.True(t, ok)

		review, ok := createAction.GetObject().(*authorizationv1.SelfSubjectAccessReview)
		require.True(t, ok)
		assert.Equal(t, "get", review.Spec.ResourceAttributes.Verb)
		assert.Equal(t, "pods", review.Spec.ResourceAttributes.Resource)
		assert.Equal(t, "default", review.Spec.ResourceAttributes.Namespace)
		assert.Equal(t, "nginx", review.Spec.ResourceAttributes.Name)

		return true, &authorizationv1.SelfSubjectAccessReview{
			ObjectMeta: metav1.ObjectMeta{Name: "review"},
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: true,
				Reason:  "allowed by test reactor",
			},
		}, nil
	})

	task := &authorizationCheckTask{provider: newTestProvider(&k8sClient{clientset: clientset})}
	res, err := task.Execute(context.Background(), map[string]any{
		"verb":      "get",
		"resource":  "pods",
		"namespace": "default",
		"name":      "nginx",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["allowed"])
	assert.Equal(t, "allowed by test reactor", data["reason"])
	assert.Equal(t, "get", data["verb"])
	assert.Equal(t, "pods", data["resource"])
	assert.Equal(t, "default", data["namespace"])
	assert.Equal(t, "nginx", data["name"])
	assert.Equal(t, "SelfSubjectAccessReview", data["evaluated_as"])
}

func TestAuthorizationCheckTaskDenied(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		require.True(t, ok)

		review, ok := createAction.GetObject().(*authorizationv1.SubjectAccessReview)
		require.True(t, ok)
		assert.Equal(t, "alice@example.com", review.Spec.User)
		assert.Equal(t, []string{"devs", "viewers"}, review.Spec.Groups)
		assert.Equal(t, "list", review.Spec.ResourceAttributes.Verb)
		assert.Equal(t, "deployments", review.Spec.ResourceAttributes.Resource)
		assert.Equal(t, "apps", review.Spec.ResourceAttributes.Group)

		return true, &authorizationv1.SubjectAccessReview{
			ObjectMeta: metav1.ObjectMeta{Name: "review"},
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: false,
				Reason:  "denied by test reactor",
			},
		}, nil
	})

	task := &authorizationCheckTask{provider: newTestProvider(&k8sClient{clientset: clientset})}
	res, err := task.Execute(context.Background(), map[string]any{
		"verb":      "list",
		"resource":  "deployments",
		"api_group": "apps",
		"user":      "alice@example.com",
		"groups":    []any{"devs", "viewers"},
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["allowed"])
	assert.Equal(t, "denied by test reactor", data["reason"])
	assert.Equal(t, "list", data["verb"])
	assert.Equal(t, "deployments", data["resource"])
	assert.Equal(t, "", data["namespace"])
	assert.Equal(t, "", data["name"])
	assert.Equal(t, "SubjectAccessReview", data["evaluated_as"])
}

func TestAuthorizationCheckTaskMissingParams(t *testing.T) {
	t.Parallel()

	task := &authorizationCheckTask{provider: newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})}

	res, err := task.Execute(context.Background(), map[string]any{"resource": "pods"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "verb")

	res, err = task.Execute(context.Background(), map[string]any{"verb": "get"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "resource")
}

func TestAuthorizationCheckTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &authorizationCheckTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"verb": "get", "resource": "pods"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
