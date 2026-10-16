# Hermes + 飞书 + 测试回复 v1

> **主题**：豆包账号租户下 `CCIT-OPC公司群` 机器人失效与修复复盘 —— 从 Hermes 安装、飞书建应用与建群、配置成功、开发 cw-tc，到群回复失效、三日修复，再到**六应用全量迁入畅网**
> **版本**：v1.3　**日期**：2026-10-16　**状态**：✅ 6 个应用全部迁入畅网 + `Hermes-CCIT` 群；**六路群内 `@`→回复全部实测通过**（§11.7）
> **范围**：Hermes 网关的飞书通道（默认 profile 及 `co`/`dev`/`pm`/`qa`/`cso` 共 6 个 profile）
> **不涉及**：cw-tc 业务代码本身

---

## 0. 一页摘要

| 项 | 内容 |
|---|---|
| **现象** | 飞书群 `@机器人` 无回复；客户端表现为「图标出现 → 消失 → 没回复」 |
| **直接原因** | 取 `tenant_access_token` 返回 `21001 Access denied by access control policy` |
| **根因一** | 网关 5 个自建应用**建在个人版身份「豆包账号」（非企业租户）下**，飞书侧对该类身份的应用执行访问控制策略 |
| **根因二** | 迁到企业租户后，`FEISHU_ALLOWED_USERS` 为空 ⇒ 授权 allowlist 语义下**谁都不许** ⇒ 消息收得到但被丢弃 |
| **修复** | 在已认证企业「畅网」下新建应用（**6 个**）＋ 新建群 **Hermes-CCIT** ＋ 补齐权限/事件/机器人能力/重新发版 ＋ 各填**该应用下**的授权名单 |
| **验证** | CEO 群里 `@` → 收到 → 生成 → 回复成功（10-15 00:44）；v1.2 六应用配置**六项全绿**（§11.2）；**v1.3 六路群内 `@`→回复全部实测通过**（§11.7） |
| **修复面扩展（v1.2）** | 默认 + `co`/`dev`/`pm`/`qa` 共 5 个旧应用全换畅网新应用，并**新增 `cso`**（第 6 个 profile）；旧 5 应用（`cli_aa48ee001278dcb8` 等）遗留豆包账号，仍 21001（§10） |
| **迁移二次踩坑** | 新应用常见三连坑：漏发 `im:message` 权限（`99991672`）、未启用机器人能力（`230006`）、授权名单仍写**旧 app 的 open_id**（`Unauthorized user`） |

> **一句话**：这不是本机故障，也不是代码故障 —— 是**「应用归属的租户身份」被飞书策略收紧**，叠加迁租户后暴露的**授权名单为空**。两层的表现完全不同（前者报 21001，后者日志写 `Unauthorized user`），必须分开诊断。

---

## 1. 背景

**Hermes** 是运行在本机的 AI 代理网关，把飞书群当作「指令通道」：群里 `@` 某个机器人（CEO / CO / DEV / PM / QA / CSO），对应的 agent 就会接活、调模型、把结果回帖到群里。

**cw-tc（第三课堂）** 这个项目的开发就是走这条通道的：在 `Hermes-CCIT` / `CCIT-OPC公司群` 里下指令，agent 在本地 `E:\2026-2027\2026-2027-1\MyProjects\cw-tc` 改代码、跑门禁、提交推送。

所以「群里不回话」= **开发通道断了**，不是一个小功能问题。

本机环境（排查期间实测）：

| 项 | 值 |
|---|---|
| Hermes home | `C:\Users\ldl\AppData\Local\hermes` |
| 网关启动方式 | Windows 计划任务 `Hermes_Gateway`（**不是** `hermes gateway start`，见 §7.2） |
| 网关卡 | `bin/hermes.exe gateway` |
| profile 数 | 6（`default` + `co`/`dev`/`pm`/`qa`/`cso`） |
| 每个 profile | 一个**独立的飞书自建应用**（独立 App ID / Secret） |

---

## 2. 系统结构

```
                     Feishu Open Platform
              tenant A: 豆包账号(个人版)   tenant B: 畅网(已认证企业)
                          |                        |
        events = websocket 长连接  |  api = REST(取 token / 发消息)
                          v                        ^
          +-------------------------------------------------+
          |         Hermes Gateway  (schtask Hermes_Gateway) |
          |  profiles: default  co  dev  pm  qa  cso         |
          |  每个 profile = 一个飞书自建应用 = 一套 App ID     |
          +-------------------------------------------------+
                          |
                          v
               agent runtime  ->  LLM  ->  生成回复
```

**关键不对称（本故障的核心知识点）**：

| 腿 | 通道 | 用什么鉴权 | 挂了的表现 |
|---|---|---|---|
| **接收** | websocket 长连接 | 应用凭证建连（建好后一直连着） | 日志零 `Inbound` |
| **发送** | REST | **每次重新取 `tenant_access_token`** | 日志 `Send attempt 1/3 failed` + `21001` |
| **事件订阅** | 长连接上的订阅项 | 应用侧的 `im.message.receive_v1` | 同上，日志零 `Inbound` |
| **授权** | 网关本地判定 | `.env` 的 `FEISHU_ALLOWED_USERS` | 日志 `Unauthorized user` |

> **教训**：这四条腿**互相独立**。本故障先后**各挂了一条**（先是发送腿的 21001，迁租户后又撞上授权腿），所以「换了应用就好了」这个直觉会把第二层根因藏起来。

---

## 3. 时间线（三日复盘）

