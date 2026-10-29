# 第三课堂 · API 接口参考（v1）

> 配套：`docs/PRD-第三课堂-v2.md`（需求）· `docs/数据字典-v1.md`（字段）。
> 本文是**前后端并行开发的对齐基线** —— 契约先定，两端各写各的，最后对接。

| 项 | 内容 |
|---|---|
| 版本 | **v1.14**（v2.24：退款分两类 `normal` / `reward`，后者即「先买后退」的免费学习；v2.23：新增管理员授权免费学习三接口 `/admin/entitlements`；v2.16：测评可信度与断线续考 · 价格试算与优惠码 · 离线下载 · 运营位与风控 · 异常复核） |
| 日期 | 2026-10-03 |
| 基址 | 生产 `https://cw-tc.ccit-agent.cn/api/v1` · staging `https://staging.cw-tc.ccit-agent.cn/api/v1` |
| 反向代理 | nginx 反代到 `127.0.0.1:8071`（**应用端口不对公网开放**） |
| 数据格式 | JSON（`Content-Type: application/json; charset=utf-8`）；上传用 `multipart/form-data` |
| 状态 | **M1 起按本契约实现**；破坏性变更必须升 `/v2` 并保留旧版至少一个迭代 |

---

## 1. 通用约定（**这节是全文最重要的部分**）

### 1.1 统一响应信封

**所有** JSON 接口（含错误）都返回同一信封：

