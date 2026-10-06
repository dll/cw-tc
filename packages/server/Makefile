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

.PHONY: fmt-check
fmt-check: ## 检查格式（不修改文件）
	@test -z "$$(gofmt -l .)" || { echo "存在未格式化的 Go 文件" >&2; gofmt -l .; exit 1; }

.PHONY: cover
cover: ## 单测 + 覆盖率（阈值 $(COVER_MIN)%）
	go test ./... -count=1 -covermode=set -coverpkg=./... -coverprofile=$(COVER_FILE)
	@go tool cover -func=$(COVER_FILE) | tail -1

.PHONY: migration-test
migration-test: ## 在内存 SQLite 中执行迁移测试
	go test ./migrations/... -count=1 -v

.PHONY: compliance
compliance: ## 执行仓库合规扫描
	bash ../../deploy/scripts/scan-sensitive.sh
	bash ../../deploy/scripts/scan-claims.sh

.PHONY: check
check: fmt-check vet test migration-test ## 提交前自检（不连接外部数据库/缓存）
