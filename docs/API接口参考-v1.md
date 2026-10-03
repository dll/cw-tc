# 第三课堂 · API 接口参考（v1）

> 配套：`docs/PRD-第三课堂-v2.md`（需求）· `docs/数据字典-v1.md`（字段）。
> 本文是**前后端并行开发的对齐基线** —— 契约先定，两端各写各的，最后对接。

| 项 | 内容 |
|---|---|
| 版本 | **v1.1**（v2.5 角色补齐：新增学员侧模块 L、教师申请、管理员分级与配置） |
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
| **公开（匿名）** | 无 | `/health` · `/public/status` · `/app/latest` · `/verify/{certNo}` · `/majors` · `/courses`（仅上架课程的公开字段） |
| **学员（需登录）** | 会话 | `/me/*` · `/orders` · `/exams` · `/ai/*` · `/notes` · `/me/certs` |
| **教师（角色）** | 会话 + `role=teacher` | `/teacher/**`（**且必须校验资源归属：只能操作自己的课程**）；**响应体须过滤身份证号与手机号**（§1.9） |
| **运营管理员** | 会话 + `role=admin_ops` | `/admin/**`（**不含** `/admin/admins` · `/admin/settings` · `/admin/exports`） |
| **超级管理员** | 会话 + `role=admin_super` | `/admin/**` 全部 |
| **渠道回调（匿名但有签名）** | **验签** | `/pay/wechat/notify` · `/pay/alipay/notify` |

- **权限判定顺序**：① 是否登录 → ② 角色是否匹配 → ③ **资源是否属于自己**（教师必需）。
- **前端只做菜单可见性，不作为安全边界** —— 越权必须由后端拒绝。

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
| `/ai/**` | 按「学员 × 课程」额度；同时限制并发 1 路/学员 |

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
| POST | `/auth/password/reset` | 公开 | 短信验证后重置密码（**不暴露是否已注册**） |
| GET | `/me` | 登录 | 我的资料（**身份证号与手机号均脱敏**） |
| PATCH | `/me/password` | 登录 | 改密码（需旧密码或短信二次验证） |
| POST | `/me/phone/rebind` | 登录 | 换绑手机号（**旧号 + 新号双向验证**） |
| POST | `/me/delete` | 登录 | 注销申请（人工审核，证书留存） |

### 2.2 课程与专业（模块 B）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/majors` | 公开 | 专业字典（启用中，按 `sort`） |
| GET | `/courses` | 公开 | 课程广场：`major_id` / `line`(main\|aux) / `q` / 分页；**只返回已上架** |
| GET | `/courses/{code}` | 可选登录 | 详情（价格、三档证书、试学讲次、**我的权益状态**） |
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
| POST | `/pay/wechat/native` | 登录 | 取微信 Native 支付二维码 |
| POST | `/pay/alipay/precreate` | 登录 | 取支付宝二维码 |
| POST | `/pay/wechat/notify` | **验签** | 微信异步回调（见 §3.6） |
| POST | `/pay/alipay/notify` | **验签** | 支付宝异步回调 |

### 2.5 测评与成绩（模块 C）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/exams` | 登录 | 开考（组卷；**需权益解锁**） |
| GET | `/exams/{id}` | 登录 | 取卷（**不返回参考答案**） |
| PATCH | `/exams/{id}/answers` | 登录 | 暂存答案（断网可恢复） |
| POST | `/exams/{id}/submit` | 登录 | 交卷 → 客观题即时判分 |
| GET | `/exams/{id}/result` | 登录 | 成绩单（分档 + 口径说明） |
| GET | `/me/scores` | 登录 | 我的成绩列表 |

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
| POST | `/teacher/courses/{code}/submit` | 教师+归属 | 提交审核 |
| GET | `/teacher/reviews` | 教师 | 待阅卷列表 |
| POST | `/teacher/reviews/{id}/score` | 教师 | 采点给分（双人阅卷） |
| GET | `/teacher/courses/{code}/stats` | 教师+归属 | 本课学情（**仅聚合，字段白名单见 §1.9**） |
| GET | `/teacher/courses/{code}/students` | 教师+归属 | 本课学员列表（**只回 `name` / `student_no` 与进度**，§1.9） |
| POST | `/teacher/courses/{code}/requests` | 教师+归属 | **发起下架 / 内容纠错申请**（`kind=下架\|紧急纠错`，§7.9 J-8） |
| GET | `/teacher/requests` | 教师 | 我的申请与处理状态 |

