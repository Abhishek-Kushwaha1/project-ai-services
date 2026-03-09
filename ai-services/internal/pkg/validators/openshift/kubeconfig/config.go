package kubeconfig

import (
	"context"
	"fmt"

	"github.com/project-ai-services/ai-services/internal/pkg/constants"
	"github.com/project-ai-services/ai-services/internal/pkg/runtime/openshift"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	operatorGroup     = "operators.coreos.com"
	operatorResource  = "subscriptions"
	operatorVerb      = "create"
	operatorNamespace = "openshift-operators"
)

type KubeconfigRule struct{}

func NewKubeconfigRule() *KubeconfigRule {
	return &KubeconfigRule{}
}

func (r *KubeconfigRule) Name() string {
	return "kubeconfig"
}

func (r *KubeconfigRule) Description() string {
	return "Validates cluster access and operator installation permission"
}

// Verify checks if the kubeconfig can access the OpenShift cluster.
func (r *KubeconfigRule) Verify() error {
	ctx := context.Background()

	client, err := openshift.NewOpenshiftClient()
	if err != nil {
		return fmt.Errorf("failed to create openshift client: %w", err)
	}

	// listing namespaces to validate cluster access.
	if err := client.Client.List(ctx, &corev1.NamespaceList{}); err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	review := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Group:     operatorGroup,
				Resource:  operatorResource,
				Verb:      operatorVerb,
				Namespace: operatorNamespace,
			},
		},
	}

	if err := client.Client.Create(ctx, review); err != nil {
		return fmt.Errorf("failed to validate operator install permission: %w", err)
	}

	if review.Status.Denied || !review.Status.Allowed {
		return fmt.Errorf("user does not have permission to install operators")
	}

	return nil
}

func (r *KubeconfigRule) Message() string {
	return "Cluster authentication and operator permission validated"
}

func (r *KubeconfigRule) Level() constants.ValidationLevel {
	return constants.ValidationLevelError
}

func (r *KubeconfigRule) Hint() string {
	return "Make sure your kubeconfig is correctly configured and that you have the necessary permissions to access the OpenShift cluster."
}
