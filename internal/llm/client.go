package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"miniclaw-go/internal/config"
	"miniclaw-go/internal/core"
)

type Client struct {
	cfg        config.LLMConfig
	httpClient *http.Client
}

type Completion struct {
	Content   string
	ToolCalls []core.ToolCall
}

func NewClient(cfg config.LLMConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Complete(ctx context.Context, systemPrompt string, history []core.Message, tools []core.ToolDescriptor) (Completion, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return Completion{}, fmt.Errorf("llm api key is empty; set llm.api_key or OPENAI_API_KEY")
	}

	reqBody := chatRequest{
		Model:       c.cfg.Model,
		Temperature: c.cfg.Temperature,
		Messages:    buildMessages(systemPrompt, history),
		Tools:       buildTools(tools),
		ToolChoice:  "auto",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return Completion{}, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return Completion{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return Completion{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Completion{}, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return Completion{}, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(body))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Completion{}, fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Completion{}, fmt.Errorf("llm response contained no choices")
	}

	msg := parsed.Choices[0].Message
	result := Completion{Content: msg.Content}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, core.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	return result, nil
}

func (c *Client) CompleteText(ctx context.Context, systemPrompt string, history []core.Message) (string, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return "", fmt.Errorf("llm api key is empty; set llm.api_key or OPENAI_API_KEY")
	}

	reqBody := chatRequest{
		Model:       c.cfg.Model,
		Temperature: c.cfg.Temperature,
		Messages:    buildMessages(systemPrompt, history),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, string(body))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm response contained no choices")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		clone := req.Clone(req.Context())
		resp, err := c.httpClient.Do(clone)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableHTTPErr(err) || attempt == 1 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func isRetryableHTTPErr(err error) bool {
	if err == nil {
		return false
	}
	if ue, ok := err.(*url.Error); ok {
		if ue.Timeout() {
			return true
		}
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "timeout") || strings.Contains(text, "tempor") || strings.Contains(text, "handshake")
}

func IsContextWindowError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "context_length_exceeded") ||
		strings.Contains(text, "context window") ||
		strings.Contains(text, "maximum context length") ||
		strings.Contains(text, "token limit") ||
		strings.Contains(text, "too many tokens") ||
		strings.Contains(text, "prompt is too long") ||
		strings.Contains(text, "request too large") ||
		strings.Contains(text, "max_tokens")
}

type chatRequest struct {
	Model       string       `json:"model"`
	Messages    []apiMessage `json:"messages"`
	Tools       []apiTool    `json:"tools,omitempty"`
	ToolChoice  string       `json:"tool_choice,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
}

type apiTool struct {
	Type     string          `json:"type"`
	Function apiToolFunction `json:"function"`
}

type apiToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type apiToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function apiToolCallFunction `json:"function"`
}

type apiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	Choices []struct {
		Message apiMessage `json:"message"`
	} `json:"choices"`
}

func buildMessages(systemPrompt string, history []core.Message) []apiMessage {
	var messages []apiMessage
	messages = append(messages, apiMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, msg := range history {
		apiMsg := apiMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				apiMsg.ToolCalls = append(apiMsg.ToolCalls, apiToolCall{
					ID:   call.ID,
					Type: "function",
					Function: apiToolCallFunction{
						Name:      call.Name,
						Arguments: string(call.Arguments),
					},
				})
			}
		}
		messages = append(messages, apiMsg)
	}
	return messages
}

func buildTools(tools []core.ToolDescriptor) []apiTool {
	var output []apiTool
	for _, tool := range tools {
		output = append(output, apiTool{
			Type: "function",
			Function: apiToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return output
}
