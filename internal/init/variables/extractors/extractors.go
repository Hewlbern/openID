//go:build cssstub

package extractors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"solid-go/internal/init/variables"
	"strings"
)

// EnvironmentVariableExtractor extracts variables from environment variables
type EnvironmentVariableExtractor struct {
	prefix string
}

// NewEnvironmentVariableExtractor creates a new EnvironmentVariableExtractor
func NewEnvironmentVariableExtractor(prefix string) *EnvironmentVariableExtractor {
	return &EnvironmentVariableExtractor{
		prefix: prefix,
	}
}

// Extract extracts variables from environment variables
func (e *EnvironmentVariableExtractor) Extract() ([]variables.Variable, error) {
	var vars []variables.Variable
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		if e.prefix != "" && !strings.HasPrefix(key, e.prefix) {
			continue
		}
		vars = append(vars, variables.Variable{
			Name:  key,
			Type:  variables.StringType,
			Value: value,
		})
	}
	return vars, nil
}

// ConfigVariableExtractor extracts variables from config files
type ConfigVariableExtractor struct {
	configPath string
}

// NewConfigVariableExtractor creates a new ConfigVariableExtractor
func NewConfigVariableExtractor(configPath string) *ConfigVariableExtractor {
	return &ConfigVariableExtractor{
		configPath: configPath,
	}
}

// Extract extracts variables from config files
func (e *ConfigVariableExtractor) Extract() ([]variables.Variable, error) {
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	vars := make([]variables.Variable, 0, len(raw))
	for k, v := range raw {
		varType := variables.StringType
		switch v.(type) {
		case float64, float32, int, int64, int32:
			varType = variables.NumberType
		case bool:
			varType = variables.BooleanType
		case []interface{}:
			varType = variables.ArrayType
		case map[string]interface{}:
			varType = variables.ObjectType
		}
		vars = append(vars, variables.Variable{
			Name:  k,
			Type:  varType,
			Value: v,
		})
	}
	return vars, nil
}

// CliVariableExtractor extracts variables from command line arguments
type CliVariableExtractor struct {
	args []string
}

// NewCliVariableExtractor creates a new CliVariableExtractor
func NewCliVariableExtractor(args []string) *CliVariableExtractor {
	return &CliVariableExtractor{
		args: args,
	}
}

// Extract extracts variables from command line arguments
func (e *CliVariableExtractor) Extract() ([]variables.Variable, error) {
	args := e.args
	if len(args) == 0 {
		args = os.Args[1:]
	}
	var vars []variables.Variable
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				vars = append(vars, variables.Variable{
					Name:  parts[0],
					Type:  variables.StringType,
					Value: parts[1],
				})
			}
		}
	}
	return vars, nil
}

// AssetPathExtractor extracts asset paths
type AssetPathExtractor struct {
	basePath string
}

// NewAssetPathExtractor creates a new AssetPathExtractor
func NewAssetPathExtractor(basePath string) *AssetPathExtractor {
	return &AssetPathExtractor{
		basePath: basePath,
	}
}

// Extract extracts asset paths
func (e *AssetPathExtractor) Extract() ([]variables.Variable, error) {
	// Get absolute path
	absPath, err := filepath.Abs(e.basePath)
	if err != nil {
		return nil, err
	}

	// Create variable
	return []variables.Variable{
		{
			Name:  "assetPath",
			Type:  variables.StringType,
			Value: absPath,
		},
	}, nil
}

// BaseUrlExtractor extracts base URLs
type BaseUrlExtractor struct {
	baseURL string
}

// NewBaseUrlExtractor creates a new BaseUrlExtractor
func NewBaseUrlExtractor(baseURL string) *BaseUrlExtractor {
	return &BaseUrlExtractor{
		baseURL: baseURL,
	}
}

// Extract extracts base URLs
func (e *BaseUrlExtractor) Extract() ([]variables.Variable, error) {
	// Create variable
	return []variables.Variable{
		{
			Name:  "baseUrl",
			Type:  variables.StringType,
			Value: e.baseURL,
		},
	}, nil
}

// KeyExtractor extracts keys
type KeyExtractor struct {
	keyPath string
}

// NewKeyExtractor creates a new KeyExtractor
func NewKeyExtractor(keyPath string) *KeyExtractor {
	return &KeyExtractor{
		keyPath: keyPath,
	}
}

// Extract extracts keys
func (e *KeyExtractor) Extract() ([]variables.Variable, error) {
	// Read key file
	data, err := os.ReadFile(e.keyPath)
	if err != nil {
		return nil, err
	}

	// Create variable
	return []variables.Variable{
		{
			Name:  "key",
			Type:  variables.StringType,
			Value: string(data),
		},
	}, nil
}

// ShorthandExtractor extracts shorthands
type ShorthandExtractor struct {
	shorthand string
}

// NewShorthandExtractor creates a new ShorthandExtractor
func NewShorthandExtractor(shorthand string) *ShorthandExtractor {
	return &ShorthandExtractor{
		shorthand: shorthand,
	}
}

// Extract extracts shorthands
func (e *ShorthandExtractor) Extract() ([]variables.Variable, error) {
	// Create variable
	return []variables.Variable{
		{
			Name:  "shorthand",
			Type:  variables.StringType,
			Value: e.shorthand,
		},
	}, nil
}