> 时间戳取自 `logs/gateway.log`（本机时钟）。已复核本机时钟与飞书服务器 `Date` 头一致（±2s），时间线可信。
> ⚠️ 本机 Windows 时间服务（`w32time`）当前为 **Stopped** —— 排查中曾据此核对过时钟，后续建议常开（§9.4）。

### 3.1 建设期：安装 → 建应用 → 建群 → 配置成功（10-04 ~ 10-06）

| 时间 | 事件 | 证据 |
|---|---|---|
| 10-04 20:48 | 飞书通道**首次成功回复**（私聊） | `response ready: chat=oc_6260a31c...` |
| 10-05 05:24 / 05:30 | **群** `oc_76dc9858...`（`CCIT-OPC公司群`）回复成功 | `response ready: chat=oc_76dc9858...` |
| 10-05 22:10 ~ 22:16 | 连续 4 次网关重启（调试期） | 4 条 `Starting Hermes Gateway...` |
| 10-06 08:12 ~ 08:25 | 3 次网关启动 + 凭据写入（`.env` 落定） | `Starting Hermes Gateway` ×3 |

**结果**：5 个应用（default/co/dev/pm/qa）全部接通，群内收发正常。

### 3.2 使用期：开发 cw-tc，群内正常（10-11 ~ 10-12）

| 时间 | 事件 | 证据 |
|---|---|---|
| 10-11 12:04 | 电脑开机 | `LastBootUpTime = 2026-10-11 12:04:28` |
| 10-11 12:16 | 网关随计划任务启动 | `Starting Hermes Gateway... 12:16:13` |
| 10-12 19:39 ~ 20:39 | **最后一段正常收发**（群内多轮问答） | `response ready` + 发送成功，**无 21001** |

### 3.3 失效点：10-13 17:25:56

| 时间 | 事件 | 证据 |
|---|---|---|
| 10-13 17:25:56 | **首次 21001**，发送 3 次重试 + 纯文本兜底全失败 | `Send attempt 1/3 failed for chat oc_76dc9858...` |
| 10-13 17:55 | 收到「测试回复」→ 生成回复（42 字符，14.2s）→ **发送全挂** | `response ready` 有、发送无 |
| 10-13 17:25 ~ 23:02 | 当天排查（587 条日志，全窗口最密集） | — |

> **注意**：这一次的「没回复」**不是 agent 没答**，是**答了发不出去**。这也是当时最容易误判的地方（见 §8.1）。

### 3.4 修复期 D1（10-13）：定位到 21001，误判为 IP 白名单

- 拉全量错误：`obtain self tenant access token failed, code: 21001, msg: Access denied by access control policy.`
- 提出假设：出口 IP 未加白名单 → 实测**把出口 IP 加白后仍 21001**，**假设当场被推翻**
- 关键纠正：**IP 白名单的错误码是 `99991401`**，且飞书官方明确「IP 白名单对 access_token 相关接口不生效」

### 3.5 修复期 D2（10-14）：差分探针 → 锁定「租户身份」

| 探针 | 结果 | 排除掉什么 |
|---|---|---|
| **凭据差分**：真凭据 / 真 app_id+错 secret / 假 app_id | `21001` / `10014 app secret invalid` / `10003 invalid param` | 凭据**有效**，是策略层拒绝 |
| **全应用扫**：5 个 profile 的 App ID 逐个打 token | **全部 `21001`** | 不是某个应用的配置问题 |
| **IP 无关性**：直连 vs 走本地代理 | 两个出口 IP **都 `21001`** | 与 **IP / 网络 / 加速器**彻底无关 |
| **重启假设**：开机时间 / 网关进程起点 | 10-11 12:04 开机、12:16 起进程，**10-13 当天未重启** | 与「重启电脑」**无关**（失效距重启已 ~29 小时） |

**结论**：全应用 + 多 IP 同挂 ⇒ **账号/租户级**访问控制，落点在**飞书侧**。

随后用浏览器登 `open.feishu.cn/app` 对比归属标签，根因显形：网关用的 5 个应用都在**个人版身份「豆包账号」**下，而非企业租户。

### 3.6 修复期 D3（10-15）：换租户 + 补授权，成功

| 步骤 | 动作 | 实测结果 |
|---|---|---|
| 1 | 在**已认证企业「畅网」**下新建应用 `Hermes-CEO`（`cli_aa451b7b4bb8dc0f`） | token 探针 → **`code:0`** ✅ |
| 2 | 写入网关凭据（原 `.env` 已备份 `.env.bak-20261015000927`） | — |
| 3 | 新建群 `Hermes-CCIT`（`oc_6c93e87b...`），把机器人拉进群 | `GET /im/v1/chats` → 群在列表内 ✅ |
| 4 | 补齐权限 5 项 + 事件 `im.message.receive_v1` + **重新发版** | 入站日志出现 `Inbound group message received` ✅ |
| 5 | 应用身份发消息自检 | `code:0 success` ✅ |
| 6 | 填 `FEISHU_ALLOWED_USERS` = 你的**新** open_id | `21001` 归零、`Unauthorized` 归零 ✅ |
| 7 | `FEISHU_HOME_CHANNEL` 指向新群 | — |

**最终验收（10-15 00:44，日志原文）**：

```
00:44:00  [Feishu] Inbound group message received: chat_id=oc_6c93e87b... text='发一句「测试」'
00:44:04  inbound message: platform=feishu user=ou_85c77a4a... chat=oc_6c93e87b...
00:44:25  response ready: ... time=21.3s api_calls=1
```

