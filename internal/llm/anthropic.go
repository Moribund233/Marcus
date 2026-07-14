// Package llm 实现了 Anthropic Claude Provider。
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

// Anthropic 实现 Provider 接口，用于访问 Anthropic Messages API.
type Anthropic struct {
	cfg    Config
	client *http.Client
}

// NewAnthropic 创建一个新的 Anthropic Provider 实例。
func NewAnthropic(cfg Config) *Anthropic {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com/v1"
	}
	return &Anthropic{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回提供者名称。
func (a *Anthropic) Name() string {
	return string(model.LLMProviderAnthropic)
}

// Models 返回 Anthropic 支持的常见模型列表。
//
// 优先级链：
//  1. 数据库缓存（由用户手动触发刷新）
//  2. 静态硬编码列表
func (a *Anthropic) Models() []model.Model {
	// Level 1: 数据库缓存
	if store := GetModelStore(); store != nil {
		if cached, err := store.GetModels(a.cfg.Provider); err == nil && len(cached) > 0 {
			return cached
		}
	}

	// Level 2: 静态列表
	return []model.Model{
		{ID: "claude-3-5-sonnet-latest", Name: "Claude 3.5 Sonnet", Context: 200000},
		{ID: "claude-3-opus-latest", Name: "Claude 3 Opus", Context: 200000},
		{ID: "claude-3-haiku-latest", Name: "Claude 3 Haiku", Context: 200000},
	}
}

// Chat 发送非流式请求。
func (a *Anthropic) Chat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	body, systemPrompt, err := a.buildRequestBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := a.newRequest(ctx, http.MethodPost, "/messages", body)
	if err != nil {
		return nil, err
	}
	if systemPrompt != "" {
		// Anthropic 将 system 放在请求体顶层，这里在发送前注入
		bodyMap := make(map[string]interface{})
		if err := json.Unmarshal(body, &bodyMap); err != nil {
			return nil, fmt.Errorf("unmarshal body for system: %w", err)
		}
		bodyMap["system"] = systemPrompt
		body, err = json.Marshal(bodyMap)
		if err != nil {
			return nil, fmt.Errorf("remarshal body: %w", err)
		}
		httpReq.Body = io.NopCloser(bytes.NewReader(body))
		httpReq.ContentLength = int64(len(body))
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, a.readError(resp)
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return a.parseResponse(&apiResp)
}

// ChatStream 发送流式请求。
func (a *Anthropic) ChatStream(ctx context.Context, req *model.ChatRequest) (<-chan *model.ChatChunk, error) {
	body, systemPrompt, err := a.buildRequestBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if systemPrompt != "" {
		bodyMap := make(map[string]interface{})
		if err := json.Unmarshal(body, &bodyMap); err != nil {
			return nil, fmt.Errorf("unmarshal body for system: %w", err)
		}
		bodyMap["system"] = systemPrompt
		body, err = json.Marshal(bodyMap)
		if err != nil {
			return nil, fmt.Errorf("remarshal body: %w", err)
		}
	}

	httpReq, err := a.newRequest(ctx, http.MethodPost, "/messages", body)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, a.readError(resp)
	}

	ch := make(chan *model.ChatChunk, 16)
	go a.streamResponse(ctx, resp.Body, ch)
	return ch, nil
}

// TestConnection 测试 API 连通性，调用 /models 端点。
func (a *Anthropic) TestConnection(ctx context.Context) error {
	httpReq, err := a.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return err
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("test connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return a.readError(resp)
	}
	return nil
}

// newRequest 创建带有认证头的 HTTP 请求。
func (a *Anthropic) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	url := strings.TrimRight(a.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	return req, nil
}

// buildRequestBody 将通用 ChatRequest 转换为 Anthropic Messages API 请求体。
// 返回请求体 JSON 和提取出的 system prompt（Anthropic 要求 system 在顶层）。
func (a *Anthropic) buildRequestBody(req *model.ChatRequest, stream bool) ([]byte, string, error) {
	var systemPrompt string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == model.RoleSystem {
			// Anthropic 将 system 消息放在请求体顶层 system 字段
			systemPrompt = m.Content
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = a.cfg.Model
	}
	body := anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: defaultIfZero(req.MaxTokens, 4096),
		Stream:    stream,
	}

	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
		body.Tools = tools
	}

	data, err := json.Marshal(body)
	return data, systemPrompt, err
}

// parseResponse 将 Anthropic 响应解析为通用 ChatResponse。
func (a *Anthropic) parseResponse(apiResp *anthropicResponse) (*model.ChatResponse, error) {
	result := &model.ChatResponse{
		Usage: model.Usage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
	}

	var contentParts []string
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			contentParts = append(contentParts, block.Text)
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, model.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(args),
				},
			})
		}
	}
	result.Content = strings.Join(contentParts, "")

	return result, nil
}

// streamResponse 解析 Anthropic SSE 流。
func (a *Anthropic) streamResponse(ctx context.Context, body io.ReadCloser, ch chan<- *model.ChatChunk) {
	defer close(ch)
	defer body.Close()

	send := func(chunk *model.ChatChunk) bool {
		select {
		case ch <- chunk:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var buffer strings.Builder
	var pendingTool *model.ToolCall

	flushTool := func() bool {
		if pendingTool != nil {
			if !send(&model.ChatChunk{ToolCalls: []model.ToolCall{*pendingTool}}) {
				return false
			}
			pendingTool = nil
		}
		return true
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			flushTool()
			send(&model.ChatChunk{Done: true})
			return
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			send(&model.ChatChunk{Error: fmt.Errorf("decode stream chunk: %w", err), Done: true})
			return
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				buffer.WriteString(event.Delta.Text)
			} else if event.Delta.Type == "input_json_delta" {
				if pendingTool == nil {
					pendingTool = &model.ToolCall{Type: "function"}
				}
				pendingTool.Function.Arguments += event.Delta.PartialJSON
			}
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				if !flushTool() {
					return
				}
				pendingTool = &model.ToolCall{
					ID:   event.ContentBlock.ID,
					Type: "function",
					Function: model.ToolCallFunction{
						Name: event.ContentBlock.Name,
					},
				}
			}
		case "message_delta":
			if event.Usage.OutputTokens > 0 {
				if !send(&model.ChatChunk{
					Usage: &model.Usage{CompletionTokens: event.Usage.OutputTokens},
				}) {
					return
				}
			}
		case "message_stop":
			flushTool()
			send(&model.ChatChunk{Done: true})
			return
		}

		if buffer.Len() > 0 {
			if !send(&model.ChatChunk{Content: buffer.String()}) {
				return
			}
			buffer.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		send(&model.ChatChunk{Error: fmt.Errorf("read stream: %w", err), Done: true})
		return
	}

	flushTool()
	send(&model.ChatChunk{Done: true})
}

// readError 读取非 200 响应的错误信息。
func (a *Anthropic) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("api error status %d: %s", resp.StatusCode, string(body))
}

func defaultIfZero(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// --- Anthropic API 结构 ---

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type anthropicStreamEvent struct {
	Type         string                `json:"type"`
	Delta        anthropicDelta        `json:"delta,omitempty"`
	ContentBlock anthropicContentBlock `json:"content_block,omitempty"`
	Usage        anthropicStreamUsage  `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type anthropicStreamUsage struct {
	OutputTokens int `json:"output_tokens,omitempty"`
}
