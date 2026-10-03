# 第三课堂（cw-tc）后端 · 常用命令
#
# 质量门禁与 CI 保持一致（见 .github/workflows/ci.yml）。

BINARY      := cw-tc-server
CMD         := ./cmd/server
COVER_FILE  := coverage.out
COVER_MIN   := 80

.PHONY: help
help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## 整理依赖
	go mod tidy

.PHONY: fmt
fmt: ## 格式化（CI 会校验 gofmt -l 为 0）
	gofmt -w .

.PHONY: vet
vet: ## 静态检查
	go vet ./...

.PHONY: build
build: ## 本机构建
	go build -trimpath -ldflags "-s -w -X github.com/cw-tc/server/internal/api.Version=$(shell git rev-parse --short HEAD 2>/dev/null || echo dev)" -o bin/$(BINARY) $(CMD)

.PHONY: run
run: ## 本地运行（需先按 deploy/.env.example 配好强制密钥）
	go run $(CMD)

.PHONY: test
test: ## 全量单测
	go test ./... -count=1

.PHONY: cover
cover: ## 单测 + 覆盖率（阈值 $(COVER_MIN)%）
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=$(COVER_FILE)
	@go tool cover -func=$(COVER_FILE) | tail -1

.PHONY: check
check: fmt vet test ## 提交前自检