切换新应用后：**`21001` 出现 0 次**、**`Unauthorized user` 出现 0 次**（最后一条 21001 停在 `2026-10-14 17:52:15`）。

---

## 4. 根因分析

### 4.1 根因一：应用建在个人版租户下，被飞书访问控制策略拦截（21001）

**机制**：网关只用**应用凭证**直连飞书服务端取 token、发消息 —— 与「飞书客户端登录的是哪个账号」**无关**。真正决定成败的是**这个自建应用挂在哪个企业/团队（租户）下**。

个人版身份的团队：飞书限制其开放能力（「对外共享」在未认证时直接置灰），且策略会**随时间收紧** —— 表现为「同一批应用、同一密钥、同一台机器，某天起突然全部 21001」。

**判定依据（三条探针互相印证）**：
1. 真凭据 → `21001`；**假 app_id → `10003`**；**真 app_id + 错 secret → `10014`** ⇒ 凭据本身有效，不是配错。
2. 5 个应用**全挂** ⇒ 不是单个应用的问题。
3. 直连与代理**两个出口 IP 都挂** ⇒ 与 IP / 网络无关。

> **为什么"以前是好的"**：飞书此前不拦这类身份的应用，**10-12 19:40 ~ 10-13 17:25 之间**上线/收紧了访问控制（策略灰度）。日志里最后一次成功发送与首次失败的边界，就是这次策略变更的落点。

### 4.2 根因二：`FEISHU_ALLOWED_USERS` 为空 = 谁都不许

迁到企业租户后，症状**换了**：消息**收得到**，但紧接着被网关丢弃：

```
WARNING gateway.run: Unauthorized user: ou_85c77a4a70a94aefdf68b027fee7750d (None) on feishu
```

判定源码（`installs/<id>/environments/<id>/workspace/gateway/authz_mixin.py`）：

```python
def _allows(allowed, candidate) -> bool:
    return "*" in allowed or candidate in allowed
```

⇒ **`FEISHU_ALLOWED_USERS` 是授权 allowlist，留空即集合为空，谁都不许。**

它与 `FEISHU_GROUP_POLICY`（「机器人是否在群里应答」的开关）**是两套独立的门**：群开关开着，allowlist 照样能把人拒掉。

**两个必须记住的细节**：

1. **`open_id` 是 per-app 的** —— 同一个人在不同应用下是**不同的 id**。换应用后，旧 allow list **全部失效**。
2. 日志里的 `(None)` 是**显示名没解析出来**（应用缺通讯录权限），**与判定无关**，别被它带偏。

> **这一层最阴的地方**：迁租户后，凭证 ✅ 长连接 ✅ 发送 API ✅ 事件订阅 ✅ —— **四个探针全绿，唯一断点在授权名单上**。如果只验前四条就宣布成功，用户会回你一句「还是不行」。

### 4.3 澄清：「重启电脑」不是原因

用户的第一直觉是「重启后失效」。用三条独立证据核过：

| 证据 | 值 |
|---|---|
| 开机时间 | `2026-10-11 12:04:28` |
| 网关进程起点 | `2026-10-11 12:16:13` |
| 重启后是否还能正常收发 | **能** —— 10-12 19:39 ~ 20:39 全程正常 |
| 首次 21001 | `2026-10-13 17:25:56`（**距重启约 29 小时**） |
| 当天是否有重启 | **没有**（进程 `uptime` 2.32 天） |

⇒ 重启与失效**只有时间上的巧合，没有因果**。变更发生在**飞书侧**。

---

## 5. 修复方案（可复用的施工单）

### 5.1 施工七步

| # | 步骤 | 要点 |
|---|---|---|
| 1 | 在**已认证企业**租户下新建自建应用 | 用「创建飞书 智能体 应用 → 立即创建」最快，**但它预置的权限/事件不全**，必须走第 4 步 |
| 2 | 记录 **App ID / App Secret** | Secret 只在控制台点「复制」；**绝不粘进对话、绝不提交仓库** |
| 3 | 建群 + 把机器人拉进群 | 群必须建在**应用所属的同一个租户**里 |
| 4 | **补权限 + 加事件 + 重新发版** | 见 §5.2，**改完必须重新发布版本才生效** |
| 5 | 写 `.env`：`FEISHU_APP_ID` / `FEISHU_APP_SECRET` | **`.env` 是受保护文件**，`patch`/`write_file` 会被拒，只能由 terminal 跑脚本改；先备份 |
| 6 | 写 `.env`：`FEISHU_ALLOWED_USERS` = **该应用下**用户本人的 open_id | 见 §5.3 取值方法 |
| 7 | 写 `.env`：`FEISHU_HOME_CHANNEL` = 新群 `oc_` id | **换 `FEISHU_APP_ID/SECRET` 必须重启网关**（适配器在启动时用凭据建连）；**仅改 `FEISHU_ALLOWED_USERS` 即时生效**（授权门 per-turn 读 + `.env` mtime 缓存失效，见 §11.4） |

### 5.2 新应用必须补齐的三样（一键创建不够用）

**① 权限管理** —— 开通（至少）：

| 权限 | 用途 |
|---|---|
| `im:message` | 获取与发送单聊、群组消息 |
| `im:message:send_as_bot` | 以应用的身份发消息 |
| `im:message.group_at_msg:readonly` | **接收群聊中 @机器人 的消息事件**（收消息必需） |
| `im:message.p2p_msg:readonly` | 读取用户发给机器人的单聊消息 |
| `im:chat:readonly` | 获取群组信息（`im/v1/chats` 探针要用） |

