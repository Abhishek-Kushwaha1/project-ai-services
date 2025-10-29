package image

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
	"github.com/project-ai-services/ai-services/internal/pkg/cli/helpers"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var templateName string

const (
	applicationTemplatesPath = "applications/"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List container images for a given application template",
	Long:  ``,
	Args:  cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return list(cmd)
	},
}

func init() {
	listCmd.Flags().StringVarP(&templateName, "template", "t", "", "Application template name (Required)")
	listCmd.MarkFlagRequired("template")
}

func list(cmd *cobra.Command) error {
	// Fetch all the application Template names
	appTemplateNames, err := helpers.FetchApplicationTemplatesNames()
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	var appTemplateName string

	if index := fetchAppTemplateIndex(appTemplateNames, templateName); index == -1 {
		return errors.New("provided template name is wrong. Please provide a valid template name")
	} else {
		appTemplateName = appTemplateNames[index]
	}

	tmpls, err := helpers.LoadAllTemplates(applicationTemplatesPath + appTemplateName)
	if err != nil {
		return fmt.Errorf("failed to parse the templates: %w", err)
	}
	// Loop through all pod templates, render and run kube play
	cmd.Printf("Total Pod Templates to be processed: %d\n", len(tmpls))
	for name, tmpl := range tmpls {
		cmd.Printf("Processing template: %s...\n", name)

		params := map[string]any{
			"AppName": "sample-app",
		}

		var rendered bytes.Buffer
		if err := tmpl.Execute(&rendered, params); err != nil {
			return fmt.Errorf("failed to execute template %s: %v", name, err)
		}

		var podYAML v1.Pod
		if err := yaml.Unmarshal(rendered.Bytes(), &podYAML); err != nil {
			return fmt.Errorf("unable to read YAML as Kube Pod: %w", err)
		}
		for _, container := range podYAML.Spec.Containers {
			cmd.Printf("Container image: %s\n", container.Image)
		}
	}

	return nil

}

// fetchAppTemplateIndex -> Returns the index of app template if exists, otherwise -1
func fetchAppTemplateIndex(appTemplateNames []string, templateName string) int {
	appTemplateIndex := -1

	for index, appTemplateName := range appTemplateNames {
		if strings.EqualFold(appTemplateName, templateName) {
			appTemplateIndex = index
			break
		}
	}

	return appTemplateIndex
}
