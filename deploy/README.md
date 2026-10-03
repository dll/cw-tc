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

## 三、GitHub Secrets（仓库 Settings → Secrets and variables → Actions）

| 名称 | 内容 | 必填 |
|---|---|---|
| `DEPLOY_HOST` | 腾讯云主机 IP/域名 | 是 |
| `DEPLOY_USER` | SSH 用户名（如 ubuntu） | 是 |
| `DEPLOY_SSH_KEY` | SSH 私钥（对应公钥须在服务器 `authorized_keys`） | 是 |
| `DEPLOY_PORT` | SSH 端口，默认 22 | 否 |
| `DEPLOY_PATH` | 部署根目录，默认 `/opt/cw-tc` | 否 |

## 四、服务器一次性准备

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
```

## 五、安全组（最小放行）

- **放行**：`80`、`443`（对外），`22`（SSH，建议限来源 IP）。
- **禁止对公网放行**：`8071`（应用）、`5432`（PG）、`6379`（Redis）。

## 六、发布流程（自动）

```
push main → 构建 → 测试(含判分器 --self-test) → 同步内容(--check)
        → 上传 releases/<时间>-<commit>/ → 迁移 DB → 重启 → 健康检查(≤90s)
        → 成功：切 current、清理旧版本(留5)  失败：自动回滚、标红
```

## 七、上线前检查清单（**门禁，逐条打勾**）

- [ ] 判分器 `--self-test` 7/7 全过
- [ ] 证书编号 1000 并发取号无重复
- [ ] 全站无明文身份证号（代码 + 日志 + 响应体扫描）
- [ ] 验真页不返回完整身份证号
- [ ] HTTPS 有效、HSTS 开启、HTTP 301
- [ ] 数据库/Redis 不对公网开放
- [ ] 备份已配置并**验证过一次恢复**
- [ ] 隐私政策与用户协议已上线
- [ ] 管理后台默认口令已改、二次验证已开

> ⚠️ 当前处于 **M0**，本目录为**骨架与约定**；发布脚本在 M4 前落地。