**② 事件与回调** —— 订阅方式选「**长连接**」，添加事件 `im.message.receive_v1`（接收消息）

> ⚠️ 飞书要求**检测到客户端已建立长连接**才允许保存「长连接」订阅方式 ⇒ **先把网关起起来，再去保存**。

**③ 版本管理与发布** —— 创建版本 → 提交发布

> ⚠️ **改权限/事件必须重新发版才生效**。这一步最容易漏，漏了前两步全白做。

### 5.3 取值与探针命令

```bash
cd ~/AppData/Local/hermes

# ① token 探针（只看响应码，secret 不打印）
APP_ID=$(grep -E '^FEISHU_APP_ID=' .env | tr -d '\r' | cut -d= -f2-)
APP_SECRET=$(grep -E '^FEISHU_APP_SECRET=' .env | tr -d '\r' | cut -d= -f2-)
curl -s --max-time 15 -X POST \
  'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal' \
  -H 'Content-Type: application/json' \
  -d "{\"app_id\":\"$APP_ID\",\"app_secret\":\"$APP_SECRET\"}" \
  | sed -E 's/"tenant_access_token":"[^"]*"/"tenant_access_token":"<redacted>"/'

# ② 用户在「该应用下」的 open_id —— 两个来源可互证
grep -oE "sender=user:ou_[a-f0-9]+" logs/gateway.log | tail -1        # 入站日志
# 或 GET /open-apis/im/v1/chats（需 im:chat:readonly）取 items[].owner_id
#    （用户自建的群，群主就是他本人，两值应一致）

# ③ 收发全链路唯一事实源
grep -nE "Inbound group message received|response ready|Send attempt|Unauthorized|21001" logs/gateway.log | tail -20
```

### 5.4 网关重启（Windows 专用，别踩坑）

```bash
H="C:/Users/ldl/AppData/Local/hermes/bin/hermes.exe"
"$H" gateway stop                                       # 干净停：drained cleanly
powershell -NoProfile -Command "Start-ScheduledTask -TaskName Hermes_Gateway"
```

- ⚠️ **别用 `hermes gateway start` 直接起**：子进程会被父 Job Object 连坐杀掉，命令自己报 `no stable gateway process detected` 并让你改用计划任务。
- ⚠️ **别在命令末尾串 `hermes gateway status`**：实测会挂住，把整条命令拖到超时。要看状态就读 `gateway_state.json` 与 `logs/gateway.log`。
- ⚠️ **重启后第一轮常出现「缺适配器」**：启动段若有 `WARNING gateway.run: No adapter available for feishu` + `Gateway running with 2 platform(s)`（正常是 5），**给它 1~3 分钟**等重连看守补上；实测干净重启第二次才补齐 —— **别反复重启**。
- ⚠️ **`gateway_state.json` 里每条记录带 `writer_pid`**：与当前 `gateway.pid` 不符的那条是**上一代网关的残留**，`state=connected` 是旧的 —— 别只读 `state`。

### 5.5 四条腿验收清单

| 腿 | 验法 | 通过标志 |
|---|---|---|
| **凭证** | token 探针（跑在**新 app_id** 上） | `code:0` |
| **连接** | 读日志 | `[Feishu] Connected in websocket mode (feishu)` |
| **事件订阅** | 群里 `@` 一句 → 读日志 | `Inbound group message received` |
| **授权** | 同上，看下一行 | **没有** `Unauthorized user` |
| **端到端** | 群里看回复 | `response ready` + 群里真出现回复 |

---

## 6. 错误码对照表（本次实际遇到）

| 码 | 含义 | 判定 |
|---|---|---|
| `21001` | `Access denied by access control policy` | **飞书侧策略拦截**（租户/身份级）。**不是**凭据错、**不是**代码问题 |
| `10003` | `invalid param` | app_id 不存在（用于**反证**凭据探针有效） |
| `10014` | `app secret invalid` | 密钥错（用于**反证**） |
| `99991401` | IP 白名单拦截 | ⚠️ **与 21001 不是一回事**；且**对 access_token 接口不生效** —— 别往这个方向修 |
| `99991672` | 缺少应用身份权限（scope） | 权限没开或**开了没重新发版** |
| `230006` | `Bot ability is not activated` | 新应用**没启用「机器人」能力**（v1.2 新增，见 §11.3） |

---

## 7. 走过的弯路（假设 → 探针 → 结论）

> 记下来，是因为这些假设在下次故障里**还会冒出来**。

| # | 假设 | 探针 | 结论 |
|---|---|---|---|
| 1 | 「出口 IP 没加白名单」 | 加白后重测 | ❌ **推翻**：仍 21001；IP 白名单错码是 `99991401`，且对该接口不生效 |
| 2 | 「需要补个人实名认证」 | 打开 `创建版本 → 对外共享` | ❌ **推翻**：两个勾选框 `disabled` 灰态，整页无认证入口（该身份根本没这条路） |
| 3 | 「把豆包账号加入企业就能好」 | 成员名单 + 重测 | ❌ **推翻**：企业成员名单里**没有**该账号；且「加人」不会迁移已有应用的归属 |
| 4 | 「转移应用所有权」 | 控制台实测搜账号 | ❌ **推翻**：只能搜**同租户**用户，返回「没有找到相关结果」 |
| 5 | 「重启电脑引起」 | 开机时间 / 进程 uptime | ❌ **推翻**：失效距重启 ~29 小时，且 10-13 当天未重启 |
| 6 | 「抖音加速器换了 IP」 | 直连 vs 代理双出口 | ❌ **推翻**：两个出口都 21001 |
| 7 | 「换到企业应用就完事」 | 群里实测 | ⚠️ **不完整**：撞上第二层（授权名单为空），还需要 §4.2 那一步 |
| 8 | 「控制台能脚本化建应用」 | 自动化填「应用名称」 | ❌ **不可行**：React 前端不认注入值，字符计数恒 `0/32` —— **需要打字的那几步只能用户本人做**；点选类操作可代劳 |

