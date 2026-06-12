package extensions

import (
	"encoding/json"
	"sort"
	"strings"
)

// SampleInputForManifest returns the smallest safe JSON object that satisfies
// common generated extension schemas. It is guidance for review/reuse, not a
// substitute for user-provided task input.
func SampleInputForManifest(manifest Manifest) map[string]any {
	properties := schemaObjectMap(manifest.InputSchema["properties"])
	if len(properties) == 0 {
		return map[string]any{}
	}

	fields := schemaStringSlice(manifest.InputSchema["required"])
	if len(fields) == 0 {
		fields = preferredSampleFields(properties)
	}

	input := map[string]any{}
	for _, field := range fields {
		property, ok := properties[field]
		if !ok {
			continue
		}
		input[field] = sampleValueForProperty(field, property)
	}
	return input
}

func SampleInputJSONForManifest(manifest Manifest) string {
	data, err := json.Marshal(SampleInputForManifest(manifest))
	if err != nil {
		return "{}"
	}
	return string(data)
}

func preferredSampleFields(properties map[string]any) []string {
	preferred := []string{"task", "url", "query", "path"}
	for _, field := range preferred {
		if _, ok := properties[field]; ok {
			return []string{field}
		}
	}
	fields := make([]string, 0, len(properties))
	for field := range properties {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	if len(fields) > 3 {
		fields = fields[:3]
	}
	return fields
}

func sampleValueForProperty(name string, property any) any {
	propertyType := schemaType(property)
	field := strings.ToLower(name)
	switch propertyType {
	case "boolean":
		return false
	case "integer", "number":
		return 0
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		switch {
		case strings.Contains(field, "url"):
			return "https://example.com"
		case strings.Contains(field, "task"):
			return "Describe the task to run."
		case strings.Contains(field, "query"):
			return "Search query"
		case strings.Contains(field, "path"):
			return "path/to/input"
		case strings.Contains(field, "context"):
			return "Optional context"
		default:
			return ""
		}
	}
}

func schemaType(property any) string {
	object := schemaObjectMap(property)
	value, ok := object["type"]
	if !ok {
		return "string"
	}
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case []string:
		if len(typed) > 0 {
			return strings.ToLower(strings.TrimSpace(typed[0]))
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				return strings.ToLower(strings.TrimSpace(text))
			}
		}
	}
	return "string"
}

func schemaObjectMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return nil
	}
}

func schemaStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					values = append(values, text)
				}
			}
		}
		return values
	default:
		return nil
	}
}
