package agent

import (
	"testing"

	"Marcus/internal/model"
)

// TestParseToolArguments 验证 JSON 参数解析。
func TestParseToolArguments(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{
			input: `{"input":"/path/img.png","width":80}`,
			expected: map[string]string{
				"input": "/path/img.png",
				"width": "80",
			},
		},
		{
			input:    "",
			expected: map[string]string{},
		},
		{
			input:    `{}`,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		got, err := parseToolArguments(tt.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.input, err)
		}
		if len(got) != len(tt.expected) {
			t.Fatalf("parse %q: got %v, want %v", tt.input, got, tt.expected)
		}
		for k, v := range tt.expected {
			if got[k] != v {
				t.Fatalf("parse %q: key %q got %q want %q", tt.input, k, got[k], v)
			}
		}
	}
}

// TestParseToolArgumentsInvalidJSON 验证非法 JSON 返回错误。
func TestParseToolArgumentsInvalidJSON(t *testing.T) {
	_, err := parseToolArguments("not-json")
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

// TestExecutorMemoryToolNotInitialized 验证未初始化 memory store 时 memory 工具返回错误。
func TestExecutorMemoryToolNotInitialized(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterMemoryTool()
	exec := NewExecutor(reg, nil, nil)

	result := exec.Execute(nil, model.ToolCall{
		Function: model.ToolCallFunction{
			Name:      "memory",
			Arguments: `{"action":"add","key":"lang","content":"Chinese"}`,
		},
	})

	if result.Content == "" {
		t.Fatal("memory tool should return error content when store is nil")
	}
}
