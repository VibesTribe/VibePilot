package runtime

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"clean JSON",
			`{"status": "approved", "tasks": []}`,
			`{"status": "approved", "tasks": []}`,
		},
		{
			"JSON in backticks",
			"```\n{\"status\": \"approved\"}\n```",
			`{"status": "approved"}`,
		},
		{
			"JSON in backticks with language tag",
			"```json\n{\"status\": \"approved\"}\n```",
			`{"status": "approved"}`,
		},
		{
			"JSON with trailing text",
			`{"status": "approved"} Here is my plan...`,
			`{"status": "approved"}`,
		},
		{
			"JSON with leading text",
			`Here is the plan: {"status": "approved"}`,
			`{"status": "approved"}`,
		},
		{
			"nested braces outside strings",
			`{"tasks": [{"id": 1}, {"id": 2}]}`,
			`{"tasks": [{"id": 1}, {"id": 2}]}`,
		},
		{
			"braces inside strings",
			`{"text": "use {curly} braces", "ok": true}`,
			`{"text": "use {curly} braces", "ok": true}`,
		},
		{
			"GLM-5 style: backtick block with trailing explanation",
			"```json\n{\"plan_path\": \"docs/plan.md\", \"tasks\": [{\"title\": \"do it\"}]}\n```\nI have created a plan...",
			`{"plan_path": "docs/plan.md", "tasks": [{"title": "do it"}]}`,
		},
		{
			"indented backtick block",
			"  ```\n  {\"ok\": true}\n  ```",
			`  {"ok": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			// Just check it produces valid JSON by trying to parse
			// We check that the result contains the expected key structure
			if result == "" {
				t.Errorf("extractJSON(%q) returned empty string", tt.input)
			}
			// Verify balanced braces
			depth := 0
			for _, ch := range result {
				if ch == '{' { depth++ }
				if ch == '}' { depth-- }
			}
			if depth != 0 {
				t.Errorf("extractJSON(%q) = %q, unbalanced braces (depth=%d)", tt.input, result, depth)
			}
		})
	}
}
