package extensions

import (
	"fmt"
	"math"
	"strings"
)

func ValidateObjectAgainstSchema(name string, schema map[string]any, value map[string]any) error {
	if err := validateSchema(name, schema); err != nil {
		return err
	}
	for _, key := range schemaRequired(schema) {
		raw, ok := value[key]
		if !ok || requiredValueBlank(raw) {
			return fmt.Errorf("%s.%s is required", name, key)
		}
	}
	properties, _ := schema["properties"].(map[string]any)
	for key, rawProperty := range properties {
		rawValue, ok := value[key]
		if !ok {
			continue
		}
		property, ok := rawProperty.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.properties.%s must be an object", name, key)
		}
		if err := validateJSONValue(name+"."+key, property, rawValue); err != nil {
			return err
		}
	}
	return nil
}

func requiredValueBlank(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return false
	}
}

func schemaRequired(schema map[string]any) []string {
	required, ok := schema["required"]
	if !ok {
		return nil
	}
	switch values := required.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func validateJSONValue(name string, schema map[string]any, value any) error {
	expected := strings.TrimSpace(fmt.Sprint(schema["type"]))
	if expected == "" {
		return nil
	}
	switch expected {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", name)
		}
	case "integer":
		number, ok := numericValue(value)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("%s must be an integer", name)
		}
	case "number":
		if _, ok := numericValue(value); !ok {
			return fmt.Errorf("%s must be a number", name)
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s must be an object", name)
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s must be an array", name)
		}
	default:
		return fmt.Errorf("%s has unsupported schema type %q", name, expected)
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		for _, allowed := range enum {
			if fmt.Sprint(value) == fmt.Sprint(allowed) {
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %v", name, enum)
	}
	return nil
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	default:
		return 0, false
	}
}