**方法论沉淀**：
- **先证明「不是本机」，再谈修**。三条只读探针（凭据差分 / 全应用扫 / IP 无关性）就能把范围钉死，避免用户白折腾。
- **结论必须能指回产出它的探针**。没有探针支撑的推断不要写成结论；被推翻的假设**显式撤回**，别静默换方向。
- **别把结论包装成「A 路 / B 路」让用户挑**。只剩一条可行路时，直接给结论 + 你立刻要做的动作，只把**必须用户本人做的那一步**单独点出来。
- **用户自己动手做的平台侧修复，按「待核实的说法」处理**，重跑探针 + 回控制台核对，别顺着说法说「应该好了」。

---

## 8. 遗留问题与后续动作

### 8.1 ✅ 分身已全量迁移（v1.2 完成；**v1.3 群内六路实测全通**，见 §11）

2026-10-15 晚，`co`/`dev`/`pm`/`qa` 四个分身全部换成畅网新应用，并**新增第 6 个 profile `cso`**。旧（豆包）应用 ID 仅留作对照：

| profile | 旧 App ID（豆包，弃用） | 新 App ID（畅网） | 状态 |
|---|---|---|---|
| `default` | `cli_aa48ee001278dcb8` | `cli_aa451b7b4bb8dc0f` | ✅ 已迁 |
| `co` | `cli_aa4916beb478dbed` | `cli_aa5a7b30adb9dbb5` | ✅ 已迁 |
| `dev` | `cli_aa491fc95338dcc6` | `cli_aa5a751bd8381cec` | ✅ 已迁 |
| `pm` | `cli_aa4922dfba38dbe3` | `cli_aa5a7633b6781d0e` | ✅ 已迁 |
| `qa` | `cli_aa491bb29cf89bd1` | `cli_aa5a77c6cfb89ce3` | ✅ 已迁 |
| `cso`（新增） | — | `cli_aa5a723b40781ce3` | ✅ 新建 |

### 8.2 ⚪ 旧群 `CCIT-OPC公司群`（历史遗留，已弃用；新群见 §11）

> 该群与群内 5 个旧应用仍留在豆包账号租户，仍 21001 —— **不再修复**，开发通道已整体切到 `Hermes-CCIT`。

群里能测的是 **CCIT-CEO**（`cli_aa48ee001278dcb8`，备份 `.env.bak-20261014-175947`）以及 `co`/`dev`/`pm`/`qa`。2026-10-15 02:20 五个全部 `21001`，发消息这一步走不到。

畅网的 Hermes-CEO 与这个群不同租户（`232010`），不能拿来顶替。`cli_aa2aa23469f8dbd6`（显示名 CCIT-CSO）不在这个群里，不能拿来测这个群。

### 8.3 🟡 本机时间服务未启动

`w32time` 状态 = **Stopped**。排查中需要靠飞书服务器 `Date` 响应头核对本机时钟（当前 ±2s，尚可）。
**动作**：启用并设自动同步，避免日志时间线整体偏移、把「首次报错时刻」算错。

### 8.4 🟡 `DEEPSEEK_API_KEY` 配置告警

网关启动时打印：

```
Config ref '${env:DEEPSEEK_API_KEY}': DEEPSEEK_API_KEY is not set (check ~/.hermes/.env);
keeping the literal placeholder
```

**实测不影响回复**（10-15 00:44 那次 `api_calls=1` 正常出答案），但值得查明取值上下文。
**动作**：核对 `config.yaml` 的引用与 `.env` 加载时序。

### 8.5 🟡 密钥与备份纪律（不可挽回项）

- **App Secret 只在 `.env`**：`patch`/`write_file` 对 `.env` 会被拒（`protected system/credential file`），只能由 terminal 跑脚本改 —— 全程**不 echo、不打印**，改完 `diff` 核对。
- **`.env` 改前必备份**：本次留存 `.env.bak-20261015000927`。
- **绝不进仓库**：`.env` 全文、任何 App Secret、token。

---

## 9. 附录

### 9.1 本次涉及的标识（可公开、不含任何密钥）

| 类别 | 值 |
|---|---|
| 企业租户 | 畅网（企业编号 `FM4LZVK30GM`，认证有效期至 2027-08-21） |
| 个人版身份 | 豆包账号（网关 5 个**旧**应用的归属） |
| 新群 | `Hermes-CCIT` / `oc_6c93e87bc97d9de2bf7e6621545d4c2b` |
| 旧群 | `CCIT-OPC公司群` / `oc_76dc985873e026f38d4c25914c3daddb` |

**畅网新应用（6 个）与其机器人 `open_id`**（`bot/v3/info` 实测）：

