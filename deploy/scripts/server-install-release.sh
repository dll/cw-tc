#!/usr/bin/env bash
# 在已审批的腾讯云/CCIT 既有服务器上安装制品；仅由受保护的 CD job 调用。
set -euo pipefail

ARCHIVE=${1:?缺少发布包路径}
ROOT=${DEPLOY_PATH:-/opt/cw-tc}
RELEASE_ID=${RELEASE_ID:?缺少 RELEASE_ID}
RELEASES="$ROOT/releases"
TARGET="$RELEASES/$RELEASE_ID"
PREVIOUS=$(readlink -f "$ROOT/current" 2>/dev/null || true)

[ -f "$ARCHIVE" ] || { echo "发布包不存在" >&2; exit 1; }
mkdir -p "$RELEASES" "$ROOT/shared" "$ROOT/backups"
tar -xzf "$ARCHIVE" -C "$RELEASES"

# 只校验服务器侧环境形状，不打印任何秘密。
DEPLOY_ENV=production bash "$TARGET/deploy/migration-preflight.sh" "$TARGET" "$ROOT/shared/.env"

ln -sfn "$TARGET" "$ROOT/next"
ln -sfn "$ROOT/next" "$ROOT/current"
systemctl restart cw-tc-server

ok=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 3 http://127.0.0.1:8071/api/v1/health >/dev/null; then ok=1; break; fi
  sleep 2
done

if [ "$ok" -ne 1 ]; then
  echo "健康检查失败，执行应用软链回滚" >&2
  if [ -n "$PREVIOUS" ] && [ -d "$PREVIOUS" ]; then
    ln -sfn "$PREVIOUS" "$ROOT/current"
    systemctl restart cw-tc-server || true
  fi
  exit 1
fi
rm -f "$ROOT/next"
printf '生产制品已安装并通过健康检查: %s\n' "$RELEASE_ID"
