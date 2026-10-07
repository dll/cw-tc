# cw-tc 部署说明（腾讯云）

> 沿用 `cw-vopc` / `ccit` 的自动部署约定：**GitHub Actions → SSH → 时间戳发布目录 → 软链切换 → 健康检查 → 自动回滚**。

## 一、目标

| 项 | 值 |
|---|---|
| 域名 | **cw-tc.ccit-agent.cn** |
| 部署根目录 | `/opt/cw-tc` |
| 应用端口 | `127.0.0.1:8071`（**不对公网开放**） |
- **生产数据库**：腾讯云 PostgreSQL 15+（沿用 CCIT 既有实例、私网、账号与备份；CI 不连接、不执行生产迁移）。
- **迁移材料**随 release bundle 提供，由审批后的服务器侧流程执行。
| 缓存 | Redis（生产沿用既有 CCIT/腾讯云内网 Redis；开发/测试不启动、不连接） |
| 反代 | nginx（`:80` 跳转 / `:443` TLS 终止） |

## 二、目录约定

```
/opt/cw-tc/
├── releases/
│   └── <时间戳>-<commit>/     每次发布一个目录
├── current -> releases/...     软链：切换即发布
├── shared/
│   ├── .env                    密钥/口令（**不进版本库**）
│   └── data/                   PostgreSQL 数据（跨版本保留）
└── backups/
```

## 三、环境变量与密钥（**照同系列项目一样处理**）

仓库内**只放模板** `deploy/.env.example`（`.gitignore` 已放行 `!deploy/**/*.example`）：

```bash
# 服务器上一次性准备
sudo mkdir -p /opt/cw-tc/shared
sudo cp deploy/.env.example /opt/cw-tc/shared/.env
sudo chmod 600 /opt/cw-tc/shared/.env
sudo chown ubuntu:ubuntu /opt/cw-tc/shared/.env
# 编辑填入真实值（切勿回填到仓库）
```

- 由 systemd `EnvironmentFile=/opt/cw-tc/shared/.env` 注入（见 `systemd/cw-tc-server.service`）；
- 做法与 ccit 的 `/etc/ccit/ccit.env` 一致：**密钥只存服务器，权限 600，不进版本库、不进日志**；
- **涵盖五类凭据**：数据库 / Redis · **强制密钥**（身份证加密 + HMAC + 会话）·
  **短信**（含防刷限额）· **支付**（微信 / 支付宝）· **大模型与 Embedding**；
- **安全默认**：任一类未配置 = 该功能 **fail-closed 不启用**；
  但**强制密钥缺失则服务拒绝启动**（不得退回明文存身份证号）。

### 不可挽回的两把钥匙（**离线备份至少 2 份异地**）

| 钥匙 | 丢了会怎样 | 要求 |
|---|---|---|
| `CWTC_ID_CARD_ENC_KEY`（AES-256-GCM） | **历史身份证号密文永不可解 → 学员证书永远补不出来** | 加密后离线存 **2 处异地**；**不得与数据库备份放同一介质 / 同一云账号**；轮换需 `key_id` 版本化，旧密钥保留 |
| `CWTC_ID_CARD_HMAC_KEY` | **去重失效 + 无法按证件号定位学员** | 同上；**一旦启用视为永久不可轮换** |
| **Android APK 签名密钥库 `.jks`**（§PRD v2 §12.7.2） | **无法再发布可覆盖安装的升级包**，用户必须卸载重装 | 离线保管，**永不入库、不进 CI 明文变量**；口令与密钥库**分开保管**（不同人 / 不同位置） |

- **备份恢复演练不能只验"文件在"**：必须**用备份密钥成功解密一条历史密文**，
  才算验证通过（§PRD v2 §12.5 清单已列）。数据库能从备份恢复，**密钥不能**。

## 三点五、staging / 沙箱环境（**与生产物理隔离**）

支付、短信、AI **都不能在生产上试错**（都是花钱的）。因此必须有 staging：

| 环境 | 域名 | 关键差异 |
|---|---|---|
| 本地 | `127.0.0.1:8071` | 支付 / 短信 / AI **全关**（fail-closed） |
| **staging** | `staging.cw-tc.ccit-agent.cn` | **微信/支付宝沙箱**；**短默认不真发**（走日志或白名单号）；AI 用**独立小额度 key** |
| 生产 | `cw-tc.ccit-agent.cn` | 真实资金与真实短信 |

- **staging 与生产：数据库物理隔离、密钥不共用、支付商户号不混用**；
- staging **必须加访问限制**（Basic Auth 或 IP 白名单），否则会被搜索引擎收录成"第二个官网"。

## 三点六、Android APK 直发（**不上应用商店**）

- 官网提供下载页（版本号 / 体积 / **SHA-256** / 更新说明）；产物 `cw-tc-<versionName>+<versionCode>-arm64-v8a.apk`；
- **包名 `cn.ccit.cwtc` 一经发布永不更改**；
- **签名**：CI 从 GitHub Secrets 取密钥库与口令，构建后**立即销毁临时文件**；**不外传、不入库**；
- **应用内更新**：App 请求 `GET /api/v1/app/latest?platform=android` → 有新版本则提示 →
  下载后**先校验 SHA-256 再安装**（防中间人替换）。

