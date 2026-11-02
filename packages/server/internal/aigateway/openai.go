package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider 兼容 OpenAI Chat Completions 协议的供应商（DeepSeek / 百炼兼容模式等）。
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		Client:  &http.Client{Timeout: 45 * time.Second},
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, model string, req Request) (Response, error) {
	if p == nil || p.BaseURL == "" || p.APIKey == "" {
		return Response{}, ErrProviderMissing
	}
	messages := make([]map[string]string, 0, 2)
	if strings.TrimSpace(req.System) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.Prompt})
	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	raw, _ := json.Marshal(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
	}
	request.Header.Set("Authorization", "Bearer "+p.APIKey)
	request.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	resp, err := client.Do(request)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("模型调用失败：HTTP %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil || len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("模型响应格式非法")
	}
	return Response{
		Content:   out.Choices[0].Message.Content,
		TokensIn:  out.Usage.PromptTokens,
		TokensOut: out.Usage.CompletionTokens,
	}, nil
}
