//go:build cssstub

package interaction

import (
	"errors"
	"net/url"
)

type Schema interface {
	Type() string
	Validate(data interface{}) error
}

type ObjectSchema struct {
	Fields map[string]Schema
}

func (s *ObjectSchema) Type() string {
	return "object"
}

func (s *ObjectSchema) Validate(data interface{}) error {
	// Placeholder for validation logic
	return nil
}

func IsUrl(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func ParseSchema(schema Schema) map[string]interface{} {
	result := make(map[string]interface{})
	result["type"] = schema.Type()
	result["required"] = true
	if schema.Type() == "object" {
		if objSchema, ok := schema.(*ObjectSchema); ok {
			fields := make(map[string]interface{})
			for field, fieldSchema := range objSchema.Fields {
				fields[field] = ParseSchema(fieldSchema)
			}
			result["fields"] = fields
		}
	}
	return result
}

func ValidateWithError(schema Schema, data interface{}) error {
	err := schema.Validate(data)
	if err != nil {
		return errors.New("validation error: " + err.Error())
	}
	return nil
}
