// Package llm 实现了 Ollama 本地模型 Provider。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"Marcus/internal/model"
)

// Ollama 实现 Provider 接口，用于访问本地 Ollama 服务。
type Ollama struct {
	cfg    Config
	client *http.Client
}

// NewOllama 创建一个新的 Ollama Provider 实例。
func NewOllama(cfg Config) *Ollama {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	return &Ollama{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回提供者名称。
func (o *Ollama) Name() string {
	return string(model.LLMProviderOllama)
}

// Models 返回 Ollama 本地常见的模型列表。
// 实际可用模型需通过 /api/tags 动态获取。
func (o *Ollama) Models() []model.Model {
	return []model.Model{
		{ID: "llama3.1", Name: "Llama 3.1", Context: 128000},
		{ID: "qwen2.5", Name: "Qwen 2.5", Context: 128000},
		{ID: "mistral", Name: "Mistral", Context: 32768},
	}
}

// Chat 发送非流式请求。
func (o *Ollama) Chat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	body, err := o.buildRequestBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := o.newRequest(ctx, http.MethodPost, "/api/chat", body)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, o.readError(resp)
	}

	var apiResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return o.parseResponse(&apiResp)
}

// ChatStream 发送流式请求。
func (o *Ollama) ChatStream(ctx context.Context, req *model.ChatRequest) (<-chan *model.ChatChunk, error) {
	body, err := o.buildRequestBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := o.newRequest(ctx, http.MethodPost, "/api/chat", body)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, o.readError(resp)
	}

	ch := make(chan *model.ChatChunk)
	go o.streamResponse(resp.Body, ch)
	return ch, nil
}

// TestConnection 测试本地 Ollama 服务是否可达，调用 /api/tags 端点。
func (o *Ollama) TestConnection(ctx context.Context) error {
	httpReq, err := o.newRequest(ctx, http.MethodGet, "/api/tags", nil)
	if err != nil {
		return err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("test connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return o.readError(resp)
	}
	return nil
}

// newRequest 创建 HTTP 请求。
func (o *Ollama) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	url := strings.TrimRight(o.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// buildRequestBody 将通用 ChatRequest 转换为 Ollama /api/chat 请求体。
func (o *Ollama) buildRequestBody(req *model.ChatRequest, stream bool) ([]byte, error) {
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
		Options:  map[string]interface{}{},
	}

	if req.Temperature > 0 {
		body.Options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body.Options["num_predict"] = req.MaxTokens
	}

	if len(req.Tools) > 0 {
		tools := make([]ollamaTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			})
		}
		body.Tools = tools
	}

	return json.Marshal(body)
}

// parseResponse 将 Ollama 响应解析为通用 ChatResponse。
func (o *Ollama) parseResponse(apiResp *ollamaChatResponse) (*model.ChatResponse, error) {
	result := &model.ChatResponse{
		Content: apiResp.Message.Content,
	}

	for _, tc := range apiResp.Message.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		result.ToolCalls = append(result.ToolCalls, model.ToolCall{
			Type: "function",
			Function: model.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: string(args),
			},
		})
	}

	// Ollama 非流式响应可能包含 prompt_eval_count / eval_count。
	if apiResp.PromptEvalCount > 0 || apiResp.EvalCount > 0 {
		result.Usage = model.Usage{
			PromptTokens:     apiResp.PromptEvalCount,
			CompletionTokens: apiResp.EvalCount,
			TotalTokens:      apiResp.PromptEvalCount + apiResp.EvalCount,
		}
	}

	return result, nil
}

// streamResponse 解析 Ollama 流式响应（每行一个 JSON）。
func (o *Ollama) streamResponse(body io.ReadCloser, ch chan<- *model.ChatChunk) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var chunk ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			ch <- &model.ChatChunk{Error: fmt.Errorf("decode stream chunk: %w", err), Done: true}
			return
		}

		item := &model.ChatChunk{Done: chunk.Done}
		item.Content = chunk.Message.Content

		for _, tc := range chunk.Message.ToolCalls {
			args, _ := json.Marshal(tc.Function.Arguments)
			item.ToolCalls = append(item.ToolCalls, model.ToolCall{
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: string(args),
				},
			})
		}

		ch <- item
		if chunk.Done {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- &model.ChatChunk{Error: fmt.Errorf("read stream: %w", err), Done: true}
		return
	}

	ch <- &model.ChatChunk{Done: true}
}

// readError 读取非 200 响应的错误信息。
func (o *Ollama) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("api error status %d: %s", resp.StatusCode, string(body))
}

// --- Ollama API 结构 ---

type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Tools    []ollamaTool           `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaChatResponse struct {
	Model           string                `json:"model"`
	CreatedAt       string                `json:"created_at"`
	Message         ollamaResponseMessage `json:"message"`
	Done            bool                  `json:"done"`
	PromptEvalCount int                   `json:"prompt_eval_count"`
	EvalCount       int                   `json:"eval_count"`
}

type ollamaResponseMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}