### 2.11 管理后台（模块 E）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/admin/courses` | 管理员 | 全部课程（含未上架 / 待审） |
| PATCH | `/admin/courses/{code}` | 管理员 | **课程设置面板**（价格、专业、上架状态、试学讲次、AI 开关与额度…） |
| POST | `/admin/courses/{code}/review` | 管理员 | 审核教师提交（通过 → 待上架 / 驳回） |
| POST | `/admin/courses/{code}/kb/rebuild` | 管理员 | 触发知识库重建 |
| GET | `/admin/kb/status` | 管理员 | 知识库状态（**切片数 / 向量数 / 最后构建时间**） |
| GET | `/admin/majors` · POST · PATCH | 管理员 | 专业字典维护 |
| GET | `/admin/orders` | 管理员 | 订单查询（筛选 + 导出 CSV） |
| POST | `/admin/orders/{orderNo}/refund` | 管理员 | 管理员退款（**连带撤销证书**） |
| GET | `/admin/reconcile` | 管理员 | 对账结果（**差异必须为 0**） |
| GET | `/admin/certs` | 管理员 | 证书台账 |
| POST | `/admin/certs/{certNo}/revoke` | 管理员 | 撤销证书（须填原因） |
| GET | `/admin/users` · PATCH `/{id}/status` | 管理员 | 用户管理 |
| POST | `/admin/users/teachers` | 管理员 | **开通教师账号** |
| POST | `/admin/users/{id}/transfer-courses` | 管理员 | **教师停用前的课程移交**（`to_teacher_id`，§7.9 J-9） |
| GET · PATCH | `/admin/settings` | **仅 super** | **系统配置**：业务参数可改；**密钥只读状态**（§7.10.6） |
| GET · POST | `/admin/admins` | **仅 super** | 管理员列表 / **创建管理员**（分级 §7.10.5） |
| DELETE | `/admin/admins/{id}` | **仅 super** | 停用管理员（**禁止停用最后一个超管**） |
| POST | `/admin/batch/courses` | 管理员 | **批量上架 / 下架 / 改价 / 挂专业**（先预览，§7.10.8） |
| GET | `/admin/batch/{batchId}` | 管理员 | 批量结果**逐条明细**（可导出） |
| POST | `/admin/exports` | **仅 super** | **代学员导出数据**（须填事由；写审计，§7.10.9） |
| GET | `/admin/requests` | 管理员 | 教师下架 / 纠错申请待办（§7.9 J-8） |
| GET | `/admin/broadcasts/preview` | 管理员 | **广播预览：返回触达人数** |
| POST | `/admin/broadcasts` | 管理员 | 发送广播 |
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
| POST | `/me/export` | 学员 | **导出我的数据**（JSON + MD 摘要，脱敏，写审计，§7.13 L-8） |
| GET | `/me/appeals` · POST | 学员 | 申诉（改姓名 / 身份证、成绩申诉）与处理状态 |

> **`/me/certs/{certNo}/file` 的硬约束**：**证书已撤销时返回 404**（不是 403 ——
> 避免暴露"这个编号曾经存在"之外的更多信息）；**重新下载必须返回与首次逐字节一致的文件**。

### 2.12 对外与运维（模块 K）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/health` | 公开 | 健康检查（部署探活依赖） |
| GET | `/public/status` | 公开 | **CCIT 官网用的聚合快照**（**不含任何个人信息**） |
| GET | `/app/latest` | 公开 | APK 最新版本（`versionCode` / 下载地址 / **SHA-256**） |

---

## 3. 关键接口详述（**只详写复杂或有坑的**）

### 3.1 `POST /auth/register` 注册

请求：

```json
{
  "phone": "13812345678",
  "sms_code": "123456",
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
- 服务端：校验校验位 → 算 `id_card_hash` → AES 加密存密文；返回**不含明文的**资料。
- 响应：`data = { "student_no": "TC2026-000137", "session": "<token>" }`。
- 错误：`10103` 手机号已注册 · `10104` 身份证号已注册 · `10105` 校验位错 · `10101` 验证码错。

### 3.2 `POST /auth/login` 登录

```json
{ "mode": "sms",  "phone": "13812345678", "sms_code": "123456", "captcha_id": "...", "captcha": "..." }
{ "mode": "password", "phone": "13812345678", "password": "********", "captcha_id": "...", "captcha": "..." }
```

- 两种 `mode` 都**必须带图形验证码**（防手机号撞库）。
- 连续失败 5 次 → 账号 + IP 双维度锁定 10 分钟，返回 `10102`。
- 响应：`data = { "session": "<token>", "role": "student", "profile": {...脱敏} }`。

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

---

**（API 接口参考 v1.1 结束）**
