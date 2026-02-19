package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/project-ai-services/ai-services/internal/pkg/runtime/openshift"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type OCPHelper struct {
	client *openshift.OpenshiftClient
}

/* ---------- Operator Validation ---------- */

func (h *OCPHelper) ValidateOperator(ctx context.Context, operatorSubstring string) error {
	csvList := &unstructured.UnstructuredList{}
	csvList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "ClusterServiceVersionList",
	})

	if err := h.client.GetClient().List(ctx, csvList); err != nil {
		return fmt.Errorf("failed to list ClusterServiceVersions: %w", err)
	}

	for _, csv := range csvList.Items {
		name := csv.GetName()

		// Using LowerCase for both makes the check case-insensitive and robust
		if !strings.Contains(strings.ToLower(name), strings.ToLower(operatorSubstring)) {
			continue
		}

		phase, _, _ := unstructured.NestedString(
			csv.Object,
			"status",
			"phase",
		)

		if phase == "Succeeded" {
			return nil // Found and healthy!
		}

		return fmt.Errorf(
			"operator %s found but not ready (phase=%s)",
			name,
			phase,
		)
	}

	return fmt.Errorf("operator not installed: %s", operatorSubstring)
}
