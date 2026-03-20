package tools

func schema(parts ...map[string]any) map[string]any {
	properties := map[string]any{}
	required := []string{}

	for _, part := range parts {
		if req, ok := part["__required__"].([]string); ok {
			required = append(required, req...)
			continue
		}
		for key, value := range part {
			properties[key] = value
		}
	}

	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func prop(name, kind, description string) map[string]any {
	return map[string]any{
		name: map[string]any{
			"type":        kind,
			"description": description,
		},
	}
}

func required(names ...string) map[string]any {
	return map[string]any{
		"__required__": names,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
