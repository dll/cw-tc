# Hermes + 飞书 + 测试回复 v1

> **主题**：豆包账号租户下 `CCIT-OPC公司群` 机器人失效与修复复盘 —— 从 Hermes 安装、飞书建应用与建群、配置成功、开发 cw-tc，到群回复失效与三日修复
> **版本**：v1.0　**日期**：2026-10-15　**状态**：已修复并实测通过
> **范围**：Hermes 网关的飞书通道（默认 profile 及 `co`/`dev`/`pm`/`qa` 分身）
> **不涉及**：cw-tc 业务代码本身

---

## 0. 一页摘要

| 项 | 内容 |
|---|---|
| **现象** | 飞书群 `@机器人` 无回复；客户端表现为「图标出现 → 消失 → 没回复」 |
| **直接原因** | 取 `tenant_access_token` 返回 `21001 Access denied by access control policy` |
| **根因一** | 网关 5 个自建应用**建在个人版身份「豆包账号」（非企业租户）下**，飞书侧对该类身份的应用执行访问控制策略 |
| **根因二** | 迁到企业租户后，`FEISHU_ALLOWED_USERS` 为空 ⇒ 授权 allowlist 语义下**谁都不许** ⇒ 消息收得到但被丢弃 |
| **修复** | 在已认证企业「畅网」下新建应用 **Hermes-CEO** ＋ 新建群 **Hermes-CCIT** ＋ 补齐权限/事件/重新发版 ＋ 填授权名单 |
| **验证** | 群里 `@Hermes-CEO` → 收到 → 生成 → 回复成功（10-15 00:44） |
| **未闭环** | `co`/`dev`/`pm`/`qa` 四个分身未迁移；旧群 `CCIT-OPC公司群` 因跨租户不可迁移 |

> **一句话**：这不是本机故障，也不是代码故障 —— 是**「应用归属的租户身份」被飞书策略收紧**，叠加迁租户后暴露的**授权名单为空**。两层的表现完全不同（前者报 21001，后者日志写 `Unauthorized user`），必须分开诊断。

---

## 1. 背景

**Hermes** 是运行在本机的 AI 代理网关，把飞书群当作「指令通道」：群里 `@` 某个机器人（CEO / CO / DEV / PM / QA），对应的 agent 就会接活、调模型、把结果回帖到群里。

**cw-tc（第三课堂）** 这个项目的开发就是走这条通道的：在 `Hermes-CCIT` / `CCIT-OPC公司群` 里下指令，agent 在本地 `E:\2026-2027\2026-2027-1\MyProjects\cw-tc` 改代码、跑门禁、提交推送。

所以「群里不回话」= **开发通道断了**，不是一个小功能问题。

本机环境（排查期间实测）：

| 项 | 值 |
|---|---|
| Hermes home | `C:\Users\ldl\AppData\Local\hermes` |
| 网关启动方式 | Windows 计划任务 `Hermes_Gateway`（**不是** `hermes gateway start`，见 §7.2） |
| 网关卡 | `bin/hermes.exe gateway` |
| profile 数 | 5（`default` + `co`/`dev`/`pm`/`qa`） |
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
          |  profiles: default  co  dev  pm  qa              |
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
| 7 | 写 `.env`：`FEISHU_HOME_CHANNEL` = 新群 `oc_` id；**重启网关** | 改 `.env` 不热生效 |

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

### 8.1 🔴 `co` / `dev` / `pm` / `qa` 四个分身未迁移

仍是个人版租户下的应用（`cli_aa4916be…` / `cli_aa491fc9…` / `cli_aa4922df…` / `cli_aa491bb2…`），**长连接能上、消息发不出**（同 21001）。

**动作**：按 §5.1 各建一个企业租户应用 + 补三样 + **各填各自的新 open_id**（注意是 per-app 的，不能复制 CEO 那个）。

| profile | 当前 App ID | 状态 |
|---|---|---|
| `default` | `cli_aa451b7b4bb8dc0f` | ✅ 已迁（畅网） |
| `co` | `cli_aa4916beb478dbed` | ⏳ 待迁 |
| `dev` | `cli_aa491fc95338dcc6` | ⏳ 待迁 |
| `pm` | `cli_aa4922dfba38dbe3` | ⏳ 待迁 |
| `qa` | `cli_aa491bb29cf89bd1` | ⏳ 待迁 |

### 8.2 🔴 旧群 `CCIT-OPC公司群` 不可迁移

`oc_76dc985873e026f38d4c25914c3daddb` 在**豆包账号租户**下；企业租户的应用**进不去**（自建应用只属于一个租户）。**跨租户搬群在控制台里没有入口。**

⇒ 结论：**保群与换租户二选一，不能兼得**。现方案是换群（`Hermes-CCIT`）。

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
| 个人版身份 | 豆包账号（网关 5 个旧应用的归属） |
| 新应用 | `Hermes-CEO` / `cli_aa451b7b4bb8dc0f` / bot open_id `ou_02dee9b1c4e0c9cae83c9814206705b2` |
| 新群 | `Hermes-CCIT` / `oc_6c93e87bc97d9de2bf7e6621545d4c2b` |
| 旧群 | `CCIT-OPC公司群` / `oc_76dc985873e026f38d4c25914c3daddb` |
| 用户 open_id（新应用下） | `ou_85c77a4a70a94aefdf68b027fee7750d` |

> ⚠️ **App Secret / tenant_access_token 一律不在此文档中出现。**

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
