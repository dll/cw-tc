# 第三课堂（cw-tc）

> 以 AI 为引擎，把"教—学—测—评—证"变成学生自己能跑完的一条闭环。

**第三课堂**是畅网智能科技（CW）的在线自主学习与能力认证平台。
它改造第一课堂（线下传统课程）与第二课堂（学校第二课堂）：
学生以**手机号注册登录**、**身份证号实名**、**自主选课**、**付费解锁后在线学习**、
**全程可与 AI 实时交互答疑**、**导出学习笔记**，并**按专业分类**检索课程；
通过分级测评后由**滁州畅网智能科技有限公司颁发能力认证证书**，编号公开可验真。

- 域名：**cw-tc.ccit-agent.cn**（隶属公司域 `ccit-agent.cn`）
- 上级站点：**CCIT 公司官网 www.ccit-agent.cn**（本产品为其平台入口之一）
- 仓库：**<https://github.com/dll/cw-tc>**（私有）
- 颁发主体：滁州畅网智能科技有限公司
- 上游内容源：`E:\2026-2027\2026-2027-1\AI化教学创新\`（8 门课程资产）

## 目录

```
cw-tc/
├── docs/
│   ├── PRD-第三课堂-v2.md      ← 产品需求文档 v2.21（**当前权威，先读这份**）
│   ├── M2-支付接入要求-v1.md   ← M2 详细要求（资质 / 渠道 / 验收 / 排期）
│   ├── M4-模型选型与AI备案要求-v1.md ← M4 详细要求（选型 / 备案登记 / 生成内容标识）
│   ├── 对外展示内容-v1.md      ← 官网/落地页的文案与数字 v1.0（**数字由脚本生成，勿手改**）
│   ├── API接口参考-v1.md       ← 接口契约 v1.12（前后端并行开发的基线）
│   ├── 数据字典-v1.md          ← 建表与迁移的唯一定义 v1.12
│   ├── 技术方案-v1.md          ← **内部结构**：分层 / 事务与幂等边界 / 任务 / 开发顺序 v1.0
│   ├── 开发准备度评估-v1.md    ← **开工前 Go / No-Go 自评**（输入清单 / 实测 / 未闭合项）v1.0
│   ├── 技术选型-v1.md          ← 技术栈决策 v1.2（含「运行支撑栈」）
│   ├── PRD-第三课堂-v1.md      ← v1.0 立项基线（已归档，仅作对照）
│   └── README.md               文档索引
├── deploy/
│   ├── README.md               部署说明（含密钥与配置约定）
│   ├── .env.example            环境变量模板（**真实值不入库**，服务器放 shared/.env）
│   ├── nginx/cw-tc.conf        域名与反代配置
│   ├── systemd/                服务单元
│   └── scripts/                脚本（含敏感信息扫描）
├── packages/
│   ├── server/                 Go 后端（Gin）· REST API + 对外静态页
│   │   ├── cmd/server/         入口（配置校验失败即拒绝启动）
│   │   ├── migrations/        数据库迁移（**两套方言**：postgres/ 生产权威 · sqlite/ 开发测试）
│   │   └── internal/
│   │       ├── api/            路由 · /health · /public/status · 中间件链（请求ID/脱敏日志）
│   │       ├── config/         配置加载 · 强制密钥校验 · **数据库边界（生产禁 SQLite）**
│   │       ├── platform/       跨模块能力（db · redis · log · redact · audit · clock）
│   │       └── redact/         身份证号 / 手机号脱敏（铁律 3 的代码落点）
│   ├── app/                    Flutter 学生端（Android / Web，一套代码）
│   └── admin/                  Vue 3 管理端（教师工作台 / 课程 / 订单 / 证书 / 广播）
├── .github/workflows/ci.yml    质量门禁（Go / Flutter / Vue / 合规扫描 / 对外文案禁词）
├── tools/
│   ├── gen_public_content.py   由上游 JSON 现场刷新对外数字（**数字不手改**）
│   ├── check_job_map_refs.py   校验就业直达的讲次引用对得上上游目录
│   └── audit_prd_closure.py    **PRD 闭环性审计**（死链 / 文件引用 / 模块落点矩阵）
└── data/                       上游内容的本地同步产物（不入版本库）
```

## 技术栈（已定，见 [`docs/技术选型-v1.md`](docs/技术选型-v1.md)）

| 端 | 选型 |
|---|---|
| **学生端（主）** | **Flutter 3.22+** —— 一套代码出 **Android / Web**（iOS / 鸿蒙不做） |
| **管理端** | **Vue 3 + Vite + Element Plus + Pinia** —— 桌面优先（含教师工作台） |
| **对外静态页** | **Go 模板直出**（验真 / 隐私政策 / 退款政策 / **官网落地页 `/about` `/majors` `/jobs` `/showcase`**，保 SEO 与首屏） |
| **后端** | **Go 1.26 + Gin** + **生产**：PostgreSQL 15（+ **pgvector**） + Redis · **开发/测试：SQLite**（**不依赖本机 PG / Redis**） |
| **支付** | 微信支付 + 支付宝（**扫码 / App 调起**，走自建网关层） |
| **AI** | 自建 **AI 网关** + **国内合规大模型**（RAG 基于课程知识库） |

## 快速开始

> ⚠️ 当前处于 **M0 · 立项与准备** 阶段（见 PRD v2 §13）。
> PRD / 选型 / 仓库 / **代码骨架 / CI 门禁** 已就位；业务功能自 M1 起实现。
> **M2（支付）前置**：支付商户号资质；**M4（AI）前置**：模型服务选型。

```bash
# 1. 读 PRD v2（唯一权威需求）
cat docs/PRD-第三课堂-v2.md

