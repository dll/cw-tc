package aigateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeProvider struct {
	model string
	resp  Response
	err   error
}

func (f *fakeProvider) Chat(_ context.Context, model string, _ Request) (Response, error) {
	f.model = model
	return f.resp, f.err
}

func TestFilingTraceability(t *testing.T) {
	fast := &fakeProvider{resp: Response{Content: "fast"}}
	strong := &fakeProvider{resp: Response{Content: "strong"}}
	gateway := New([]Model{
		{Alias: "assistant.fast", Provider: "p", Model: "m-fast", ModelVersion: "v1", FilingNo: "FIL-1"},
		{Alias: "assistant.strong", Provider: "q", Model: "m-strong", ModelVersion: "v2", FilingNo: "FIL-2"},
	}, map[string]Provider{"p": fast, "q": strong})
	resp, err := gateway.Complete(context.Background(), "assistant.fast", Request{Prompt: "q"})
	if err != nil || fast.model != "m-fast" || resp.FilingNo != "FIL-1" || resp.ModelVersion != "v1" {
		t.Fatalf("fast 路由 / 留痕异常: %+v %v", resp, err)
	}
	resp, err = gateway.Complete(context.Background(), "assistant.strong", Request{Prompt: "q"})
	if err != nil || strong.model != "m-strong" || resp.FilingNo != "FIL-2" {
		t.Fatalf("strong 路由 / 留痕异常: %+v %v", resp, err)
	}
}

func TestUnregisteredModelRejected(t *testing.T) {
	provider := &fakeProvider{resp: Response{Content: "x"}}
	gateway := New([]Model{
		{Alias: "assistant.fast", Provider: "p", Model: "m"},
		{Alias: "assistant.strong", Provider: "p", Model: "m", FilingNo: "F"},
		{Alias: "assistant.noprovider", Provider: "missing", Model: "m", FilingNo: "F"},
	}, map[string]Provider{"p": provider})
	if _, err := gateway.Complete(context.Background(), "assistant.unknown", Request{}); !errors.Is(err, ErrModelUnregistered) {
		t.Fatalf("未登记别名应拒绝: %v", err)
	}
	if _, err := gateway.Complete(context.Background(), "assistant.fast", Request{}); !errors.Is(err, ErrFilingRequired) {
		t.Fatalf("缺备案号应拒绝: %v", err)
	}
	if _, err := gateway.Complete(context.Background(), "assistant.noprovider", Request{}); !errors.Is(err, ErrProviderMissing) {
		t.Fatalf("缺供应商应拒绝: %v", err)
	}
	if _, err := New(nil, nil).Complete(context.Background(), "x", Request{}); !errors.Is(err, ErrModelUnregistered) {
		t.Fatalf("空网关应拒绝: %v", err)
	}
}

func TestOpenAIProviderSuccessAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("请求异常: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "m1" {
			t.Fatalf("模型参数异常: %v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "答案"}}},
			"usage":   map[string]int{"prompt_tokens": 3, "completion_tokens": 4},
		})
	}))
	defer server.Close()
	provider := NewOpenAIProvider(server.URL, "key")
	resp, err := provider.Chat(context.Background(), "m1", Request{System: "s", Prompt: "p", Temperature: 0.2, MaxTokens: 10})
	if err != nil || resp.Content != "答案" || resp.TokensIn != 3 || resp.TokensOut != 4 {
		t.Fatalf("调用结果异常: %+v %v", resp, err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	if _, err := NewOpenAIProvider(bad.URL, "k").Chat(context.Background(), "m", Request{}); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("非 2xx 应报错: %v", err)
	}
	invalid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer invalid.Close()
	if _, err := NewOpenAIProvider(invalid.URL, "k").Chat(context.Background(), "m", Request{}); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	if _, err := NewOpenAIProvider("", "").Chat(context.Background(), "m", Request{}); !errors.Is(err, ErrProviderMissing) {
		t.Fatalf("未配置供应商应拒绝: %v", err)
	}
}
