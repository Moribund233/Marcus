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
	"sync"
	"time"

	"Marcus/internal/model"
)

// OpenAICompatible 实现 Provider 接口，用于访问任何 OpenAI 兼容 API。
// 通过配置 BaseURL 和 APIKey 即可对接 OpenAI、DeepSeek、Groq、Together、
// OpenRouter、Ollama v1 等兼容端点。
type OpenAICompatible struct {
	cfg          Config
	client       *http.Client
	cachedModels []model.Model
	modelsOnce   sync.Once
}

// NewOpenAICompatible 创建一个新的 OpenAI 兼容 Provider 实例。
func NewOpenAICompatible(cfg Config) *OpenAICompatible {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &OpenAICompatible{
		cfg: cfg,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回提供者名称。
func (o *OpenAICompatible) Name() string {
	return string(o.cfg.Provider)
}

// Models 返回该提供商支持的模型列表。
//
// 优先级链（三级 fallback）：
//  1. 调用 /v1/models API 获取动态列表，用静态表补充 context length
//  2. API 失败时使用注册表中的静态模型列表
//  3. 兜底：返回当前配置模型的单条目占位（context=128K）
//
// 结果在首次调用后缓存，后续直接返回缓存。
func (o *OpenAICompatible) Models() []model.Model {
	o.modelsOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		models, err := o.fetchModelsFromAPI(ctx)
		if err == nil {
			o.cachedModels = models
			return
		}

		// Level 2: 注册表的静态列表
		entry := DefaultRegistry().Lookup(o.cfg.Provider)
		if entry != nil && len(entry.Models) > 0 {
			o.cachedModels = entry.Models
			return
		}

		// Level 3: 兜底占位
		cl := ContextLengthForModel(o.cfg.Provider, o.cfg.Model)
		if cl == 0 {
			cl = 128000
		}
		o.cachedModels = []model.Model{
			{ID: o.cfg.Model, Name: o.cfg.Model, Context: cl},
		}
	})
	return o.cachedModels
}

// fetchModelsFromAPI 调用 /v1/models 端点获取模型列表。
// 返回的模型会通过静态 fallback 表补充 context length 字段，
// 静态表查不到的模型默认 128K。
func (o *OpenAICompatible) fetchModelsFromAPI(ctx context.Context) ([]model.Model, error) {
	httpReq, err := o.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do models request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, o.readError(resp)
	}

	var apiResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	models := make([]model.Model, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		ctx := ContextLengthForModel(o.cfg.Provider, m.ID)
		if ctx == 0 {
			ctx = 128000
		}
		models = append(models, model.Model{
			ID:      m.ID,
			Name:    m.ID,
			Context: ctx,
		})
	}
	return models, nil
}

// Chat 发送非流式请求。
func (o *OpenAICompatible) Chat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
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
func (o *OpenAICompatible) ChatStream(ctx context.Context, req *model.ChatRequest) (<-chan *model.ChatChunk, error) {
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

	ch := make(chan *model.ChatChunk, 16)
	go o.streamResponse(ctx, resp.Body, ch)
	return ch, nil
}

// TestConnection 测试 API 连通性，调用 /models 端点。
func (o *OpenAICompatible) TestConnection(ctx context.Context) error {
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
func (o *OpenAICompatible) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
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
	if o.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}
	return req, nil
}

// buildRequestBody 将通用 ChatRequest 转换为 OpenAI 请求体。
func (o *OpenAICompatible) buildRequestBody(req *model.ChatRequest, stream bool) ([]byte, error) {
	messages := make([]openAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
			ToolCalls:  toOpenAIToolCalls(m.ToolCalls),
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

// toOpenAIToolCalls 将 model.ToolCall 转换为 openAIToolCall 列表。
func toOpenAIToolCalls(tcs []model.ToolCall) []openAIToolCall {
	if len(tcs) == 0 {
		return nil
	}
	result := make([]openAIToolCall, 0, len(tcs))
	for _, tc := range tcs {
		result = append(result, openAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openAIToolFunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return result
}

// parseResponse 将 OpenAI 响应解析为通用 ChatResponse。
func (o *OpenAICompatible) parseResponse(apiResp *openAIChatResponse) (*model.ChatResponse, error) {
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
func (o *OpenAICompatible) streamResponse(ctx context.Context, body io.ReadCloser, ch chan<- *model.ChatChunk) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	send := func(chunk *model.ChatChunk) bool {
		select {
		case ch <- chunk:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			send(&model.ChatChunk{Done: true})
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			send(&model.ChatChunk{Error: fmt.Errorf("decode stream chunk: %w", err), Done: true})
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
		if !send(item) {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		send(&model.ChatChunk{Error: fmt.Errorf("read stream: %w", err), Done: true})
		return
	}

	send(&model.ChatChunk{Done: true})
}

// readError 读取非 200 响应的错误信息。
func (o *OpenAICompatible) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("api error status %d: %s", resp.StatusCode, string(body))
}

// --- OpenAI API 结构 ---

// openAIModelsResponse 表示 GET /v1/models 的响应结构。
type openAIModelsResponse struct {
	Object string          `json:"object"`
	Data   []openAIModelRef `json:"data"`
}

type openAIModelRef struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

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
