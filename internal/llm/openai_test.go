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

// TestOpenAIChat 验证 OpenAI Provider 能正确发起请求并解析工具调用响应。
func TestOpenAIChat(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected authorization: %s", auth)
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "I will convert it.",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "marcus-img2ascii",
							"arguments": "{\"input\":\"/path/img.png\"}"
						}
					}]
				}
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`)
	}))
	defer server.Close()

	provider := NewOpenAI(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o",
	})

	resp, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
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
	if resp.Usage.TotalTokens != 30 {
		t.Fatalf("total tokens = %d, want 30", resp.Usage.TotalTokens)
	}

	// 验证请求体结构
	if capturedBody["model"] != "gpt-4o" {
		t.Fatalf("request model = %v, want gpt-4o", capturedBody["model"])
	}
	// 非流式请求可以不包含 stream 字段，或明确为 false
	if stream, ok := capturedBody["stream"].(bool); ok && stream {
		t.Fatal("non-stream request should not have stream=true")
	}
}

// TestOpenAIChatStream 验证流式响应解析。
func TestOpenAIChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flush")
		}

		chunks := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		}
		for _, c := range chunks {
			fmt.Fprintln(w, c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewOpenAI(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o",
	})

	ch, err := provider.ChatStream(context.Background(), &model.ChatRequest{
		Model:    "gpt-4o",
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

// TestOpenAIChatError 验证非 200 响应返回错误。
func TestOpenAIChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": {"message": "Invalid API key"}}`)
	}))
	defer server.Close()

	provider := NewOpenAI(Config{
		BaseURL: server.URL,
		APIKey:  "wrong-key",
		Model:   "gpt-4o",
	})

	_, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should contain 401: %v", err)
	}
}

// TestOpenAITestConnection 验证连接测试端点使用 GET 方法。
func TestOpenAITestConnection(t *testing.T) {
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

	provider := NewOpenAI(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})

	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("test connection failed: %v", err)
	}
}

// TestOpenAIChat_ToolResultMessage 验证 tool 结果消息携带 tool_call_id。
func TestOpenAIChat_ToolResultMessage(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"}}],"usage":{}}`)
	}))
	defer server.Close()

	provider := NewOpenAI(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o",
	})

	_, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "hi"},
			{Role: model.RoleTool, Name: "test", Content: "result", ToolCallID: "call_1"},
		},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	msgs, ok := capturedBody["messages"].([]interface{})
	if !ok || len(msgs) < 2 {
		t.Fatalf("unexpected request messages: %v", capturedBody["messages"])
	}
	toolMsg, ok := msgs[1].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected tool message type")
	}
	if toolMsg["tool_call_id"] != "call_1" {
		t.Errorf("tool_call_id = %v, want call_1", toolMsg["tool_call_id"])
	}
}
