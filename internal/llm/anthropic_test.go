package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Marcus/internal/model"
)

// TestAnthropicChat 验证 Anthropic Provider 能正确发起请求并解析工具调用响应。
func TestAnthropicChat(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("x-api-key"); auth != "test-key" {
			t.Errorf("unexpected api key: %s", auth)
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"content": [
				{"type": "text", "text": "I will convert it."},
				{"type": "tool_use", "id": "tu_1", "name": "marcus-img2ascii", "input": {"input": "/path/img.png"}}
			],
			"usage": {"input_tokens": 15, "output_tokens": 25}
		}`)
	}))
	defer server.Close()

	provider := NewAnthropic(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "claude-3-5-sonnet-latest",
	})

	resp, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "You are a helpful assistant."},
			{Role: model.RoleUser, Content: "把这张图片转成字符画"},
		},
		Tools: []model.ToolDefinition{
			{
				Function: model.ToolFunctionDefinition{
					Name:        "marcus-img2ascii",
					Description: "Convert image to ASCII art",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"input": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	if resp.Content != "I will convert it." {
		t.Fatalf("content = %q, want %q", resp.Content, "I will convert it.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls count = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "marcus-img2ascii" {
		t.Fatalf("tool name = %q, want marcus-img2ascii", resp.ToolCalls[0].Function.Name)
	}
	if resp.Usage.TotalTokens != 40 {
		t.Fatalf("total tokens = %d, want 40", resp.Usage.TotalTokens)
	}

	// Anthropic 要求 system 在请求体顶层
	if capturedBody["system"] != "You are a helpful assistant." {
		t.Fatalf("system = %v, want top-level system prompt", capturedBody["system"])
	}
}

// TestAnthropicChatStream 验证流式响应解析。
func TestAnthropicChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flush")
		}

		chunks := []string{
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
			`data: {"type":"message_stop"}`,
		}
		for _, c := range chunks {
			fmt.Fprintln(w, c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewAnthropic(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "claude-3-5-sonnet-latest",
	})

	ch, err := provider.ChatStream(context.Background(), &model.ChatRequest{
		Model:    "claude-3-5-sonnet-latest",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	var content strings.Builder
	var done bool
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream chunk error: %v", chunk.Error)
		}
		if chunk.Done {
			done = true
			break
		}
		content.WriteString(chunk.Content)
	}

	if !done {
		t.Fatal("stream did not receive done chunk")
	}
	if content.String() != "Hello world" {
		t.Fatalf("stream content = %q, want %q", content.String(), "Hello world")
	}
}

// TestAnthropicChatError 验证非 200 响应返回错误。
func TestAnthropicChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": {"type": "authentication_error", "message": "Invalid API key"}}`)
	}))
	defer server.Close()

	provider := NewAnthropic(Config{
		BaseURL: server.URL,
		APIKey:  "wrong-key",
		Model:   "claude-3-5-sonnet-latest",
	})

	_, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model:    "claude-3-5-sonnet-latest",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should contain 401: %v", err)
	}
}

// TestAnthropicTestConnection 验证连接测试端点使用 GET 方法。
func TestAnthropicTestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	provider := NewAnthropic(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("test connection failed: %v", err)
	}
}
