// Package llm 实现了 OpenAI 兼容的 LLM Provider。
//
// 该实现同时支持 OpenAI 官方 API 以及任何提供兼容 /chat/completions 端点的服务。
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

// OpenAI 实现 Provider 接口，用于访问 OpenAI 兼容 API。
type OpenAI struct {
	cfg    Config
	client *http.Client
}

// NewOpenAI 创建一个新的 OpenAI Provider 实例。
func NewOpenAI(cfg Config) *OpenAI {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &OpenAI{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回提供者名称。
func (o *OpenAI) Name() string {
	return string(model.LLMProviderOpenAI)
}

// Models 返回 OpenAI 支持的常见模型列表。
func (o *OpenAI) Models() []model.Model {
	return []model.Model{
		{ID: "gpt-4o", Name: "GPT-4o", Context: 128000},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Context: 128000},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Context: 128000},
		{ID: "gpt-4", Name: "GPT-4", Context: 8192},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Context: 16385},
	}
}

// Chat 发送非流式请求。
func (o *OpenAI) Chat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	body, err := o.buildRequestBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := o.newRequest(ctx, http.MethodPost, "/chat/completions", body)
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

	var apiResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return o.parseResponse(&apiResp)
}

// ChatStream 发送流式请求。
func (o *OpenAI) ChatStream(ctx context.Context, req *model.ChatRequest) (<-chan *model.ChatChunk, error) {
	body, err := o.buildRequestBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := o.newRequest(ctx, http.MethodPost, "/chat/completions", body)
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

// TestConnection 测试 API 连通性，调用 /models 端点。
func (o *OpenAI) TestConnection(ctx context.Context) error {
	httpReq, err := o.newRequest(ctx, http.MethodGet, "/models", nil)
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

// newRequest 创建带有认证头的 HTTP 请求。
func (o *OpenAI) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
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
	req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	return req, nil
}

// buildRequestBody 将通用 ChatRequest 转换为 OpenAI 请求体。
func (o *OpenAI) buildRequestBody(req *model.ChatRequest, stream bool) ([]byte, error) {
	messages := make([]openAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		})
	}

	body := openAIChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	if len(req.Tools) > 0 {
		tools := make([]openAITool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, openAITool{
				Type: "function",
				Function: openAIToolFunction{
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

// parseResponse 将 OpenAI 响应解析为通用 ChatResponse。
func (o *OpenAI) parseResponse(apiResp *openAIChatResponse) (*model.ChatResponse, error) {
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	choice := apiResp.Choices[0]
	result := &model.ChatResponse{
		Content: choice.Message.Content,
		Usage: model.Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, model.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: model.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return result, nil
}

// streamResponse 解析 SSE 流并写入通道。
func (o *OpenAI) streamResponse(body io.ReadCloser, ch chan<- *model.ChatChunk) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- &model.ChatChunk{Done: true}
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- &model.ChatChunk{Error: fmt.Errorf("decode stream chunk: %w", err), Done: true}
			return
		}

		item := &model.ChatChunk{}
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			item.Content = delta.Content
			for _, tc := range delta.ToolCalls {
				item.ToolCalls = append(item.ToolCalls, model.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: model.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}
		ch <- item
	}

	if err := scanner.Err(); err != nil {
		ch <- &model.ChatChunk{Error: fmt.Errorf("read stream: %w", err), Done: true}
		return
	}

	ch <- &model.ChatChunk{Done: true}
}

// readError 读取非 200 响应的错误信息。
func (o *OpenAI) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("api error status %d: %s", resp.StatusCode, string(body))
}

// --- OpenAI API 结构 ---

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolFunctionCall `json:"function"`
}

type openAIToolFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string           `json:"role,omitempty"`
			Content   string           `json:"content,omitempty"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}