# 2. 读技术选型（栈已定，含多端边界与风险）
cat docs/技术选型-v1.md

# 2.5 跑合规门禁（对外文案禁词 + 敏感信息 + 对外数字一致性）
bash deploy/scripts/scan-claims.sh
bash deploy/scripts/scan-sensitive.sh

# 3. 后端：构建与自检
cd packages/server
make build        # 产出 bin/cw-tc-server
make test         # 全量单测
make check        # fmt + vet + test

# 4. 学生端（Flutter）
cd packages/app && flutter pub get && flutter analyze && flutter test

# 5. 管理端（Vue 3）
cd packages/admin && npm install && npm run typecheck && npm run test
```

> **本地运行后端前**，必须先准备强制密钥（否则服务会**拒绝启动**，这是设计如此）：
> 见 [`deploy/.env.example`](deploy/.env.example) —— `CWTC_ID_CARD_ENC_KEY`（**恰好 32 个字符**）、
> `CWTC_ID_CARD_HMAC_KEY`、`CWTC_SESSION_SECRET`（均 **≥32 个字符**）。
>
> ```bash
> # ⚠️ 校验的是「字符串长度」：ENC 恰好 32 个字符；HMAC / SESSION ≥32 个字符
> export CWTC_ID_CARD_ENC_KEY=$(openssl rand -base64 24)   # 恰好 32 字符
> export CWTC_ID_CARD_HMAC_KEY=$(openssl rand -base64 24)
> export CWTC_SESSION_SECRET=$(openssl rand -base64 24)
> ```
>
> 开工前自查清单见 [`docs/开发准备度评估-v1.md`](docs/开发准备度评估-v1.md)。

## 铁律（**共 15 条**，下列为前 8 条概要；**详版见 [`AGENTS.md`](AGENTS.md)**）

1. **事实源分两类**：`upstream` 课程以 `AI化教学创新/` 为事实源，**代码不写死内容**；
   `platform` 课程（教师建）**平台即事实源**，`sync` 必须按 `source` 过滤。
   （**课程码、专业分类、价格、权益、订单是平台自有数据**，不来自上游 —— PRD v2 §8.3 / §8.4）
2. **数字要有出处**：平台展示的每个数字都能追到上游冻结输出；测不出的标【未取到】；
   **未实测的性能指标不写数**；**AI 的每条回答必须带出处**。
3. **账号红线**：**账号是手机号，身份证号只做实名与证书**；明文绝不落库、展示一律脱敏、
   日志 / URL / 响应体全部过滤；**短信验证码接口必须防刷**（PRD v2 §9.6）；
   **实名是平台认证，文案绝不得写"已核验"**（§9.5）。
4. **证书编号唯一**：唯一约束 + 原子取号，并发取号必须压测。
5. **改文案不改代码**：预期与实测不符时，改文案。
6. **支付走网关**：验签 + 幂等，每日对账差异为 0。
7. **AI 有据才答**：无据明说、越界拒答、测评期关闭、输出带 AI 标识。
8. **角色数据边界**：教师端**只看得到姓名 + 学号 + 本课进度**，
   **身份证号与手机号永不返回**；系统配置页**无密钥值**；高危操作**后端也要二次确认**（PRD v2 §7.9 J-6）。

## 文档

| 文档 | 说明 |
|---|---|
| [`docs/PRD-第三课堂-v2.md`](docs/PRD-第三课堂-v2.md) | 产品需求文档 **v2.21**（**当前权威**，含 **§13 任务驱动开发计划** · **§12.0 三环境矩阵**） |
| [`docs/技术选型-v1.md`](docs/技术选型-v1.md) | 技术栈决策 **v1.3**（主栈 + **运行支撑栈** + **数据库边界**） |
| [`docs/API接口参考-v1.md`](docs/API接口参考-v1.md) | **接口契约 v1.12（前后端并行开工的依据）** |
| [`docs/数据字典-v1.md`](docs/数据字典-v1.md) | **数据字典 v1.12（建表与 migration 的依据）** |
| [`docs/对外展示内容-v1.md`](docs/对外展示内容-v1.md) | 官网与落地页文案与数字 v1.0 |
| [`docs/M2-支付接入要求-v1.md`](docs/M2-支付接入要求-v1.md) | M2 详细要求 v1.0 |
| [`docs/M4-模型选型与AI备案要求-v1.md`](docs/M4-模型选型与AI备案要求-v1.md) | M4 详细要求 v1.0 |
| [`docs/技术方案-v1.md`](docs/技术方案-v1.md) | **内部结构 v1.0**（分层 / 决策 / 开发顺序） |
| [`docs/开发准备度评估-v1.md`](docs/开发准备度评估-v1.md) | **开工前 Go / No-Go 自评 v1.0** |
| `docs/部署运维手册-v1.md` | 部署手册（M6 前建） |

## 相关项目

- `ccit`：畅网官网（Go/Gin + Flutter Web，工程规范来源；**本产品作为其平台入口接入**）
- `cw-vopc`：腾讯云自动部署流水线（发布约定来源）
- `wxx`：蔚小芯（Flutter 六端 + 独立原生小程序，多端实践参照）
- `tsloms`：管理端 Vue 3 + Element Plus 规范来源
- `AI化教学创新`：课程内容事实源
