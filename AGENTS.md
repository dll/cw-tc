# cw-tc 项目开发规范

> 沿用 CCIT / TSLOMS / 蔚小芯 同一套工程规范。**与 `ccit/AGENTS.md` 保持一致。**

## 项目简介

**第三课堂**（cw-tc）：以 AI 为引擎的在线自主学习与能力认证平台。
学生实名注册 → 自主选课学习 → 在线分级测评 → 通过后由畅网智能科技颁发证书。

- `packages/server`：Go 后端（Gin），提供 REST API 与判分/制证引擎
- `packages/app`：**Flutter 学生端**（Android / iOS / Web，一套代码，见 `docs/技术选型-v1.md`）
- `packages/admin`：**Vue 3 管理端**（阅卷台 / 内容管理 / 证书台账，桌面优先）
- `docs/PRD-第三课堂-v1.md`：**唯一权威需求文档**
- `docs/技术选型-v1.md`：**技术栈决策**（选型依据、多端一致边界、风险）

## 语言规范

### 文档语言
- **所有文档内容**：使用中文
- **ASCII 图表**：使用英文，等宽字体，右边绝对对齐
- **图表注解**：使用中文

### 代码语言
- **代码注释**：使用中文
- **变量命名**：使用英文（遵循 Go/JavaScript 规范）
- **函数命名**：使用英文
- **提交信息**：使用中文

### 沟通语言
- **回复用户**：使用中文
- **代码审查**：使用中文
- **文档编写**：使用中文

## 铁律（**违反即打回**）

1. **上游是事实源**：课程/题目/证书元数据来自 `AI化教学创新/`，**代码不得写死**课程名、题目、合格线。
2. **数字要有出处**：每个展示的数字能追到上游冻结输出或 JSON；测不出就标【未取到】，**不许编数**。
3. **身份证号红线**（PRD §9）：
   - 明文**绝不落库**（AES-256-GCM 加密）；
   - 唯一性判断用**单向哈希**；
   - 展示**一律脱敏**（前 6 + 8★ + 后 4）；
   - **日志/埋点/响应体必须过滤**。
4. **证书编号唯一**：数据库唯一约束 + 原子取号；并发取号必须压测。
5. **预期与实测不符时，改文案不改代码**（全系列传统）。

## 质量门禁

```bash
# 后端：测试 + 覆盖率 ≥ 80%
cd packages/server && go test ./... -cover

# 判分器自检（移植自上游，必须全过）
go run ./cmd/sync --self-test

# 内容一致性（资源齐备 + 题库无重复编号）
go run ./cmd/sync --check

# 合规扫描（身份证号不得明文出现在代码/日志）
bash deploy/scripts/scan-sensitive.sh

# 学生端（Flutter）：静态检查 + 单测
cd packages/app && flutter analyze && flutter test

# 管理端（Vue 3）：lint + 单测
cd packages/admin && npm run lint && npm run test
```

## 目录与命名约定

- 后端：`packages/server/` → `cmd/`（入口）、`internal/`（业务，不可外部导入）、`migrations/`（数据库迁移）
- 学生端（Flutter）：`packages/app/lib/` → `pages/`（页面）、`widgets/`（组件）、`api/`（接口封装）、`models/`
- 管理端（Vue 3）：`packages/admin/src/` → `pages/`（页面）、`components/`（组件）、`api/`（接口封装）
- 部署：`deploy/{nginx,systemd,scripts}`
- 文档：`docs/`，命名 `主题-版本.md`

## 分支与提交

- 主分支 `main`（受保护）；开发走 `feat/xxx`、`fix/xxx`
- 提交信息中文，格式：`类型: 说明`（如 `feat: 课程广场接口`）
- **禁止**绕过 hooks（`--no-verify`）、禁止强推 `main`

## 部署边界

- 生产部署**只生成审批请求**，不自动执行（沿用 CCIT 约定）
- 数据卷、密钥（`.env`）**不进版本库**
- 已签发证书、`_ai_draft/` 原始稿**不放公开目录**
