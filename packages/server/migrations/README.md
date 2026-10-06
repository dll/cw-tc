# migrations · 数据库迁移

> **字段、约束、索引与切分以 `docs/数据字典-v1.md` §8 为唯一来源。** 本目录只放它的落地 SQL。

## 数据库边界（2026-10-06 明确）

| 环境 | 引擎 | 说明 |
|---|---|---|
| **开发 / 测试** | **SQLite** | 纯 Go 驱动（`modernc.org/sqlite`，无 CGO）。**不依赖本机 PostgreSQL / Redis**，也不需要起任何本机服务 |
| **生产** | **腾讯云 PostgreSQL** | 沿用 CCIT 的服务器、部署与环境变量约定；库与账号由部署环节创建 |

- **不新建服务器 / 数据库 / 本机服务**；**不读取或改动 CCIT 的真实生产数据**。
- 本机曾有的 PostgreSQL 只作「存在性探测」，**后续不再使用**。
- ⚠️ **`DEPLOY_ENV=production` 时驱动若为 `sqlite`，服务拒绝启动**（`internal/config` 校验）——
  拿开发库跑生产会造成「测试通过、线上丢数据」。

## 目录布局：**两套方言，静态并存**

```
migrations/
├── postgres/   ← 生产唯一权威（对应数据字典 §8 的切分）
│   ├── 0001_enable_extensions.sql      （pgvector）
│   └── 0002_identity.sql … 0005_exam_cert.sql
└── sqlite/     ← 开发与测试（由 postgres/ 机械翻译，表与列一一对应）
    └── 0002_identity.sql … 0005_exam_cert.sql
```

**为什么是两套文件，而不是"运行时翻译"**：
翻译器要在运行时把 PG 语法改写成 SQLite 语法，**改错是静默的**（跑过了、但类型/约束悄悄变了）。
两套静态文件**可以逐行 review**，且由门禁保证不会分叉。

**SQLite 侧没有 `0001`**：`pgvector` 仅 PostgreSQL 有；开发/测试的向量检索走**暴力检索兜底**
（见 `docs/技术方案-v1.md`）。这条豁免**显式登记在测试里**（`dialectException`，带理由），不是自动放行。

## 一致性门禁（`migrations_test.go`，全部为**真跑**而非字符串检查）

| # | 门禁 | 抓什么 |
|---|---|---|
| 1 | **SQLite 迁移真的执行**：内存库跑完整套 → 关键表 / 部分唯一索引必须建出来 → **再跑一遍验证幂等** | 「勾了 DoD 其实没跑」这类假通过 |
| 2 | **外键真的生效**：插入不存在的 `student_id` 必须失败 | SQLite 默认**不** enforce 外键，与生产行为不一致 |
| 3 | **两套方言结构一致**：逐文件比对 **表名与列名**（顺序无关） | 手工改了单侧、两套悄悄分叉 |
| 4 | **方言 lint**：`sqlite/` 不许出现 `BIGSERIAL` / `TIMESTAMPTZ` / `BYTEA` / `JSONB` / `::jsonb` / `DEFAULT now()` / `CREATE EXTENSION`；`postgres/` 不许出现 `AUTOINCREMENT` / `PRAGMA` / `BLOB` | 把另一边的语法写进来 |
| 5 | 幂等声明 + 无密钥材料 | `IF NOT EXISTS`；`CREATE ROLE` / `PASSWORD '` 等 |

> **门禁必须自己验证一次**：把 `sqlite/0002` 的某个列改名 → 门禁 3 会**准确报出是哪张表哪一列**（已实测）。

## 与数据字典的对齐（**v2.25 重建**）

> **字典是建表唯一依据。** 重建后，**`0002~0005` 的表与列逐字段对齐字典 §2~§5**。
>
> ⚠️ **但字典写的是"终态"**（含 `0010/0011/0015/0018/0019` 才新增的列）——
> 所以**不能把终态列一次性塞进早期迁移**（那会让后续迁移的 `ALTER` 全部撞车）。
> 做法：**本批次只建基列**，后续迁移新增的列在迁移文件里**显式登记**：