| profile | 应用 / 机器人 | App ID | bot open_id |
|---|---|---|---|
| `default` | Hermes-CEO | `cli_aa451b7b4bb8dc0f` | `ou_02dee9b1c4e0c9cae83c9814206705b2` |
| `co` | Hermes-CO | `cli_aa5a7b30adb9dbb5` | `ou_a235ac07e1b9b0cd5580d54090c9cf1e` |
| `dev` | Hermes-DEV | `cli_aa5a751bd8381cec` | `ou_286d0f1eac42b82f3b732b69c20b03ae` |
| `pm` | Hermes-PM | `cli_aa5a7633b6781d0e` | `ou_cce6fe166f28eeafef3e53f3747d7f19` |
| `qa` | Hermes-QA | `cli_aa5a77c6cfb89ce3` | `ou_3caaff4c793370eb6cab3c66e2f32d28` |
| `cso` | Hermes-CSO | `cli_aa5a723b40781ce3` | `ou_e84fc800c769371310ce469b7aa98c26` |

**「我」（用户本人）在每个应用下的 `open_id`**（**per-app，逐个不同**，各自写进该 profile 的 `FEISHU_ALLOWED_USERS`）：

| profile | 用户 open_id |
|---|---|
| `default` | `ou_85c77a4a70a94aefdf68b027fee7750d` |
| `co` | `ou_088a584c506521425cdc7a8e07f06c9a` |
| `dev` | `ou_245fa70e835074df3b6af782e287ac7f` |
| `pm` | `ou_7353d2df7780487b2761f0aaa5ff79aa` |
| `qa` | `ou_7e4d599d8a3f4cb4f40ca3f79bc1d88f` |
| `cso` | `ou_d246984d029a15c9a980e0485dd5dcba` |

> ⚠️ **App Secret / tenant_access_token 一律不在此文档中出现。**
> ⚠️ 上表两组 `ou_` **不可互相复制**（同一应用内才唯一）；换应用即失效，这是迁移期最大的坑（§4.2 / §11.3）。

### 9.2 关键文件位置

| 文件 | 作用 |
|---|---|
| `~/.hermes/.env` | 默认 profile 的飞书凭据与配置（受保护文件） |
| `~/AppData/Local/hermes/profiles/<name>/.env` | 各分身的独立凭据 |
| `~/AppData/Local/hermes/logs/gateway.log` | **收发全链路唯一事实源**（排查第一站） |
| `~/AppData/Local/hermes/gateway_state.json` | 各平台连接状态（**注意 `writer_pid`**） |
| `~/AppData/Local/hermes/channel_directory.json` | chat id → 群名映射 |
| `.../workspace/gateway/authz_mixin.py` | **授权判定源码**（`_allows`） |

### 9.3 排查起手式（三条腿，按顺序）

1. **收到没** → `Inbound group message received`
2. **生成没** → `response ready: ... api_calls=N`
3. **发出没** → `Sending response` 之后有无 `Send attempt 1/3 failed` / `21001`
4. **授权没** → 紧邻 `Inbound` 的那行 `Unauthorized user`

> **零 `Inbound`** ⇒ 不是发送挂，是**收不到**（权限/事件没配）—— 别去查网络和 21001。

### 9.4 一句话结论

> **「机器人不回话」有四种彼此独立的成因**：凭证被策略拦（21001）、事件订阅没生效（零 `Inbound`）、授权名单不含本人（`Unauthorized user`）、以及 agent 侧出错（有 `Inbound` 无 `response ready`）。
> **四条腿要一条一条验，不能靠"换了应用"这类直觉跳步。**

---

## 10. v1.1 复核（2026-10-15 02:20）：测错应用，已撤回

上一稿把 `.env.bak-20261015000927` 里的 `cli_aa2aa23469f8dbd6`（机器人显示名 **CCIT-CSO**）当成旧群机器人。**撤回。** CCIT-CSO 不在 `CCIT-OPC公司群`。那个备份是 10-15 00:09 切走之前的中间状态，不是群里的 CCIT-CEO。

群里要测的 5 个应用，凭据在 `.env.bak-20261014-175947`（CEO）和 `profiles/{co,dev,pm,qa}/.env`。02:20 重打 `tenant_access_token/internal`（未走系统代理）：

| 群内应用 | App ID | 结果 |
|---|---|---|
| CCIT-CEO | `cli_aa48ee001278dcb8` | **`21001`** |
| co | `cli_aa4916beb478dbed` | **`21001`** |
| dev | `cli_aa491fc95338dcc6` | **`21001`** |
| pm | `cli_aa4922dfba38dbe3` | **`21001`** |
| qa | `cli_aa491bb29cf89bd1` | **`21001`** |

五个都是同一句：`Access denied by access control policy. Contact the administrator for assistance.` 密钥被认出来了（否则是 `10014`），token 不发，所以发送探针做不了。这不是「没去测」，是测完发不出。

对照（都不是这个群的机器人）：

| 应用 | App ID | 结果 | 为何不能用来测这个群 |
|---|---|---|---|
| Hermes-CEO（现网关，畅网） | `cli_aa451b7b4bb8dc0f` | `code: 0` | `GET` 该群 → **`232010`** 不同租户 |
| CCIT-CSO | `cli_aa2aa23469f8dbd6` | `code: 0` | 发该群 → **`230002`** 不在群里 |