```json
{ "code": 0, "data": { }, "msg": "" }
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | int | **0 = 业务成功**；非 0 见 §1.4 错误码表 |
| `data` | object / array / null | 业务数据；失败时为 `null` |
| `msg` | string | 面向人的说明（**中文，可直接展示给用户**；不得含敏感信息） |

- **`code != 0` 时 HTTP 状态码仍是 200** —— 这是"业务失败"与"协议失败"的区分：
  协议层错误（401/403/404/429/500）走 HTTP 状态码，业务层错误走 `code`。
- **例外**：`/verify` 验真页是 **HTML（Go 模板直出）**，不套信封（保 SEO）；
  其 JSON 版本 `/api/v1/verify/{certNo}` 套信封。

### 1.2 鉴权

**同一后端支持两种载体**（Web 与 App 各取所需）：

| 端 | 载体 | 说明 |
|---|---|---|
| **Web** | **HttpOnly Cookie**（`cw_tc_sid`，`Secure` + `SameSite=Lax`） | 浏览器不接触 token；**不得写 localStorage** |
| **App** | **`Authorization: Bearer <token>`** | token 只存 `flutter_secure_storage` |

- 服务端**统一签发会话 token**，中间件**两种载体都认**（Cookie 优先，其次 Header）。
- **有效期**：会话 **≤ 7 天**；敏感操作（改密 / 换绑手机号 / 注销 / 退款申请）**需二次短信验证**。
- **登出**：`POST /auth/logout` 销毁当前会话；`POST /auth/logout-all` 登出全部设备。
- **不做 refresh token**（首发）：会话到期重新登录。理由：少一个长期凭据就少一个泄露面。
- ⚠️ **绝不允许**：把身份证号 / 手机号放进 URL、query、Referer；
  登录与注册**一律 POST body**（`packages/app` 客户端已对此做运行时断言，见 `api_client.dart`）。

### 1.3 分页、排序与筛选

- **分页参数**：`page`（从 1 开始，默认 1）、`page_size`（默认 20，**最大 100**）。
- **分页响应**：`data = { "items": [...], "total": <int>, "page": <int>, "page_size": <int> }`。
- **排序**：`sort=<field>`，前缀 `-` 表示倒序（如 `sort=-created_at`）。**只允许白名单字段**。
- **筛选**：具名 query（如 `major_id`、`status`、`q`）；**不提供通用查询 DSL**（防注入与越权）。

### 1.4 错误码表

| 段 | 含义 | 示例 |
|---|---|---|
| `0` | 成功 | — |
| `10000–10099` | **通用参数错误** | `10001` 参数缺失 · `10002` 参数格式非法 · `10003` 分页越界 |
| `10100–10199` | **账号与认证** | `10101` 验证码错误或过期 · `10102` 账号已锁定 · `10103` 手机号已注册 · `10104` 身份证号已注册 · `10105` 身份证号校验位错误 · `10106` 需二次短信验证 · `10107` 未实名 |
| `10200–10299` | **课程与资源** | `10201` 课程不存在 · `10202` 课程未上架 · `10203` **权益未解锁** · `10204` 资源不存在 |
| `10300–10399` | **订单与支付** | `10301` 订单不存在 · `10302` 订单状态不允许此操作 · `10303` 重复下单被拒（幂等） · `10304` 金额不一致 · `10305` **退款窗口已过** · `10306` 渠道调用失败 |
| `10400–10499` | **测评** | `10401` 试卷不存在 · `10402` 已交卷不可改 · `10403` **权益未解锁，不可参加测评** · `10404` 该档可组卷套数不足 |
| `10500–10599` | **证书** | `10501` 证书不存在 · `10502` 同课程同等级已发证 · `10503` 成绩未达标 · `10504` 发证前置不满足（权益失效） |
| `10600–10699` | **AI** | `10601` 额度用尽 · `10602` 知识库未就绪 · `10603` **测评进行中，AI 已关闭** · `10604` 内容审核拦截 |
| `10700–10799` | **权限** | `10701` 未认证（对应 HTTP 401） · `10702` 无权限（对应 HTTP 403） |
| `10900–10999` | **系统** | `10901` 内部错误 · `10902` 上游依赖不可用 · `10903` 限频（对应 HTTP 429） |

- **约定**：`msg` 必须能直接展示；**技术细节只进日志**，不进 `msg`。
- 新增错误码**只增不改**（避免前端文案错位）。

### 1.5 时间、时区与枚举

- **存储**：数据库一律 **`timestamptz`（UTC）**。
- **传输**：API 时间字段一律 **RFC3339 带时区**（如 `2026-10-03T15:04:05+08:00`）。
- **展示**：前端一律按 **`Asia/Shanghai`** 渲染。
- **枚举**：**数据库与 API 一律小写英文**（`pending` / `paid` / `revoked`）；
  **中文只在前端展示**。理由：中文进 URL/日志会带来编码与检索问题。
- ⚠️ **证书编号的「年份」按 `Asia/Shanghai` 的签发日期取年** ——
  跨年场景（12/31 考、1/1 发证）**以签发日为准**，避免"考试年"与"签发年"两个口径打架。

### 1.6 幂等

| 场景 | 幂等键 | 行为 |
|---|---|---|
| **下单** | 客户端 `request_id` + `(student_id, course_code, status=pending)` 部分唯一索引 | 已存在 pending 订单 → **返回同一 `order_no`**，不新建 |
| **支付回调** | 渠道交易号 + `idempotency_key` 唯一约束 | 重复回调**只生效一次**，第二次返回成功但不重复开通 |
| **发证** | `(student_id, course_code, level)` 唯一 | 重复请求返回**已存在的证书编号** |
| **短信发送** | 手机号 + 模板 + 60s 冷却 | 冷却期内重复请求 → `10101` |

- 所有**写操作**都应是幂等的或至少是"重放安全"的（重放不会产生第二条数据）。

### 1.7 接口鉴权矩阵

| 类别 | 鉴权 | 示例 |
|---|---|---|
| **公开（匿名）** | 无 | `/health` · `/public/status` · `/app/latest` · `/verify/{certNo}` · `/majors` · `/courses`（仅上架课程的公开字段） · **`/public/overview` · `/public/job-map` · `/public/showcase`（v2.13，只回已发布/已授权/已核验的内容）** |
| **学员（需登录）** | 会话 | `/me/*` · `/orders` · `/exams` · `/ai/*` · `/notes` · `/me/certs` |
| **教师（角色）** | 会话 + `role=teacher` | `/teacher/**`（**且必须校验资源归属：只能操作自己的课程**）；**响应体须过滤身份证号与手机号**（§1.9） |
| **运营管理员** | 会话 + `role=admin_ops` | `/admin/**`（**不含** `/admin/admins` · `/admin/settings` · `/admin/exports`） |
| **超级管理员** | 会话 + `role=admin_super` | `/admin/**` 全部（含 `/admin/admins` · `/admin/settings` · `/admin/exports` · **`reveal` 看全号**） |
| **渠道回调（匿名但有签名）** | **验签** | `/pay/wechat/notify` · `/pay/alipay/notify` |

- **权限判定顺序**：① 是否登录 → ② 角色是否匹配 → ③ **资源是否属于自己**（教师必需）。
- **前端只做菜单可见性，不作为安全边界** —— 越权必须由后端拒绝。

### 1.10 管理员可见字段白名单（**v2.6 新增**）

> 对应 PRD §7.10.10（二）。与 §1.9 同一条原则，但管理员范围更宽 —— **宽 ≠ 无痕**。

| 默认返回（**脱敏**） | 看全号的条件 |
|---|---|
| `phone_masked`（`138****5678`） | `GET /admin/users/{id}/reveal?field=phone&reason=...` → **ops 可，写审计** |
| `id_card_masked` | `?field=id_card&reason=...` → **仅 super，写审计** |
| `status` · `status_reason_kind` · 订单 / 证书 / 学习记录 | 无需额外条件 |

| **永不返回** | 说明 |
|---|---|
| `password_hash`（**任何情况**） | 重置只能"发链接"，**不得代设、不得回显** |
| 学员**笔记**内容 | 仅本人（PRD §7.6） |

- **实现口径同 §1.9**：序列化层白名单 DTO，**不得直接返回数据库实体**。
- **搜索约束**：`GET /admin/users` **不接受 `id_card` 参数** ——
  传了也**必须被忽略**（否则等于给身份证号开查询接口，与 §9.6 防撞库冲突）。

### 1.9 教师可见字段白名单（**v2.5 新增 —— 隐私硬约束**）

> 对应 PRD §7.9 J-6。**这是白名单，不是黑名单** —— 未列入的字段一律不返回。

| 允许返回 | 说明 |
|---|---|
| 学员 `name` · `student_no` | 阅卷与答疑必需 |
| 该学员在**本课**的学习 / 测评聚合数据 | 进度、成绩、错题分布 |

| 永不返回（**任何教师接口**） | 说明 |
|---|---|
| `id_card_hash` · `id_card_cipher` · **脱敏身份证号** | 与教学无关 |
| `phone` · **脱敏手机号** | 需触达学员时走站内信 / 平台转发 |
| 订单 / 支付 / 退款字段 | 属管理员 |
| 学员笔记 | 仅本人（PRD §7.6） |

- **实现口径**：教师接口的响应**在序列化层用白名单构造 DTO**，
  **不得**直接返回数据库实体（返回实体 = 迟早漏字段）。
- **门禁**：遍历教师接口做正则扫描（18 位身份证号 / 11 位手机号），**必须 0 命中**。

### 1.8 限频（与 PRD §9.6 对应）

| 接口 | 限制 |
|---|---|
| `/auth/sms/send` | 图形验证码前置；单手机号 ≤ 10 条/日、60 s 冷却；单 IP ≤ 20 次/时 |
| `/auth/login` | 单账号 + 单 IP 失败 5 次锁定 10 分钟 |
| `/verify/{certNo}` | 单 IP ≤ 20 次/分 |
| `/public/overview` · `/public/job-map` | 单 IP ≤ 120 次/分；**以 `web_content.version` 为缓存键**，发布即失效 |
| `/public/showcase` | 单 IP ≤ 60 次/分；**学员撤回授权必须主动失效缓存**（否则“撤回了还挂着”是合规事故） |
| `/ai/**` | 按「学员 × 课程」额度；同时限制并发 1 路/学员 |
| `/promo/validate` | 单学员 ≤ 20 次/日（**防撞码** —— 优惠码是费用入口，与短信同级对待，§9.13） |

- 触发限频返回 **HTTP 429** 且 `code=10903`。

---

## 2. 接口清单

> 标记：`[P0]` 首发必须 · `[P1]` 后续。**未标 P1 的一律为 P0**。

### 2.1 账号与实名（模块 A）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/auth/captcha` | 公开 | 取图形 / 滑块验证码（返回 `captcha_id` + 图片 base64） |
| POST | `/auth/sms/send` | 公开 | 发短信验证码；**必须带 `captcha_id` + 答案** |
| POST | `/auth/register` | 公开 | 注册（见 §3.1） |
| POST | `/auth/login` | 公开 | 登录（验证码 / 密码两种模式，见 §3.2） |
| POST | `/auth/logout` | 登录 | 销毁当前会话 |
| POST | `/auth/logout-all` | 登录 | 登出全部设备 |
| POST | `/auth/password/reset` | 公开 | 用管理员下发的一次性重置码设置新密码（**不暴露是否已注册**；重置码 30 分钟、仅存哈希） |
| GET | `/me` | 登录 | 我的资料（**身份证号与手机号均脱敏**） |
| POST | `/me/password` | 登录 | 改密码（需旧密码；成功后全部会话失效） |
| POST | `/me/phone` | 登录 | 换绑手机号（**旧号 + 新号双向验证**） |
| POST | `/me/delete` | 登录 | 注销账号（匿名化，证书台账留存） |

### 2.2 课程与专业（模块 B）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/majors` | 公开 | 专业字典（启用中，按 `sort`） |
| GET | `/courses` | 公开 | 课程广场：`major_id` / `line`(main\|aux) / `q` / 分页；**只返回已上架** |
| GET | `/courses/{code}` | 可选登录 | 详情（价格、三档证书、试学讲次、**我的权益状态**、`ai_level` AI 化等级 + `quality_badge` 质检通过标、**`stats` 六项状态数 + `data_as_of` 数据截至日期**） |
| GET | `/courses/{code}/comments` | 公开 | **课程评论列表**（**均分需满 10 条才返回**；昵称脱敏；分页） |
| GET | `/courses/{code}/lectures` | 可选登录 | 讲次列表（未解锁时**只返回到试学讲次**） |
| GET | `/courses/{code}/lectures/{no}` | 可选登录 | 讲详情 + 资源清单（**未解锁 → `10203`**） |
| GET | `/me/courses` | 登录 | 我的课程（已解锁 / 未解锁） |
| PUT | `/me/courses/{code}/progress` | 登录 | 上报进度（讲级） |

### 2.3 资源访问（模块 B 续）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/assets/{assetId}/url` | 登录 | 换取**带签名、短时效**的资源地址（见 §3.3） |

- **COS 一律私有读**；不暴露永久直链（防抄课件、防外链盗用）。

### 2.4 订单与支付（模块 G）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/orders` | 登录 | 下单（**幂等**，见 §3.4） |
| GET | `/orders` | 登录 | 我的订单列表 |
| GET | `/orders/{orderNo}` | 登录 | 订单详情（**前端轮询用**） |
| POST | `/orders/{orderNo}/close` | 登录 | 主动关闭待支付订单 |
| POST | `/orders/{orderNo}/refund` | 登录 | **学员自助退款**（窗口内，见 §3.5） |
| POST | `/orders/quote` | 登录 | **价格试算**（下单前展示）：返回 `list_cents`（原价）/ `discount_cents`（优惠）/ `pay_cents`（实付），§7.3 G-6 |
| POST | `/promo/validate` | 登录 | **优惠码校验**（单人限用 / 总量 / 有效期）；**限频防撞码**（§1.8） |
| POST | `/pay/wechat/native` | 登录 | 取微信 Native 支付二维码 |
| POST | `/pay/alipay/precreate` | 登录 | 取支付宝二维码 |
| POST | `/pay/wechat/notify` | **验签** | 微信异步回调（见 §3.6） |
| POST | `/pay/alipay/notify` | **验签** | 支付宝异步回调 |
| POST | `/pay/wechat/app` | 登录 | 取微信 **App 支付**参数（`prepay_id` + 签名）；**包名与签名须与商户号登记一致** |
| POST | `/pay/alipay/app` | 登录 | 取支付宝 **App 支付** `orderStr` |
| POST | `/orders/{orderNo}/sync` | 登录 | **主动向渠道查单并同步状态**（回调丢失兜底；**幂等**，重复调不重复开通权益） |
| GET | `/orders/{orderNo}/refund` | 登录 | 退款状态查询（**学员可见**，含失败原因） |

### 2.5 测评与成绩（模块 C）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/exams` | 登录 | 开考（组卷；**需权益解锁**） |
| GET | `/exams/{id}` | 登录 | 取卷（**不返回参考答案**） |
| PATCH | `/exams/{id}/answers` | 登录 | 暂存答案（断网可恢复） |
| POST | `/exams/{id}/submit` | 登录 | 交卷 → 客观题即时判分 |
| GET | `/exams/{id}/result` | 登录 | 成绩单（分档 + 口径说明） |
| GET | `/me/scores` | 登录 | 我的成绩列表 |
| GET | `/exams/{id}/state` | 登录 | **续考状态**（断线重连用）：当前题号 · 已答清单 · **剩余秒数（服务端计算）** · 是否被标记异常 · **是否需接管**（单会话） |
| POST | `/exams/{id}/takeover` | 登录 | **换设备接管答题**：结束旧会话 + **留痕 + 标记异常**（**不静默允许**，§7.7 C-7） |
| POST | `/exams/{id}/anomaly` | 登录 | 客户端上报异常事件（切屏 / 离开答题页）—— **只记录，不判定**（§7.7 C-7） |

### 2.6 证书（模块 D）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/exams/{id}/issue-cert` | 登录 | 触发发证（**幂等**；达标才发） |
| GET | `/me/certs` | 登录 | 我的证书列表（含状态、**有效期至**） |
| POST | `/me/certs/{certNo}/download` | 登录 | 取**短时签名**下载链接（不暴露永久地址） |
| GET | `/verify/{certNo}` | **公开** | 验真 **API**（JSON，四态） |
| GET | `/verify` | **公开** | 验真**页面**（HTML，Go 模板直出，套 SEO） |

### 2.7 AI 助教与知识库（模块 H）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/ai/sessions` | 登录 | 建会话（绑定 `course_code`，可带 `lecture_no`） |
| GET | `/ai/sessions` | 登录 | 我的会话列表 |
| GET | `/ai/sessions/{id}/messages` | 登录 | 历史消息（含引用） |
| POST | `/ai/sessions/{id}/messages` | 登录 | 提问（**SSE 流式**，见 §3.7） |
| DELETE | `/ai/sessions/{id}` | 登录 | 删除会话（学员可自行删除） |
| POST | `/ai/messages/{id}/report` | 登录 | 举报 / 纠错 |

### 2.8 学习笔记（模块 I）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/notes` | 登录 | 列表（按课程 / 讲次筛选） |
| POST | `/notes` | 登录 | 新建（手动 / 从 AI 消息收藏 / 从题目解析收藏） |
| PATCH | `/notes/{id}` | 登录 | 修改 |
| DELETE | `/notes/{id}` | 登录 | 删除 |
| POST | `/notes/export` | 登录 | 导出（**返回文件流**，`text/markdown`） |

### 2.9 通知（模块 F）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/me/notifications` | 登录 | 我的通知（分页，未读数放 `data.unread`） |
| POST | `/me/notifications/{id}/read` | 登录 | 标记已读 |

### 2.10 教师工作台（模块 J）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/teacher/courses` | 教师 | 我的课程（草稿 / 待审 / 已上架） |
| POST | `/teacher/courses` | 教师 | 新建课程（**自动分配课程码**） |
| GET | `/teacher/courses/{code}` | 教师+归属 | 课程详情 |
| PATCH | `/teacher/courses/{code}` | 教师+归属 | 编辑（**不可改价格 / 上架状态 / 课程码**） |
| POST | `/teacher/courses/{code}/submit` | 教师+归属 | **送审**（**三项前置校验**；失败返回**缺项明细**，§J-2.1） |
| POST | `/teacher/courses/{code}/withdraw` | 教师+归属 | **撤回送审**（未被受理前，回到 `building`） |
| GET | `/teacher/courses/{code}/review` | 教师+归属 | **审核结果与历史轮次**（含**逐维问题清单**，§J-2.3） |
| GET | `/teacher/courses/{code}/selfcheck` | 教师+归属 | **送审前自检**：九件套齐备度 + 缺项清单（**不消耗送审次数**） |
| POST | `/teacher/courses/{code}/changes` | 教师+归属 | **提交变更**（已上架课程走变更草稿，§J-2.5） |
| GET | `/teacher/reviews` | 教师 | 待阅卷列表 |
| POST | `/teacher/reviews/{id}/score` | 教师 | 采点给分（双人阅卷） |
| GET | `/teacher/courses/{code}/stats` | 教师+归属 | 本课学情（**仅聚合，字段白名单见 §1.9**）+ **六项状态数 + 漏斗 + 转化率**（`range=7d\|30d\|90d`，v2.8） |
| GET | `/teacher/courses/{code}/students` | 教师+归属 | 本课学员列表（**只回 `name` / `student_no` 与进度**，§1.9） |
| POST | `/teacher/courses/{code}/requests` | 教师+归属 | **发起下架 / 内容纠错申请**（`kind=下架\|紧急纠错`，§7.9 J-8） |
| GET | `/teacher/requests` | 教师 | 我的申请与处理状态 |
| GET | `/teacher/insights` | 教师+归属 | **AI 运营周报列表**（`course_code` + 周期） |
| GET | `/teacher/insights/{id}` | 教师+归属 | **周报详情**（五段结构；`evidence_map` **每条建议附依据指标与评论 id**） |
| POST | `/teacher/insights/{id}/verdict` | 教师+归属 | **采纳 / 驳回**（记意见）—— **不能编辑报告正文** |
| GET · POST | `/teacher/revision-plans` | 教师+归属 | **改版计划列表 / 新建**（`problem` / `evidence` / `measure` / `target_metric` **必填**） |
| PATCH | `/teacher/revision-plans/{id}` | 教师+归属 | 更新状态（`已完成` 时**关联 `course_revision`**；`已放弃` 须填原因） |
| GET | `/teacher/revisions/{id}/compare` | 教师+归属 | **改版前后对比**（按 `content_version` 切分，窗口默认前后各 30 天） |

### 2.11 管理后台（模块 E）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/admin/courses` | 管理员 | 全部课程（含未上架 / 待审） |
| PATCH | `/admin/courses/{code}` | 管理员 | **课程设置面板**（价格、专业、上架状态、试学讲次、AI 开关与额度…） |
| GET | `/admin/reviews` | 管理员 | **审核待办队列**（含等待天数，**超 SLA 高亮**，§J-2.2） |
| GET | `/admin/courses/{code}/review` | 管理员 | **审核详情**：内容快照 + **六维自动初值**（材料完整性）+ 历史轮次 |
| POST | `/admin/courses/{code}/review` | 管理员（**特色标需 super**） | **提交审核结论**：**六维打分 + 逐维评语**；`verdict=rejected` **必须带 `issue_list`**（§J-2.3） |
| POST | `/admin/courses/{code}/publish` | 管理员 | **上架三联动**：`publish` / `notify`（广播）/ `promote`（宣传位）**三个开关独立**，§J-2.4 |
| POST | `/admin/courses/{code}/unpublish` | 管理员 | 下架（**宣传位自动移除**；已购学员不受影响） |
| GET · POST | `/admin/courses/{code}/quality` | 管理员 | **质量分与 AI 化等级**查询 / 核定覆写（写审计） |
| GET | `/admin/courses/{code}/checkup` | 管理员 | **内容体检**：九件套齐备度 + 交错序列校验 + 题库一致性（§B-9 / §B-10） |
| POST | `/admin/courses/{code}/kb/rebuild` | 管理员 | 触发知识库重建 |
| GET | `/admin/kb/status` | 管理员 | 知识库状态（**切片数 / 向量数 / 最后构建时间**） |
| GET | `/admin/majors` · POST · PATCH | 管理员 | 专业字典维护 |
| GET | `/admin/orders` | 管理员 | 订单查询（筛选 + 导出 CSV） |
| POST | `/admin/orders/{orderNo}/refund` | 管理员 | 管理员退款。**`kind`**：`normal`（默认，**连带撤销证书**）/ `reward`（**奖励性退款**：保留权益与证书）；**`reward` 必填 `basis` + `basis_ref`，可选 `amount_cents`（部分 = 打折）**；二次确认 + 写审计（§7.3 G-4 / G-7） |
| GET | `/admin/reconcile` | 管理员 | 对账结果（**差异必须为 0**） |
| GET | `/admin/reconcile/{date}` | 管理员 | **按日对账明细**（笔数 / 金额 / 退款，含渠道原始对账单对照） |
| GET | `/admin/reconcile/{date}/diff` | 管理员 | **差异明细导出 CSV**（差异 ≠ 0 时必看；同时触发告警，不靠人盯页面） |
| GET | `/admin/payments/channels` | 管理员 | **渠道配置状态**（只回"已配置 / 未配置"与商户号后四位；**绝不返回值**，§7.10.6） |
| POST | `/admin/orders/{orderNo}/partial-refund` | 管理员 | **[P0] 部分退款（= 打折）** —— **语义已定（v2.24）**：等价于 `kind=reward` + 指定 `amount_cents`；**保留权益与证书**、**不计入退款率**、**计入对账**（§7.3 G-7）。保留本路径仅为兼容早期引用 |
| GET | `/admin/certs` | 管理员 | 证书台账 |
| POST | `/admin/certs/{certNo}/revoke` | 管理员 | 撤销证书（须填原因） |
| GET | `/admin/users` | 管理员 | **用户列表**：`type` 可选 `student` / `teacher` / `admin_super` / `admin_ops`，不传则展示全部；`limit`（1~100，默认 20）和 `offset` 分页，返回 `items` 与 `total`。每项带 `user_type`；学员带学号和脱敏手机号，内部人员带账号，不返回手机号或身份证号。`id_card` 参数被忽略。 |
| GET | `/admin/users/{id}` | 管理员 | **用户详情**（**默认脱敏**，见 §1.10） |
| GET | `/admin/users/{id}/reveal` | **super**（手机号 ops 可） | **查看全号**：须带 `reason`，**每次写审计**（§7.10.10（二）） |
| PATCH | `/admin/users/{id}/status` | 管理员（永久封禁仅 super） | **封禁 / 解封**：须带 `reason_kind` + `note`（+ `until`） |
| POST | `/admin/users/{id}/reset-password` | 管理员 | **生成一次性重置码**（30 分钟，须带 `reason` + `confirm:true`）：返回 `reset_code` / `expires_at` / `phone_last4`，**绝不返回密码**；管理员转交学员后用 `/auth/password/reset` 自行设新密码；写审计 |
| POST | `/admin/users/teachers` | 管理员 | **开通教师账号** |
| POST | `/admin/users/{id}/transfer-courses` | 管理员 | **教师停用前的课程移交**（`to_teacher_id`，§7.9 J-9） |
| GET · PATCH | `/admin/settings` | **仅 super** | **系统配置**：读业务参数列表（含 `min_value`/`max_value`/`description`）；写**逐个 key**（如 `stats.display_threshold`）—— **低于 `min_value` 返回 400**；**密钥只读状态**（§7.10.6） |
| GET · POST | `/admin/admins` | **仅 super** | 管理员列表 / **创建管理员**（分级 §7.10.5） |
| DELETE | `/admin/admins/{id}` | **仅 super** | 停用管理员（**禁止停用最后一个超管**） |
| POST | `/admin/batch/courses` | 管理员 | **批量上架 / 下架 / 改价 / 挂专业**（先预览，§7.10.8） |
| GET | `/admin/batch/{batchId}` | 管理员 | 批量结果**逐条明细**（可导出） |
| POST | `/admin/exports` | **仅 super** | **代学员导出数据**（须填事由；写审计，§7.10.9） |
| GET | `/admin/requests` | 管理员 | 教师下架 / 纠错申请待办（§7.9 J-8） |
| GET | `/admin/comments` | 管理员 | **UGC 统一待办**（评论 + 讨论帖 + 回复；机审疑似 + 被举报），按 `target_type` 筛选 |
| POST | `/admin/comments/{id}/remove` | 管理员 | **下架评论**：**`reason_kind` 必填**（广告/辱骂/隐私/违法/无关/其他，**无"差评"选项**），写审计 + 通知评论者 |
| POST | `/admin/comments/{id}/restore` | 管理员 | 恢复（误判纠正，写审计） |
| GET · POST | `/admin/comments/reports/{id}` | 管理员 | 举报处理（**结果通知举报人**） |
| GET | `/admin/discussions` | 管理员 | 讨论治理列表（**返回真实 `author_id`，匿名帖可见发布者**，§9.10.1） |
| POST | `/admin/ugc/{targetType}/{id}/remove` | 管理员 | 下架评论 / 帖 / 回复：**`reason_kind` 必填**（枚举无"差评/不利"），写审计 + 通知发布者 |
| POST | `/admin/ugc/{targetType}/{id}/restore` | 管理员 | 误判恢复（写审计） |
| GET | `/admin/insights` | 管理员 | **全平台课程运营看板**（六项状态 + 漏斗，可按专业/来源筛选） |
| GET | `/admin/stats/{courseCode}` | 管理员 | 单课统计明细（**与教师版同一套口径**，M-0） |
| GET · POST | `/admin/appeals` · `/{id}/handle` | 管理员 | **账号申诉待办与处理**（§7.10.10（三）） |
| GET · POST | `/admin/deletions` · `/{id}/approve` | 管理员 | **注销审核**：前置校验 + 匿名化执行（§7.10.10（五）） |
| GET | `/admin/broadcasts/preview` | 管理员 | **广播预览：返回触达人数** |
| POST | `/admin/broadcasts` | 管理员 | 发送广播 |
| GET·PATCH | `/admin/content` | 管理员 | **对外内容管理**（简介三档 / 六条亮点 / 声明 / AI 公示 `filing_note`）；**发布即生成版本，旧版本不删**；**`evidence_ref` 为空不允许发布**；发布与下架**二次确认** |
| GET·PATCH | `/admin/job-map` | 管理员 | **就业直达映射**维护与发布；**`evidence_ref` 与 `lecture_refs` 缺一不得发布**；**可改不可清空** |
| POST | `/admin/entitlements` | 管理员 | **授权免费学习**（`student_id` + `course_id` + **`reason` 必填**）；**重复授权 409**；**写 `audit_log`**；**二次确认**（§7.3 G-7） |
| DELETE | `/admin/entitlements/{id}` | 管理员 | **撤销授权**（`reason` 必填）；**只收权益，已发证保留**；写审计 |
| GET | `/admin/entitlements` | 管理员 | 授权台账（按学员 / 课程 / 时间筛；**只回脱敏后的学员标识**） |
| GET·PATCH | `/admin/showcase` | 管理员 | **学员成果核验与授权台账**（提交 / 授权 / 撤回时间 · 核验人）；**一键下架**；**撤回后 24h 内下架（含缓存失效）**；**只核验不存档**（不留劳动合同 / 社保 / 薪资） |
| GET·PATCH | `/admin/operations` | 管理员 | **运营位管理**（banner / 推荐位 / 置顶）：排期 · 权重 · 上下线；**课程下架时自动移除**；**文案过禁词门禁**（§7.10.12） |
| GET·PATCH | `/admin/promo` | 管理员 | **优惠码管理**：创建 / 定向发放 / **单人限用 + 总量上限 + 有效期**（§7.3 G-6） |
| GET·PATCH | `/admin/risk` | 管理员 | **风控规则与黑名单**：退款次数上限 · 关联提示阈值 · 黑名单（**必须有期限 + 可申诉**）（§9.13） |
| GET | `/admin/exams/{id}/anomalies` | 管理员 | **测评异常记录复核**（切屏 / 秒答 / 设备突变 / 多会话）—— **人工判定，AI 不参与**（§7.7 C-7） |
| POST | `/admin/exams/{id}/verdict` | 管理员 | **异常复核结论**（维持成绩 / 判无效 / 转人工核验）；**必填理由，写审计，学员可申诉**（§7.7 C-7） |
| GET | `/admin/audit` | 管理员 | 审计日志 |

### 2.13 学员侧「我的」模块（模块 L，**v2.5 新增**）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/me/courses` | 学员 | 我的课程（`unlocked=true/false` + **学习进度**） |
| GET | `/me/orders` | 学员 | 我的订单（**含"可继续支付"标记与退款进度**） |
| POST | `/me/orders/{orderNo}/pay` | 学员 | **继续支付**（复用同一订单，不新建，§7.13 L-2） |
| POST | `/me/orders/{orderNo}/cancel` | 学员 | 取消待支付订单 |
| GET | `/me/scores` | 学员 | 我的成绩（**历次不覆盖**） |
| GET | `/me/certs` | 学员 | 我的证书（含 `status`：有效 / 即将到期 / 已过期 / 已撤销） |
| GET | `/me/certs/{certNo}/file` | 学员 | **下载 / 重新下载证书 PDF**（短时私有链接，§7.8 D-5） |
| GET | `/me/messages` | 学员 | 消息中心（`type`：系统 / 事务 / 广播；`unread`） |
| POST | `/me/messages/read` | 学员 | 批量标记已读 |
| PATCH | `/me/messages/prefs` | 学员 | **广播退订**（系统消息与事务通知不可退订，§7.13 L-5） |
| GET | `/me/devices` | 学员 | 登录设备列表（§7.13 L-6） |
| POST | `/me/devices/{id}/revoke` | 学员 | 踢出指定设备 |
| POST | `/me/logout-all` | 学员 | 登出全部设备 |
| GET | `/me/downloads` | 学员 | **离线下载清单**（占用空间 / **离线有效期** / 状态，§7.4 B-11） |
| POST | `/me/downloads` | 学员 | **申请离线下载授权**（**已解锁且权益有效**才签发；退款后失效） |
| DELETE | `/me/downloads/{lectureNo}` | 学员 | 删除单讲下载（客户端同步清理本地文件） |
| POST | `/me/export` | 学员 | **导出我的数据**（JSON + MD 摘要，脱敏，写审计，§7.13 L-8） |
| GET | `/me/appeals` · POST | 学员 | 申诉（改姓名 / 身份证、成绩申诉）与处理状态 |
| POST | `/auth/appeal` | **匿名**（登录页） | **被封 / 登录不上的学员发起申诉**（带手机号 + 验证码，**限频同短信**，§7.10.10（三）） |

> **`/me/certs/{certNo}/file` 的硬约束**：**证书已撤销时返回 404**（不是 403 ——
> 避免暴露"这个编号曾经存在"之外的更多信息）；**重新下载必须返回与首次逐字节一致的文件**。

### 2.16 AI 生成内容与双审（模块 P，**v2.11 新增 · v2.14 补合规留痕**）

> **v2.14 补两条硬要求**（M4 文档 §五）：① 每次生成必须记录
> **`model` / `model_version` / `filing_no`（备案号或上线编号）** —— 用于监管抽查举证；
> ② AI 生成的资产（讲义 / 课件 / 导出包）必须带**显式标识**（学员端可见）与
> **隐式标识**（文件元数据含 `AIGC` 字段）。**模型不在已备案清单内 → 拒绝调用 + 告警。**

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/teacher/ai-drafts` | 教师+归属 | **草稿列表**（待我一审 / 我已审 / 已上架），可按状态筛选 |
| POST | `/teacher/ai-drafts` | 教师+归属 | **生成草稿**（`trigger_kind` + `draft_scope` + `kind` + `source_topic_ids[]`）——<br>**`draft_scope=chapter` 时报 `lecture_count`（≤ 配置上限，默认 5）**；<br>**无归属教师的课程返回 409**；**额度用尽返回 429**；<br>**生成后自动体检**，`auto_check` 结果随响应返回（未过则停在草稿态、**不消耗一审**） |
| GET | `/teacher/ai-drafts/{id}` | 教师+归属 | 草稿详情（**内容 + AI 标识 + 生成依据 + 触发建议原帖**） |
| PATCH | `/teacher/ai-drafts/{id}` | 教师+归属 | **编辑草稿**（保存即更新 `reviewed_content_hash` 的依据版本） |
| POST | `/teacher/ai-drafts/{id}/review` | 教师+归属 | **一审提交**：`verdict=approved\|rejected`；**`rejected` 必须带 `issue_list`**；<br>**即使一字未改也必须提交**（`edited_by_teacher` 如实记录，§7.16 P-3） |
| GET | `/teacher/ai-drafts/{id}/check` | 教师+归属 | **重新查看体检报告**（`auto_check` 逐项结论 + 未过原因） |
| GET | `/admin/ai-drafts` | 管理员 | **二审待办队列**（**只含已过一审的草稿**） |
| POST | `/admin/ai-drafts/{id}/review` | 管理员 | **二审提交**：内容安全 / AI 标识 / 侵权 / **`in_exam_scope` 显式决定** / 通过或驳回（驳回必带 `issue_list`） |
| POST | `/admin/ai-drafts/{id}/publish` | 管理员 | **上架**：**走 §7.9 J-2.5**（`content_version+1` + 原子替换）+ J-2.4 三联动；<br>**未过双审返回 409** |
| POST | `/admin/ai-drafts/{id}/abandon` | 管理员 | 废弃草稿（**须填原因**；不计入课程版本） |
| POST | `/admin/lectures/{id}/unpublish` | 管理员 | **下架某条增补讲次**（`origin<>upstream`）；已发证书不受影响（§7.16 P-6） |
| POST | `/admin/lectures/{id}/emergency-remove` | 管理员 | **讲次级紧急撤下（v2.12）**：只撤这一讲（不必整课下架）；**须填原因 + 写审计**；<br>撤下后该讲**不进测评范围**、学员端显示"内容核对中"；**已学进度与证书不受影响**；**不物理删除** |
| GET | `/admin/lectures?origin=ai_assisted` | 管理员 | 本课 **AI 增补讲次**列表（含 `in_exam_scope` 与效果对比入口） |

> ⚠️ **三条接口层硬约束**：
> ① **单审不可上架**：`/publish` 前置校验**必须存在 `stage=teacher` 与 `stage=admin` 两条 `approved`**，
>    否则 **409**（**跳过审核或顺序颠倒一律拒绝**）；
> ② **`rejected` 必带 `issue_list`**，否则 **400**；
> ③ **`ai_labeled=false` 或 `trigger_topic_ids` 为空的草稿**，**不允许进入审核队列**（400）；
> ④ **`source_citations` 为空** 或 **`auto_check_passed=false`** 的草稿，
>    **不允许进入一审队列**（400，v2.12，§7.16 P-2.1 / P-2.2）；
> ⑤ **`draft_scope=chapter` 的上架必须一次性插入全部讲次** ——
>    **部分插入一律拒绝**（`lecture_count` 与数组长度不符 → 409，v2.12）。

### 2.15 讨论区（模块 N，**v2.10 新增**）

#### 2.15.1 每讲开放练习与讨论（当前实现）

以下接口按讲次号 `no` 定位，独立于课程总测评与证书。练习题由上游《思考题.md》同步（兼容行内解答、章节集中解答、`summary` 折叠、参考要点/判据等实际写法），`GET practice` 不返回参考解答；学员提交自答后才返回已有的参考解答与自动评分。参考答案可用时按关键词重合度给出 `score`（0~100，仅供自查），无参考答案时 `graded=false`、不判分。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/courses/{code}/lectures/{no}/practice` | 有该讲学习权限的学员 | 本讲思考题、出处、题号（不返回答案） |
| POST | `/courses/{code}/lectures/{no}/practice/{questionID}/answers` | 有该讲学习权限的学员 | 保存自答，返回 `reference_answer` / `graded` / `score`；不影响总课程测评 |
| GET | `/courses/{code}/lectures/{no}/topics` | 公开 | 帖子列表，点赞数降序，同票按时间与 ID 降序 |
| POST | `/courses/{code}/lectures/{no}/topics` | 登录学员 | 发帖；隐私与测评泄漏拦截 |
| GET · POST | `/courses/{code}/lectures/{no}/topics/{topicID}/replies` | 公开读；登录学员写 | 查看与发表公开评论 |
| POST · DELETE | `/courses/{code}/lectures/{no}/topics/{topicID}/like` | 登录学员 | 点赞与取消点赞，重复操作幂等 |

公开讨论只返回化名，不返回实名、手机号、身份证号。写入前拒绝个人敏感信息和疑似测评题目。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/courses/{code}/topics` | 学员（**已解锁**） | 帖子列表（`kind` / `sort=newest\|useful\|replies` 筛选；**置顶恒在首**） |
| POST | `/courses/{code}/topics` | 学员（已解锁） | **发帖**（`kind` 五选一；`anonymous` 可选；**疑似试题/答案 → 拒**） |
| GET · PATCH · DELETE | `/topics/{id}` | 作者本人 / 教师（**仅可置顶与标已解答**） | 详情 / 编辑 / **删除自己的帖**（**教师无删除权**） |
| POST | `/topics/{id}/replies` | 学员（已解锁） / 教师 | **回复**（`quote_reply_id` 一层引用；教师回复带官方标识且**不可匿名**） |
| DELETE | `/replies/{id}` | **仅作者本人** | **教师调此接口 → 403**（§7.15 N-5） |
| POST · DELETE | `/topics/{id}/useful` · `/replies/{id}/useful` | 学员（已解锁） | 点 / 取消"有用"（**不能自投**；重复幂等） |
| POST | `/topics/{id}/solved` | 作者 / 教师 | 标记已解答 + 指定 `solved_reply_id`（最佳回复） |
| POST | `/topics/{id}/pin` | **教师+归属** | 置顶 / 取消置顶 |
| GET | `/topics/{id}/poll` | 学员（已解锁） | **投票详情**：截止前**只返回 `voted_count`**；截止后返回各选项票数 |
| POST | `/topics/{id}/poll/vote` | 学员（已解锁） | 投票（单选 / 多选；**截止后拒绝**） |
| GET | `/teacher/discussions` | 教师+归属 | 讨论区工作台：**建议池 / 提问 / 待回复**（分 `kind` 与 `solved` 筛选） |
| POST | `/teacher/suggestions/{topicId}/feedback` | 教师+归属 | **建议处理**：`accepted / scheduled / rejected`（**rejected 必须带 `reason`**） |
| POST | `/teacher/suggestions/{topicId}/to-plan` | 教师+归属 | **一键转改版计划**：`problem` = 帖标题，`evidence` = **原帖链接**（§7.15 N-7） |
| GET | `/me/discussions` | 学员 | 我的帖子与"待我查看的新回复" |
| GET | `/me/suggestions` | 学员 | **我的建议与处理状态**（含不采纳理由与"已在 vN 改进"） |

> ⚠️ **两条接口层硬约束**：
> ① **不存在**教师可用的删帖 / 删回复路径（教师 token 调 → **403**）；
> ② **匿名帖**：对 `/courses/{code}/topics` 返回 `author_display = "匿名学员"`、`author_id = null`；
> 但 **`/admin/discussions` 返回真实 `author_id` 与学号**（**对同学匿名 ≠ 无痕**，§9.10.1）。
>
> **通知**：被回复 / @ / 被标已解答 / 建议被采纳 → **SSE 实时推送**（事件 `discussion`）；
> 断线后由客户端轮询兜底（30 s）。

### 2.14 收藏 · 评论 · 课程统计（模块 M，**v2.8 新增**）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST · DELETE | `/courses/{code}/favorite` | 学员 | **收藏 / 取消收藏**（重复收藏不报错、不增计数） |
| GET | `/me/favorites` | 学员 | 我的收藏（分页；下架课仍可见但标注"已下架"） |
| GET | `/me/reviews` | 学员 | 我的评价（已评 / 待评列表） |
| POST | `/courses/{code}/comments` | 学员（**已解锁**） | **发表评价**（`rating` 必填；正文含证件号/手机号 → **400 拒绝**） |
| PATCH · DELETE | `/comments/{id}` | 作者本人 | **修改一次**（标注"已修改"）/ **删除自己的评价** |
| POST | `/comments/{id}/report` | 登录 | **举报评论**（`reason_kind` 必选） |
| POST | `/comments/{id}/reply` | 教师+归属 | **教师回复**（**无删除接口** —— 见 §7.14 M-3） |
| GET | `/teacher/courses/{code}/stats` | 教师+归属 | **本课六项状态数 + 漏斗 + 转化率**（`range=7d\|30d\|90d`） |

> ⚠️ **接口层就掐死"教师删评论"**：**不存在** `DELETE /comments/{id}` 的教师可用路径；
> 教师 token 调该接口 → **403**（§9.10 验收项）。

### 2.12 对外与运维（模块 K）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/health` | 公开 | 健康检查（部署探活依赖） |
| GET | `/public/status` | 公开 | **CCIT 官网用的聚合快照**（**不含任何个人信息**） |
| GET | `/app/latest` | 公开 | APK 最新版本（`versionCode` / 下载地址 / **SHA-256**） |

### 2.17 对外展示（模块 Q，**v2.13 新增**）

> 与 §2.12 的 `/public/status` **分工不同**：**`/public/status` 答"平台活着没有"**，
> 本节三个接口答"**官网上说什么**"（PRD §7.17）。四者共用一条原则：
> **无需鉴权 · 可缓存 · 不含任何个人信息 · 不可用时如实返回**。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/public/overview` | 公开 | 落地页聚合内容（简介三档 / 六条亮点 / 专业矩阵 / AI 技术 / 含金量四证 + 上游只读数字） |
| GET | `/public/job-map` | 公开 | 就业直达映射；**只回 `status=published` 且 `evidence_ref` 与 `lecture_refs` 均非空**的条目 |
| GET | `/public/showcase` | 公开 | 学员成果与评价；**只回 `status=verified` 且授权有效**（未撤回）的条目，分页 |

- **管理端配套**（`admin_ops`，见 §2.11）：`/admin/content`（发布对外内容）·
  `/admin/job-map`（发布就业映射）· `/admin/showcase`（核验 / 授权台账 / **一键下架**）。
- **硬约束**：**`self_report` 与 `withdrawn` 条目在任何客户端接口都取不到**（有测试）。

---

## 3. 关键接口详述（**只详写复杂或有坑的**）

### 3.1 `POST /auth/register` 注册

请求：

```json
{
  "phone": "13812345678",
  "password": "********",
  "name": "张三",
  "id_card": "110101199001011234",   // 样例值（虚构），仅示意格式
  "agreements": [{ "doc": "user_agreement", "version": "v1.0" },
                 { "doc": "privacy_policy",  "version": "v1.0" },
                 { "doc": "refund_policy",   "version": "v1.0" }]
}
```

- **`id_card` 只在请求体**，绝不进 query / URL / Referer（服务端与客户端双重断言）。
- 📌 **文档中出现的样例号码必须带"样例"标记**（同一行内），
  这是 `deploy/scripts/scan-sensitive.sh` 的放行约定 —— 见 AGENTS.md 铁律 3。
  上面的 `// 样例值（虚构）` 就是该标记；**写文档时不要漏，否则合规门禁会拦下**。
- 服务端：手机号仅做格式校验，**不调用短信服务**；注册后状态为 `pending_review`，由运营管理员审核手机号后才可登录。
- `id_card` 为可选字段；填写时校验校验位并以 HMAC + AES-256-GCM 保存，未填写也可完成注册。
- 响应：`data = { "student_no": "...", "name": "...", "phone": "138****5678", "status": "pending_review" }`，不返回会话 token。
- 错误：`10103` 手机号已注册 · `10104` 身份证号已注册 · `10105` 身份证号校验位错误。

注册审核：`GET /admin/registration-pending` 返回待审学员数量，运营后台每 30 秒刷新待办提示；`/admin/users` 将待审学员排在前面。`POST /admin/users/{id}/approve-registration`，运营管理员或超级管理员必须提交 `reason` 与 `confirm=true`；成功后写入审核人、审核时间、`phone_verified_at` 与审计日志。密码正确但待审时登录返回 `10108`。

### 3.2 `POST /auth/login` 登录

```json
{ "mode": "sms",  "phone": "13812345678", "sms_code": "123456", "captcha_id": "...", "captcha": "..." }
{ "mode": "password", "phone": "13812345678", "password": "********", "captcha_id": "...", "captcha": "..." }
```

- 两种 `mode` 都**必须带图形验证码**（防手机号撞库）。
- 连续失败 5 次 → 账号 + IP 双维度锁定 10 分钟，返回 `10102`。
- 响应：`data = { "session": "<token>", "role": "student", "profile": {...脱敏} }`。
- 注册审核状态不是 `approved` 的账号禁止登录。

### 3.3 `GET /assets/{assetId}/url` 资源签名地址

- 服务端校验：① 登录；② 该资源所属课程的**权益是否解锁**（未解锁 → `10203`）。
- 返回：`data = { "url": "https://...&sign=...", "expires_in": 600 }`，**有效期 10 分钟**。
- **试学讲次**：即使未解锁也允许换取（管理员配置的试学讲次）。

### 3.4 `POST /orders` 下单（**幂等，直接对应"防重复付款"**）

```json
{ "course_code": "07-CN", "channel": "wechat", "request_id": "<uuid，客户端生成>" }
```

服务端行为（**必须按此顺序**）：

1. 校验权益：**已解锁** → 返回 `10302`（不必重复购买）；
2. 查 `(student_id, course_code, status='pending')`：
   **存在则直接返回该 `order_no`**（不新建、不改价）；
3. 不存在则快照**当前课程名与价格**建单，`expires_at = now + 30min`；
4. 并发时由**部分唯一索引**兜底：冲突则重查并返回既有订单（**不要报错给用户**）。

响应：`data = { "order_no": "...", "amount_cents": 3900, "expires_at": "..." }`。

### 3.5 `POST /orders/{orderNo}/refund` 学员自助退款

- **窗口**：未参加测评 **7 日内**，或已考未发证 → 允许；
  **已发证** → 返回 `10305`「已发证，需联系客服」。
- 通过后：**撤销权益 + 若已发证则连带撤销证书**（同一事务）；
  **原证书编号永久保持「已撤销」，不复活、不复用**。
- 响应：`data = { "refund_no": "...", "amount_cents": 3900, "status": "processing" }`。

### 3.6 支付回调（**验签 + 幂等，资金安全的核心**）

`POST /pay/wechat/notify` · `POST /pay/alipay/notify`

处理顺序（**不得颠倒**）：

1. **验签**（微信 APIv3 平台证书 / 支付宝公钥）→ 失败：**返回失败并按告警处理**，不写业务数据；
2. **金额二次核对**：渠道金额 vs 订单金额 → 不一致：**不开通**，记异常并告警；
3. **幂等**：以渠道交易号为唯一键插入流水；**冲突即视为已处理**，直接返回成功；
4. **开通权益 + 写通知**（同事务）；
5. **必须响应渠道要求的成功报文**，否则渠道会重试（重试是正常的，靠幂等兜住）。

- ⚠️ **回调不携带用户手机号** → **对账一律以「订单号 + 渠道交易号」为准**，不得用手机号。
- 前端轮询 `GET /orders/{orderNo}` 只作**辅助**，不作为开通依据。

### 3.7 `POST /ai/sessions/{id}/messages` 提问（**SSE 流式**）

请求：`{ "content": "CRC 怎么算？", "lecture_no": 3 }`

响应：`Content-Type: text/event-stream`，事件序列**固定为**：

```
event: meta
data: {"message_id":"...","quota_left":19}

event: delta
data: {"text":"CRC "}

event: delta
data: {"text":"是校验码……"}

event: citations
data: {"citations":[{"lecture_no":3,"source":"讲义-03.md","title":"差错检测"},
                    {"lecture_no":3,"source":"冻结输出-03-02.txt","title":"CRC 手算"}]}

event: done
data: {"tokens_in":812,"tokens_out":256}
```

**硬约定（对应 PRD §7.5 与铁律"AI 有据才答"）**：

| 情形 | 行为 |
|---|---|
| 检索无命中 / 模型给不出引用 | **不发 `citations`**，改发 `event: fallback`，前端展示"这门课的资料里没有这条" |
| 与课程无关 | `event: refused` + `10604` 语义，前端引导回课程 |
| 测评进行中 | 直接 `10603`（**入口也应在前端关闭**） |
| 额度用尽 | `10601`，`msg` 提示下次可用时间 |
| 内容审核拦截 | `10604` |

- **`citations` 是硬门禁**：解析不出引用的回答**不得下发给学员**（网关层拦，不是前端拦）。

### 3.8 `GET /public/status` 对外快照（**供 CCIT 官网**）

```json
{ "code": 0, "data": {
  "source": "cw-tc",
  "observedAt": "2026-10-03T15:04:05+08:00",
  "status": "available",
  "dataSource": "db",
  "courses": { "total": 7, "published": 7, "byMajor": { "网络工程": 1 } },
  "certificates": { "issued": 128, "revoked": 3 },
  "learners": { "registered": 96 }
}}
```

- **硬约束**：**不得**出现姓名、身份证号（含脱敏）、手机号、订单金额明细。
- **不可用时如实返回** `status: "unavailable"`，**不得伪造健康**。
- ⚠️ **接入数据源前**各计数返回 **`null` 而非 `0`**（`0` 会被读成"真的没有课程"，属编数）。

### 3.9 `GET /verify/{certNo}` 公开验真（四态）

```json
{ "code": 0, "data": {
  "cert_no": "CW-CN-M202600137",
  "state": "valid",
  "course": "计算机网络", "level": "L2", "band": "良好",
  "holder_masked": "张*",
  "issued_at": "2026-10-03", "expires_at": "2029-10-03",
  "issuer": "滁州畅网智能科技有限公司",
  "disclosure": "本证书所载身份信息由持证人自行申报，平台已留存申报记录，未接入官方身份核验系统。"
}}
```

- `state` 取值：**`valid` / `expired` / `revoked` / `not_found`**。
  **已过期绝不能返回 `valid`**（宁严勿宽）。
- `holder_masked` **只给姓氏级信息**（如 `张*`），**不回显身份证号任何片段**。
- `disclosure` 字段**必须原样出现在页面与 API**（平台认证口径的如实披露，PRD v2 §9.5）。

---

### 3.10 对外展示三接口（**v2.13，落地页与官网共用**）

**（一）`GET /api/v1/public/overview`**

```json
{
  "code": 0,
  "data": {
    "source": "cw-tc",
    "contentVersion": 7,
    "updatedAt": "<RFC3339>",
    "intro": { "short": "...", "para": "...", "long": "..." },
    "highlights": [
      { "slot": 1, "title": "AI 陪学，答得出来源", "body": "...", "evidenceRef": "/learn/{code}/{n}" }
    ],
    "majors": [ { "id": 1, "name": "软件工程", "courseCodes": ["02-DO"] } ],
    "aiTech": [ { "title": "统一 AI 网关", "body": "...", "evidenceRef": "PRD §11.3" } ],
    "filingNote": {
      "modelName": "<已备案模型名称>",
      "filingNo": "<备案号或上线登记编号>",
      "text": "本平台生成式人工智能服务使用的模型已完成备案，备案信息可查。"
    },
    "achievability": {
      "issuer": "畅网智能科技",
      "certNoRule": "CW-{课程码}-{级别位}{年份}{流水号6位}",
      "bands": { "L1": { "pass": 60 }, "L2": { "pass": 70 }, "L3": { "pass": 80 } },
      "stats": { "courses": 7, "lectures": 96, "questions": 1204, "statsAt": "<date>" }
    }
  }
}
```

- **`filingNote` 是法定公示义务**（不是营销文案）：国家网信办公告要求"已上线的生成式人工智能应用或功能，
  应在显著位置或产品详情页面公示所使用已备案或登记生成式人工智能服务情况，
  **注明模型名称、备案号或上线编号**" —— 故它**由 `web_content.block='filing_note'` 提供，管理员可改，但不能为空**。
- **`evidenceRef` 一律非空** —— 后端发布时校验；**返回体里带不出"没有依据的亮点"**。
- **`stats` 由上游只读 JSON 现场计算**，**不落人工维护的字段**；`statsAt` 标明数据截至。

**（二）`GET /api/v1/public/job-map?majorId=&courseCode=`**

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "courseCode": "05-OS",
        "certLevel": "L3",
        "jobDirection": "系统软件与运维方向 · 操作系统架构师",
        "skillItems": ["能做设计", "能定责", "能跨层综合"],
        "lectureRefs": ["05-OS/17", "05-OS/21", "05-OS/22"],
        "evidenceRef": "证书名 + 课程大纲:64"
      }
    ]
  }
}
```

- **`lectureRefs` 为空数组的条目不会被返回**（后端过滤，不靠前端）。
- **不得返回就业率 / 薪资字段** —— **接口层面就没有这些字段**，从根上避免被拿去做宣传。

**（三）`GET /api/v1/public/showcase?kind=&page=1&pageSize=20`**

```json
{
  "code": 0,
  "data": {
    "page": 1, "pageSize": 20, "total": 0,
    "items": [
      {
        "kind": "cert_earned",
        "nickname": "小舟",
        "majorName": "软件工程",
        "certLevel": "L2",
        "certNoMasked": "CW-CN-M2026****37",
        "summary": "...",
        "verifiedAt": "<RFC3339>",
        "companyName": null
      }
    ],
    "notice": "以上为经学员授权展示的真实成果，非全部学员。"
  }
}
```

- **永不返回**：真名 · 身份证号（含脱敏）· 手机号（含脱敏）· 薪资 ·
  `display_consent_at` / `consent_revoked_at` 原始值（对外只反映"是否展示"）。
- **`companyName` 默认 `null`**，仅在学员**单独授权**企业名时返回。
- **`notice` 字段是固定的**（防幸存者偏差，PRD §9.12（二）），**不允许被文案覆盖**。
- **撤回后 24h 内**：该条目 `status=withdrawn`，接口立即取不到，并**主动失效 CDN / 服务端缓存**。

---

## 4. 与前端的两条约定（避免扯皮）

1. **跨端表现可差异，正文与数字必须一致**（PRD §10.3）：
   HTML 课件在 **App 走 WebView**、Web 走 iframe；接口返回的**内容字段完全相同**。
2. **前端不实现业务规则**：价格、是否解锁、能否退款、能否发证，
   一律**以后端判断为准**；前端只做展示与"提前禁用"体验优化。

---

## 5. 待补（明确的后续动作）

| # | 事项 | 何时 |
|---|---|---|
| 1 | 支付渠道**沙箱地址与证书配置**参数 | M2（拿到商户号后） |
| 2 | 各接口的**示例响应**逐一补齐（本文只详写复杂接口） | M1 实现时同步补 |
| 3 | OpenAPI 3.1 规范文件 `docs/openapi.yaml`（由本文生成，供两端生成 SDK） | M1 末 |
| 4 | 压测接口清单与目标 QPS | M6 前 |
| 5 | **教师端字段白名单的落地检查脚本**（遍历教师接口，正则扫身份证号 / 手机号） | M5（v2.5 新增） |
| 6 | **`/admin/users` 列表与详情的脱敏检查**（同上门禁，扩到管理员接口） | M5（v2.6 新增） |
| 7 | **六维打分的「材料完整性」自动给分规则**（九件套齐备度 → 0–25 分的映射表） | M5（v2.7 新增） |
| 8 | **交错序列校验规则**（`lecture.kind` 怎样算"交错"：连续理论讲数上限？） | M5（v2.7 新增） |
| 9 | **六项统计的跑批 SQL 与幂等校验脚本**（§7.14 M-0） | M5（v2.8 新增） |
| 10 | **AI 运营周报的 Prompt 模板与 `evidence_map` 回填规则**（保证"每条建议可溯源"） | M5（v2.8 新增） |
| 11 | **评论内容安全接口选型**（腾讯云天御 / 其他，与 §9.10 机审对应） | M5（v2.8 新增） |

---

**（API 接口参考 v1.8 结束）**