```sql
-- @deferred student: status_reason_kind@0010 status_note@0010 status_until@0010 status_by@0010
-- @note student: 0010：`student.phone` 的普通唯一索引改为部分唯一
```

**门禁**（`tools/audit_prd_closure.py`）：**「本批次基列 ∪ `@deferred` 登记的列」== 字典列，不多不少**。
→ 所以**改表必须同时改这三处之一**：字典 · 迁移 · `@deferred`（漏一处门禁就红）。

## 类型映射（`postgres/` → `sqlite/`，机械翻译，不可手改）

| PostgreSQL | SQLite |
|---|---|
| `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`（字典口径） | `INTEGER PRIMARY KEY AUTOINCREMENT` |
| `TIMESTAMPTZ` | `TEXT`（ISO-8601 文本） |
| `BYTEA` | `BLOB` |
| `JSONB` / `'{}'::jsonb` | `TEXT` / `'{}'` |
| `DATE` / `INET` | `TEXT` |
| `DEFAULT now()` | `DEFAULT CURRENT_TIMESTAMP` |

部分唯一索引（`... WHERE ...`）**SQLite 原生支持**，故 `uq_exam_ongoing` / `uq_certificate_active` 等两套一致。

## 命名与顺序

- 文件名：`NNNN_<snake_case_主题>.sql`，四位编号，**只增不改**。
- **已执行过的迁移不得修改**（改了不会重跑，只会让环境之间产生"看不见的差异"）。要改就**新增编号**。
- 编号与内容对照见 `docs/数据字典-v1.md` §8（当前 **0001~0019**，已落地 **0001~0005**）。

## 执行

```bash
# 开发 / 测试：内存 SQLite，跑整套迁移（含幂等与外键检查）
cd packages/server && go test ./migrations/... -count=1 -v

# 生产：**发布流水线内执行，先迁移后切流量**（PRD §12.3）
#   迁移必须可重复执行；生产驱动为 postgres（见 deploy/.env.example）
```

- **回滚**：DML 型回滚走业务接口（如撤证）；**DDL 不写 down 脚本** ——
  生产回滚靠「备份恢复」，不靠反向 DDL（反向 DDL 在真实数据面前经常是错的）。

## 铁律

1. **迁移必须幂等**：`CREATE TABLE IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`。
2. **不删列、不改类型**（首发阶段）：加列给默认值；确需变更走"新列 + 回填 + 切换"三步。
3. **强制项靠代码启动校验，不靠迁移**：加密 / HMAC / 会话密钥缺失则服务**拒绝启动**
   （`internal/config`），**不在迁移里给默认密钥**（那等于把密钥写进版本库）。
4. **种子数据用 upsert**，保证重跑结果一致。
5. **一次迁移只做一件事**，便于定位与手工补跑。
6. **改 `postgres/` 就必须同步改 `sqlite/`**（或登记豁免），否则门禁 3 会红。

## 目录现状（2026-10-06）

| 编号 | postgres/ | sqlite/ |
|---|---|---|
| `0001_enable_extensions` | ✅ | — （豁免：pgvector） |
| `0002_identity` | ✅ | ✅ |
| `0003_catalog` | ✅ | ✅ |
| `0004_commerce` | ✅ | ✅ |
| `0005_exam_cert` | ✅ | ✅ |
| `0006~0019` | ⏳ 按批次实现（数据字典 §8） | 同左 |

> ⚠️ **为什么不一次性把 19 个都建出来**：迁移是**跟着功能走**的（先有表才有代码，或反之）。
> 一次性写完只会得到一堆**没被任何代码验证过的 DDL** —— 那种"看起来齐了"的假完成，比空目录更危险。
> **按批次写、写一个跑一个。**
>
> ⚠️ **PostgreSQL 侧本机无法执行验证**（边界要求不使用本机 PG）。
> 因此 `postgres/` 目前只受 **结构一致性 + 方言 lint** 保护，**"能被 PG 解析"尚未实测** ——
> 接生产实例时（批次 1）必须补一次真跑，**在那之前不声称它已在 PG 上验证过**。
