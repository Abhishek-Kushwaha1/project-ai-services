package openshift

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/project-ai-services/ai-services/internal/pkg/logger"
)

const (
	SecondarySchedulerOperator = "secondaryscheduleroperator"
	CertManagerOperator        = "cert-manager-operator"
	ServiceMeshOperator        = "servicemeshoperator3"
	NFDOperator                = "nfd"
	RHOAIOperator              = "rhods-operator"
)

func (o *OpenshiftBootstrap) Validate(skip map[string]bool) error {
	ctx := context.Background()

	if err := o.validateAuthentication(ctx); err != nil {
		return err
	}

	if err := o.validateOperators(ctx); err != nil {
		return err
	}

	logger.Infoln("All validations passed")

	return nil
}

func (o *OpenshiftBootstrap) validateAuthentication(ctx context.Context) error {
	if err := ValidateAuthentication(ctx); err != nil {
		logger.Infoln("Authentication to OpenShift cluster")
		logger.Infof("HINT: %s\n", "Check your Kubeconfig and cluster access")

		return fmt.Errorf("cluster authentication failed: %w", err)
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#32BD27"))
	logger.Infoln(fmt.Sprintf("%s %s", style.Render("✓"), "Authentication to OpenShift cluster"))

	return nil
}

func (o *OpenshiftBootstrap) validateOperators(ctx context.Context) error {
	var validationErrors []error

	for _, check := range getOperatorChecks() {
		if err := ValidateOperator(ctx, check.operator); err != nil {
			logger.Infoln(check.name)
			logger.Infof("HINT: %s\n", check.hint)
			validationErrors = append(validationErrors, err)
		} else {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#32BD27"))
			logger.Infoln(fmt.Sprintf("%s %s", style.Render("✓"), check.name))
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("bootstrap validation failed: %d validation(s) failed", len(validationErrors))
	}

	return nil
}

func getOperatorChecks() []struct {
	name     string
	operator string
	hint     string
} {
	return []struct {
		name     string
		operator string
		hint     string
	}{
		{
			"Secondary Scheduler Operator installed",
			SecondarySchedulerOperator,
			"Install Secondary Scheduler Operator from OperatorHub",
		},
		{
			"Cert-Manager Operator installed",
			CertManagerOperator,
			"Install Cert-Manager Operator from OperatorHub",
		},
		{
			"Service Mesh 3 Operator installed",
			ServiceMeshOperator,
			"Install OpenShift Service Mesh Operator from OperatorHub",
		},
		{
			"Node Feature Discovery Operator installed",
			NFDOperator,
			"Install Node Feature Discovery Operator from OperatorHub",
		},
		{
			"RHOAI Operator installed and ready",
			RHOAIOperator,
			"Install RHOAI Operator or check CSV phase",
		},
	}
}