## 四、GitHub Secrets（仓库 Settings → Secrets and variables → Actions）

| 名称 | 内容 | 必填 |
|---|---|---|
| `DEPLOY_HOST` | 腾讯云主机 IP/域名 | 是 |
| `DEPLOY_USER` | SSH 用户名（如 ubuntu） | 是 |
| `DEPLOY_SSH_KEY` | SSH 私钥（对应公钥须在服务器 `authorized_keys`） | 是 |
| `DEPLOY_PORT` | SSH 端口，默认 22 | 否 |
| `DEPLOY_PATH` | 部署根目录，默认 `/opt/cw-tc` | 否 |

## 五、服务器一次性准备

```bash
# 1. 依赖（沿用既有 CCIT/腾讯云服务器；不新建服务器、数据库或本机服务）
sudo apt update
sudo apt install -y rsync curl nginx

# 2. 部署目录
sudo mkdir -p /opt/cw-tc/{releases,shared,backups}
sudo chown -R ubuntu:ubuntu /opt/cw-tc

# 3. nginx 配置
sudo cp deploy/nginx/cw-tc.conf /etc/nginx/conf.d/
sudo nginx -t && sudo systemctl reload nginx

# 4. systemd 服务
sudo cp deploy/systemd/cw-tc-server.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable cw-tc-server

# 5. 环境变量（见第三节）
sudo cp deploy/.env.example /opt/cw-tc/shared/.env && sudo chmod 600 /opt/cw-tc/shared/.env

# 6. pgvector/数据库扩展：由既有腾讯云 PostgreSQL DBA 按变更单执行；本机不执行。
```

## 六、安全组（最小放行）

- **放行**：`80`、`443`（对外），`22`（SSH，建议限来源 IP）。
- **禁止对公网放行**：`8071`（应用）、`5432`（PG）、`6379`（Redis）。

## 七、发布流程（自动）

```
push main → 构建 → 测试 → 生成不可变制品 + 审批材料
        → CO/运维审批 → 服务器侧注入 shared/.env
        → 既有腾讯云 PostgreSQL 先迁移 → 重启 → 健康检查
        → 成功：切 current、清理旧版本(留5)  失败：自动回滚、标红
```

## 八、上线前检查清单（**门禁，逐条打勾**）

**通用**

- [ ] 判分器 `--self-test` 7/7 全过
- [ ] 证书编号 1000 并发取号无重复
- [ ] 全站无明文身份证号（代码 + 日志 + 响应体扫描）
- [ ] 验真页不返回完整身份证号
- [ ] HTTPS 有效、HSTS 开启、HTTP 301
- [ ] 数据库 / Redis 不对公网开放
- [ ] 备份已配置并**验证过一次恢复**
- [ ] 隐私政策与用户协议已上线（**含付费与退款政策**）
- [ ] 管理后台默认口令已改、二次验证已开
- [ ] `shared/.env` 权限为 600、属主正确，且**未被提交到仓库**

**v2 新增**

- [ ] **登录限频与锁定**脚本验证生效（100 次无效尝试）
- [ ] **短信发码防刷**脚本验证生效（图形验证码前置 / 日上限 / 冷却 / IP 限频 / 费用告警）
- [ ] **换绑手机号**双向验证演练一次（含原号通知）
- [ ] **支付**：商户号资质齐备；回调验签通过；**重复回调幂等**验证
- [ ] **对账**：跑通一次每日对账，**差异为 0**
- [ ] **退款**演练一次：权益撤销 + **证书连带撤销** + 验真页显示「已撤销」
- [ ] **退款后重新购买**路径演练一次（重新发证，旧编号不复用）
- [ ] **AI**：回答**带引用比例 100%**（抽样 50 条）；违规样本被内容审核拦截
- [ ] **知识库**：不含证书 / `_ai_draft/`；重建可重入
- [ ] **广播**：触达人数与实际一致，且不重复发送
- [ ] **下单幂等**：并发连点只产生一笔待支付订单（v2.3）
- [ ] **密钥备份可解密**：用备份的加密密钥**成功解密一条历史密文**（不是只验证文件在）
- [ ] **`sync` 不动 `platform` 课程**：跑一次同步，教师建的课**未被清空或改写**（最易写错）
- [ ] **证书有效期**：「有效期至」= 签发 + 3 年；已过期证书在验真页显示「已过期」
- [ ] **实名披露一致**：注册页 / 证书 / 验真页三处措辞一致，**未出现"已核验"字样**
- [ ] **APK**：官网下载页可用；SHA-256 与实际包一致；应用内更新提示可触发
- [ ] **staff 开通**：种子管理员可登录且**强制改密**；管理员可开通教师账号
- [ ] **staging**：与生产物理隔离，且有访问限制

> 生产安装已按 CO 指令恢复：CO workflow 会 SSH 安装、执行服务端迁移、重启并健康检查，失败自动回滚。
