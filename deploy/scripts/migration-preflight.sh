#!/usr/bin/env bash
# 仅检查 release bundle 与服务器侧配置的形状；不连接数据库、缓存或生产服务器。
set -euo pipefail

BUNDLE_DIR=${1:?用法: bash migration-preflight.sh <release-dir> [server-env-file]}
SERVER_ENV=${2:-}

fail() { printf '✗ %s\n' "$1" >&2; exit 1; }
[ -d "$BUNDLE_DIR" ] || fail "制品目录不存在: $BUNDLE_DIR"
[ -x "$BUNDLE_DIR/bin/cw-tc-server" ] || fail "缺少可执行服务二进制"
[ -d "$BUNDLE_DIR/migrations/postgres" ] || fail "缺少 postgres 迁移目录"
[ -f "$BUNDLE_DIR/deployment-approval-request.json" ] || fail "缺少审批请求材料"
[ -f "$BUNDLE_DIR/SHA256SUMS" ] || fail "缺少二进制校验文件"

(cd "$BUNDLE_DIR" && sha256sum -c SHA256SUMS >/dev/null) || fail "制品校验失败"

count=$(find "$BUNDLE_DIR/migrations/postgres" -maxdepth 1 -type f -name '*.sql' | wc -l | tr -d ' ')
[ "$count" -gt 0 ] || fail "postgres 迁移目录为空"

# 只读取键名与非敏感值形状；不输出密码、DSN 或密钥。
if [ -n "$SERVER_ENV" ]; then
  [ -f "$SERVER_ENV" ] || fail "服务器环境文件不存在: $SERVER_ENV"
  driver=$(awk -F= '$1=="CWTC_DB_DRIVER" {print $2}' "$SERVER_ENV" | tail -1)
  env=$(awk -F= '$1=="DEPLOY_ENV" {print $2}' "$SERVER_ENV" | tail -1)
  [ "$env" = "production" ] || fail "服务器环境 DEPLOY_ENV 必须为 production"
  [ "$driver" = "postgres" ] || fail "服务器环境 CWTC_DB_DRIVER 必须为 postgres"
  for key in CWTC_DB_HOST CWTC_DB_PORT CWTC_DB_USER CWTC_DB_NAME CWTC_ID_CARD_ENC_KEY CWTC_ID_CARD_HMAC_KEY CWTC_SESSION_SECRET; do
    grep -q "^${key}=" "$SERVER_ENV" || fail "服务器环境缺少键: $key"
  done
fi

printf '✓ 制品、迁移材料与生产配置形状检查通过（未连接外部服务）\n'
printf '  postgres migrations: %s files\n' "$count"
printf '  production migration execution: approval required / server-side only\n'
