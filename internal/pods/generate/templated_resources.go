//go:build cssstub

package generate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"text/template"

	"solid-go/internal/pods/settings"
)

// TemplatedResourcesGenerator generates resources using templates
type TemplatedResourcesGenerator struct {
	templatePath string
	outputPath   string
}

// NewTemplatedResourcesGenerator creates a new TemplatedResourcesGenerator
func NewTemplatedResourcesGenerator(templatePath, outputPath string) *TemplatedResourcesGenerator {
	return &TemplatedResourcesGenerator{
		templatePath: templatePath,
		outputPath:   outputPath,
	}
}

// Generate implements ResourcesGenerator.Generate
func (g *TemplatedResourcesGenerator) Generate(ctx context.Context, settings *settings.PodSettings) error {
	// Load template
	tmpl, err := template.ParseFiles(g.templatePath)
	if err != nil {
		return err
	}

	// Create output buffer
	var buf bytes.Buffer

	// Execute template with settings
	if err := tmpl.Execute(&buf, settings); err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(g.outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Write output
	return os.WriteFile(g.outputPath, buf.Bytes(), 0644)
}
