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

// TestOllamaChat 验证 Ollama Provider 能正确发起请求并解析工具调用响应。
func TestOllamaChat(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"model": "llama3.1",
			"message": {
				"role": "assistant",
				"content": "I will convert it.",
				"tool_calls": [{
					"function": {
						"name": "marcus-img2ascii",
						"arguments": {"input": "/path/img.png"}
					}
				}]
			},
			"done": true,
			"prompt_eval_count": 30,
			"eval_count": 20
		}`)
	}))
	defer server.Close()

	provider := NewOllama(Config{
		BaseURL: server.URL,
		Model:   "llama3.1",
	})

	resp, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model: "llama3.1",
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
	if resp.Usage.TotalTokens != 50 {
		t.Fatalf("total tokens = %d, want 50", resp.Usage.TotalTokens)
	}

	if capturedBody["stream"] != false {
		t.Fatalf("stream = %v, want false", capturedBody["stream"])
	}
}

// TestOllamaChatStream 验证流式响应解析。
func TestOllamaChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flush")
		}

		chunks := []string{
			`{"message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"message":{"role":"assistant","content":" world"},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true}`,
		}
		for _, c := range chunks {
			fmt.Fprintln(w, c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewOllama(Config{
		BaseURL: server.URL,
		Model:   "llama3.1",
	})

	ch, err := provider.ChatStream(context.Background(), &model.ChatRequest{
		Model:    "llama3.1",
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

// TestOllamaChatError 验证非 200 响应返回错误。
func TestOllamaChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error": "model not found"}`)
	}))
	defer server.Close()

	provider := NewOllama(Config{
		BaseURL: server.URL,
		Model:   "unknown",
	})

	_, err := provider.Chat(context.Background(), &model.ChatRequest{
		Model:    "unknown",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