21001 仍不在官方[通用错误码表](https://open.feishu.cn/document/server-docs/api-call-guide/generic-error-code?lang=zh-CN)里。它和 `99991401`（IP 白名单，且不对 token 接口生效）、`99991672`（缺 scope，要先拿到 token）不是一回事。长连接可以还连着，发消息每次重新要 token，所以只有发送腿死。本机没有开关能放开这 5 个应用。

### 10.1 控制台复核（同日，豆包账号已登录）

开发者后台应用列表（`正式应用@豆包账号`，所有者刘老师）与网关凭据对得上，状态都是已启用、已发布：

| 名称 | App ID | 发布 |
|---|---|---|
| CCIT-CEO | `cli_aa48ee001278dcb8` | 2026-10-05 22:11 已发布，版本 1.0.3 |
| CCIT-CO | `cli_aa4916beb478dbed` | 2026-10-05 22:46 已发布 |
| CCIT-DEV | `cli_aa491fc95338dcc6` | 2026-10-05 21:20 已发布 |
| CCIT-PM | `cli_aa4922dfba38dbe3` | 2026-10-05 21:30 已发布 |
| CCIT-QA | `cli_aa491bb29cf89bd1` | 2026-10-05 21:37 已发布 |

同页还有 CCIT-CDO（`cli_aa45ef4aff781c01`，2026-10-14 发布），不在网关的 5 个 profile 里，没有用来测这个群。

安全设置里 IP 白名单不是已配置的拦截（错误码也不会是 21001）。`版本管理 → 创建版本`：「对外共享」两个勾选框为 disabled，整页没有「实名」。`admin.feishu.cn` 原文：「当前登录身份为刘老师（豆包账号），请创建企业以访问管理后台」——这个身份没有管理后台，没有访问控制开关可以关。畅网的 Hermes-CEO 页面返回 403（当前登录身份不是它的所有者）。

⇒ 这 5 个应用的 21001 不是停用、未发布、IP 白名单或本机配置。开发者后台里没有能让 token 重新签发的操作。

---

## 11. v1.2 ~ v1.3 迁移完成：6 应用全量迁入畅网并群内实测全通（2026-10-15 晚 ~ 10-16 早）

### 11.1 做了什么

- **新增第 6 个 profile `cso`**：`hermes profile create cso --clone-from co`（克隆 `config.yaml`/`.env`/`SOUL.md`/skills，**不带飞书通道** —— 两个 profile 共用一套 bot 凭据会互抢），模型沿用 `qwen-max`；自动生成别名 `~/.local/bin/cso.bat`。
- **`co`/`dev`/`pm`/`qa`/`cso` 五个 profile 全部换畅网新应用**：`FEISHU_APP_ID/SECRET` 换新，`FEISHU_DOMAIN/CONNECTION_MODE/GROUP_POLICY/REQUIRE_MENTION/ALLOWED_USERS/HOME_CHANNEL` 与 CEO 口径对齐。
- 应用 ID 与 open_id 全表见 §9.1。

### 11.2 六项核验矩阵（本机实测，**全绿**）

| 机器人 | App ID | token | 在群内 | 主动发信 | 授权名单匹配 | 长连接 |
|---|---|---|---|---|---|---|
| CEO | `cli_aa451b7b4bb8dc0f` | `0` | ✅ | `0` | ✅ | connected |
| co | `cli_aa5a7b30adb9dbb5` | `0` | ✅ | `0` | ✅ | connected |
| dev | `cli_aa5a751bd8381cec` | `0` | ✅ | `0` | ✅ | connected |
| pm | `cli_aa5a7633b6781d0e` | `0` | ✅ | `0` | ✅ | connected |
| qa | `cli_aa5a77c6cfb89ce3` | `0` | ✅ | `0` | ✅ | connected |
| cso | `cli_aa5a723b40781ce3` | `0` | ✅ | `0` | ✅ | connected |

> 其中「在群内 / 授权名单匹配」由 `GET /im/v1/chats/{chat_id}/members` 取群里真实 `open_id` 后与 `.env` 逐字比对得出 —— **不是靠"应该没问题"**。
> ⚠️ 表中「长连接 = connected」是**那一刻**的读数；改过 `.env` 后可能失效为 `0 adapter(s) connected`（§11.8 陷阱 B），**以当日该 profile 日志为准**。

### 11.3 迁移期新踩的三个坑（都在"新应用"上）

| # | 现象 / 错误码 | 真实原因 | 修法 |
|---|---|---|---|
| 1 | 发消息 `99991672`（缺 `im:message` / `im:message:send` / `im:message:send_as_bot`） | 新应用**权限没开或开了没重新发版** | 补权限 → 重新发版 |
| 2 | 发消息 `230006 Bot ability is not activated` | 新应用**没启用「机器人」能力** | 应用能力页开启机器人 |
| 3 | 入站 `Unauthorized user: ou_xxx` | 授权名单写的是**旧 app 的 open_id**（per-app 失效） | 换成**该应用下**本人的 open_id（§5.3 取值法） |
| 4 | 群里 `@` **只有某一个** bot 不回，其余都回 | 该 profile 的**适配器实际没在跑**：改过其 `.env` 后 `run_profile_reconcile.py` 重建结果为 `0 adapter(s) connected`，而 `gateway_state.json` 仍显示 `connected`（**过期值**） | **重启网关**（`hermes gateway stop` → 计划任务启动），见 §11.8 |

> 坑 3 最隐蔽 —— 前五项（token/连接/发信/事件）全绿，唯一断点在授权名单（与 §4.2 同源，换应用必复现一次）。
> 坑 4 也隐蔽 —— 配置六项全绿、控制台四样都对，却单路不回；**根因在进程里，不在控制台**（§11.8）。

### 11.4 纠正：`.env` 到底哪些改动要重启

| 改的键 | 是否需重启 | 机制 |
|---|---|---|
| `FEISHU_APP_ID` / `FEISHU_APP_SECRET` | **需要重启网关** | 适配器在**启动时**用凭据建长连接 |
| `FEISHU_ALLOWED_USERS`（仅授权名单） | **不需要，即时生效** | 授权门 **per-turn** 读（`build_profile_secret_scope` + `.env` **mtime 缓存失效**） |
| `FEISHU_HOME_CHANNEL` / 群策略 | 不需要（下条消息生效） | 读取点同上 |

> §5.1 步骤 7 原写「改 `.env` 不热生效」——**修订**：只有**凭据**要重启，**授权名单是热的**。
> ⚠️ 但热重扫**不保证重建成功**：多路复用网关对 `.env` 变更会打 `Re-scanned profile 'x' after config/.env change (N adapter(s) connected)`，**`N=0` 表示该 profile 的适配器没起来**（坑 4）——此时"授权名单热生效"也救不了，必须重启。**改过凭据或迁移后收尾，一律重启一次再验**。

### 11.5 施工单补充：新应用要配「四样」（原 §5.2 三样 + 机器人能力）

> 原 §5.2 写的「权限 / 事件 / 发版」三样，**漏了「启用机器人能力」**（坑 2）。合一版：

1. **权限 5 条**（`im:message` / `im:message:send_as_bot` / `im:message.group_at_msg:readonly` / `im:message.p2p_msg:readonly` / `im:chat:readonly`）
2. **事件** `im.message.receive_v1`（订阅方式=长连接）
3. **机器人能力** —— 开启「机器人」
4. **版本发布** —— 以上任一改动都要**重新发版**才生效

### 11.6 探针脚本（可复用）

```powershell
# 逐个 profile：token + 在群 + 发信 + 群里真实 open_id（secret 不打印）
$h="C:\Users\ldl\AppData\Local\hermes"; $G="oc_6c93e87bc97d9de2bf7e6621545d4c2b"
foreach($p in @('','co','dev','pm','qa','cso')){
  $f = if($p){ "$h\profiles\$p\.env" } else { "$h\.env" }
  $L=Get-Content $f
  $id=(($L|?{$_ -match '^\s*FEISHU_APP_ID='}|Select -Last 1) -replace '^\s*FEISHU_APP_ID=','').Trim()
  $se=(($L|?{$_ -match '^\s*FEISHU_APP_SECRET='}|Select -Last 1) -replace '^\s*FEISHU_APP_SECRET=','').Trim()
  $t=(irm -Uri 'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal' -Method Post -ContentType json -Body (@{app_id=$id;app_secret=$se}|ConvertTo-Json -c) -TimeoutSec 15)
  $m=irm -Uri "https://open.feishu.cn/open-apis/im/v1/chats/$G/members?member_id_type=open_id" -Headers @{Authorization="Bearer $($t.tenant_access_token)"} -TimeoutSec 15
  "$p token=$($t.code) members=$(($m.data.items.member_id) -join ',')"
}
```

### 11.7 群内 @ 入站实测（**六路全通**，本机日志逐条）

| 机器人 | `Inbound` 收到 | `response ready` | 回群 | 结论 |
|---|---|---|---|---|
| CEO | 06:52:21 | 06:52:34 | 33 字 | ✅ |
| co | 06:52:21 | 06:52:41 | 81 字 | ✅ |
| dev | 06:52:21 | 06:52:36 | 140 字 | ✅ |
| pm | 06:52:21 | 06:52:30 | 41 字 | ✅ |
| qa | 07:28:45 | 07:29:07 | 369 字 | ✅（**重启网关后**） |
| cso | 06:52:21 | 06:52:37 | 153 字 | ✅ |

> 判定依据：`Inbound` → `response ready` → `Sending response` 三段齐，且**无** `Send failed` / `Unauthorized user`。
> `cso` 06:00 曾 `Send failed [99991672]`，补齐 `im:message:send_as_bot` 后 06:52 起正常。

### 11.8 收尾复盘：两个"看起来像控制台问题，其实在本机"的陷阱（v1.3）

| # | 陷阱 | 表现 | 正确做法 |
|---|---|---|---|
| A | **二级 profile 的日志不在根目录** | 只翻 `logs/gateway.log` 会得出「**只有 CEO 收到入站**」的错误结论 | `co`/`dev`/`pm`/`qa`/`cso` 各写**自己的** `profiles/<名>/logs/gateway.log`；根日志**只记 default**。查谁收没收到，**看各自的日志** |
| B | **`gateway_state.json` 的 `connected` 会是过期值** | state 显示 `connected`，实际适配器没在跑，群里 @ 无人应答 | 判定"在没在跑"看**当日该 profile 日志有没有 `Connected in websocket mode` / `Inbound`**；并搜根日志的 `Re-scanned profile 'x' ... (0 adapter(s) connected)` |

**由此定下的收尾动作**：凡**改过 `FEISHU_APP_ID/SECRET`**（或迁移后未重启过），**收尾前重启一次网关**，再逐 profile 确认 fresh `Connected`；随后群里逐 bot `@` 一次收尾。

### 11.9 未闭环 / 待人工

| 项 | 说明 |
|---|---|
| ✅ **群内 @ 入站实测** | 六路全部实测通过（§11.7），v1.3 关闭 |
| 🟡 §8.3 本机时间服务 | 仍 `Stopped` |
| 🟡 §8.4 `DEEPSEEK_API_KEY` 告警 | 未处理（实测不影响回复） |

> **纪律**：用户本人做的平台侧修复，按「待核实的说法」处理 —— 配置项用探针逐条核（§11.2），**群内 `@` 的入站腿只能本人 @**（探针替代不了），逐 bot 单独标出，不混进"已通"；本节结论均出自本机日志。
