//go:build cssstub

package generate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"solid-go/internal/pods/settings"
)

// PodGenerator generates pods
type PodGenerator interface {
	// Generate generates a pod
	Generate(ctx context.Context, settings *settings.PodSettings) error
}

// ResourcesGenerator generates resources for a pod
type ResourcesGenerator interface {
	// Generate generates resources for a pod
	Generate(ctx context.Context, settings *settings.PodSettings) error
}

// IdentifierGenerator generates identifiers for pods
type IdentifierGenerator interface {
	// Generate generates an identifier for a pod
	Generate(settings *settings.PodSettings) (string, error)
}

// BaseResourcesGenerator is a base implementation of ResourcesGenerator
type BaseResourcesGenerator struct {
	generators []ResourcesGenerator
}

// NewBaseResourcesGenerator creates a new BaseResourcesGenerator
func NewBaseResourcesGenerator(generators ...ResourcesGenerator) *BaseResourcesGenerator {
	return &BaseResourcesGenerator{
		generators: generators,
	}
}

// Generate implements ResourcesGenerator.Generate
func (g *BaseResourcesGenerator) Generate(ctx context.Context, settings *settings.PodSettings) error {
	for _, generator := range g.generators {
		if err := generator.Generate(ctx, settings); err != nil {
			return err
		}
	}
	return nil
}

// TemplatedPodGenerator generates pods using templates
type TemplatedPodGenerator struct {
	generator ResourcesGenerator
}

// NewTemplatedPodGenerator creates a new TemplatedPodGenerator
func NewTemplatedPodGenerator(generator ResourcesGenerator) *TemplatedPodGenerator {
	return &TemplatedPodGenerator{
		generator: generator,
	}
}

// Generate implements PodGenerator.Generate
func (g *TemplatedPodGenerator) Generate(ctx context.Context, settings *settings.PodSettings) error {
	return g.generator.Generate(ctx, settings)
}

// SubfolderResourcesGenerator generates resources in subfolders
type SubfolderResourcesGenerator struct {
	generator ResourcesGenerator
}

// NewSubfolderResourcesGenerator creates a new SubfolderResourcesGenerator
func NewSubfolderResourcesGenerator(generator ResourcesGenerator) *SubfolderResourcesGenerator {
	return &SubfolderResourcesGenerator{
		generator: generator,
	}
}

// Generate implements ResourcesGenerator.Generate
func (g *SubfolderResourcesGenerator) Generate(ctx context.Context, settings *settings.PodSettings) error {
	return g.generator.Generate(ctx, settings)
}

// StaticFolderGenerator generates static folders
type StaticFolderGenerator struct {
	SourcePath string
	OutputPath string
}

// NewStaticFolderGenerator creates a new StaticFolderGenerator
func NewStaticFolderGenerator(sourcePath, outputPath string) *StaticFolderGenerator {
	return &StaticFolderGenerator{
		SourcePath: sourcePath,
		OutputPath: outputPath,
	}
}

// Generate implements ResourcesGenerator.Generate
func (g *StaticFolderGenerator) Generate(ctx context.Context, settings *settings.PodSettings) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(g.OutputPath, 0755); err != nil {
		return err
	}

	// Walk through source directory
	return filepath.Walk(g.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip source directory itself
		if path == g.SourcePath {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(g.SourcePath, path)
		if err != nil {
			return err
		}

		// Create target path
		targetPath := filepath.Join(g.OutputPath, relPath)

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy file
		return copyFile(path, targetPath)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	// Copy contents
	_, err = io.Copy(destination, source)
	return err
}
