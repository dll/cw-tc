// Package aigateway 是业务与模型之间的唯一入口。
//
// 硬约束（AGENTS 铁律 14 / PRD §7.5、§7.16）：
//   - 业务只认模型别名（assistant.fast / assistant.strong），换厂商只改配置；
//   - 模型必须在“已备案清单”内，且每次调用都带 model / model_version / filing_no；
//   - 未备案或缺少备案号一律拒绝调用（fail-closed），不得静默降级。
package aigateway

import (
	"context"
	"errors"
)

var (
	ErrModelUnregistered = errors.New("模型未备案或不在已备案清单")
	ErrFilingRequired    = errors.New("模型缺少备案号 / 上线编号")
	ErrProviderMissing   = errors.New("模型供应商未配置")
	ErrEmptyResponse     = errors.New("模型返回空内容")
)

type Model struct {
	Alias        string
	Provider     string
	Model        string
	ModelVersion string
	FilingNo     string
}

type Request struct {
	System      string
	Prompt      string
	Temperature float64
	MaxTokens   int
}

type Response struct {
	Content      string
	TokensIn     int
	TokensOut    int
	Alias        string
	Provider     string
	Model        string
	ModelVersion string
	FilingNo     string
}

type Provider interface {
	Chat(ctx context.Context, model string, req Request) (Response, error)
}

type Gateway struct {
	models    map[string]Model
	providers map[string]Provider
}

func New(models []Model, providers map[string]Provider) *Gateway {
	registry := make(map[string]Model, len(models))
	for _, model := range models {
		registry[model.Alias] = model
	}
	return &Gateway{models: registry, providers: providers}
}

// Complete 通过别名调用模型，并返回完整留痕字段（model / model_version / filing_no）。
func (g *Gateway) Complete(ctx context.Context, alias string, req Request) (Response, error) {
	if g == nil {
		return Response{}, ErrModelUnregistered
	}
	model, ok := g.models[alias]
	if !ok || model.Model == "" {
		return Response{}, ErrModelUnregistered
	}
	if model.FilingNo == "" {
		return Response{}, ErrFilingRequired
	}
	provider, ok := g.providers[model.Provider]
	if !ok || provider == nil {
		return Response{}, ErrProviderMissing
	}
	resp, err := provider.Chat(ctx, model.Model, req)
	if err != nil {
		return Response{}, err
	}
	if resp.Content == "" {
		return Response{}, ErrEmptyResponse
	}
	resp.Alias = alias
	resp.Provider = model.Provider
	resp.Model = model.Model
	resp.ModelVersion = model.ModelVersion
	if resp.ModelVersion == "" {
		resp.ModelVersion = model.Model
	}
	resp.FilingNo = model.FilingNo
	return resp, nil
}
