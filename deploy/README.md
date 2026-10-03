# cw-tc 部署说明（腾讯云）

> 沿用 `cw-vopc` / `ccit` 的自动部署约定：**GitHub Actions → SSH → 时间戳发布目录 → 软链切换 → 健康检查 → 自动回滚**。

## 一、目标

| 项 | 值 |
|---|---|
| 域名 | **cw-tc.ccit-agent.cn** |
| 部署根目录 | `/opt/cw-tc` |
| 应用端口 | `127.0.0.1:8071`（**不对公网开放**） |
| 数据库 | PostgreSQL（仅容器/本机访问） |
| 缓存 | Redis（仅容器/本机访问） |
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
# 1. 依赖
sudo apt update
sudo apt install -y docker.io docker-compose-v2 rsync curl nginx postgresql redis-server

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

# 6. pgvector 扩展（知识库依赖）
sudo -u postgres psql -d cwtc -c 'CREATE EXTENSION IF NOT EXISTS vector;'
```

## 六、安全组（最小放行）

- **放行**：`80`、`443`（对外），`22`（SSH，建议限来源 IP）。
- **禁止对公网放行**：`8071`（应用）、`5432`（PG）、`6379`（Redis）。

## 七、发布流程（自动）

```
push main → 构建 → 测试(含判分器 --self-test) → 同步内容(--check)
        → 上传 releases/<时间>-<commit>/ → 迁移 DB → 重启 → 健康检查(≤90s)
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

> ⚠️ 当前处于 **M0**，本目录为**骨架与约定**；发布脚本在 **M6 前**落地。
