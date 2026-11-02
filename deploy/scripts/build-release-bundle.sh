#!/usr/bin/env bash
# 构建不可变发布制品与部署审批材料；本脚本不连接数据库、缓存或生产服务器。
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
SERVER="$ROOT/packages/server"
OUT_DIR="${1:-$ROOT/.release-output}"
case "$OUT_DIR" in
  /*|[A-Za-z]:/*) ;;
  *) OUT_DIR="$ROOT/$OUT_DIR" ;;
esac
COMMIT="${GITHUB_SHA:-$(git rev-parse HEAD)}"
SHORT_COMMIT=$(printf '%s' "$COMMIT" | cut -c1-12)
TIMESTAMP="${SOURCE_DATE_EPOCH:-$(date -u +%Y%m%dT%H%M%SZ)}"
RELEASE_ID="${TIMESTAMP}-${SHORT_COMMIT}"
RELEASE_DIR="$OUT_DIR/$RELEASE_ID"

rm -rf "$OUT_DIR"
mkdir -p "$RELEASE_DIR/bin" "$RELEASE_DIR/deploy" "$RELEASE_DIR/migrations" "$RELEASE_DIR/web"

(
  cd "$SERVER"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X github.com/cw-tc/server/internal/api.Version=$SHORT_COMMIT" \
    -o "bin/cw-tc-server" ./cmd/server
)
cp "$SERVER/bin/cw-tc-server" "$RELEASE_DIR/bin/cw-tc-server"
rm -f "$SERVER/bin/cw-tc-server"
cp "$ROOT/deploy/.env.example" "$RELEASE_DIR/deploy/.env.example"
cp -R "$SERVER/migrations/postgres" "$RELEASE_DIR/migrations/"
cp "$ROOT/deploy/systemd/cw-tc-server.service" "$RELEASE_DIR/deploy/"
cp "$ROOT/deploy/nginx/cw-tc.conf" "$RELEASE_DIR/deploy/"
cp "$ROOT/deploy/scripts/migration-preflight.sh" "$RELEASE_DIR/deploy/"
cp "$ROOT/deploy/scripts/server-install-release.sh" "$RELEASE_DIR/deploy/"

# 管理端（Vue 3）：构建产物直接进发布包，避免“只发后端、页面还是旧的”。
if ! command -v npm >/dev/null 2>&1; then
  echo "缺少 npm，无法构建管理端；发布包必须包含前端资产" >&2
  exit 1
fi
(cd "$ROOT/packages/admin" && npm ci --no-audit --no-fund && MSYS_NO_PATHCONV=1 npm run build -- --base=/admin/)
mkdir -p "$RELEASE_DIR/web/admin"
cp -R "$ROOT/packages/admin/dist/." "$RELEASE_DIR/web/admin/"

# 学员端（Flutter Web）：必须固定 base-href=/student/。
if ! command -v flutter >/dev/null 2>&1; then
  echo "缺少 flutter，无法构建学员端；发布包必须包含前端资产" >&2
  exit 1
fi
(cd "$ROOT/packages/app" && flutter pub get && MSYS_NO_PATHCONV=1 flutter build web --release --no-pub --base-href=/student/)
mkdir -p "$RELEASE_DIR/web/student"
cp -R "$ROOT/packages/app/build/web/." "$RELEASE_DIR/web/student/"

# 门户静态页
cp -R "$ROOT/deploy/static" "$RELEASE_DIR/web/portal"

cat > "$RELEASE_DIR/RELEASE-MANIFEST.txt" <<EOF
release_id=$RELEASE_ID
commit=$COMMIT
environment=production
database_driver=postgres
database_provider=existing-ccit-tencent-cloud-postgresql
secrets_source=/opt/cw-tc/shared/.env
production_touch=false
approval_required=true
files:
- bin/cw-tc-server
- migrations/postgres/*.sql
- web/admin/*
- web/student/*
- web/portal/*
- deploy/.env.example
- deploy/cw-tc-server.service
- deploy/cw-tc.conf
- deploy/migration-preflight.sh
- deploy/server-install-release.sh
- deployment-approval-request.json
EOF

(cd "$RELEASE_DIR" && { sha256sum bin/cw-tc-server; find web -type f -print0 | sort -z | xargs -0 sha256sum; } > SHA256SUMS)
cat > "$RELEASE_DIR/deployment-approval-request.json" <<EOF
{
  "release_id": "$RELEASE_ID",
  "commit": "$COMMIT",
  "environment": "production",
  "database_driver": "postgres",
  "database_provider": "腾讯云 PostgreSQL（沿用 CCIT 既有环境参数，由服务器端 shared/.env 注入）",
  "migration_source": "migrations/postgres",
  "frontend_assets": ["web/admin", "web/student", "web/portal"],
  "deployment_mode": "approval_required",
  "production_touch": false,
  "rollback": "切回上一 releases/<timestamp>-<commit> 软链；数据库 DDL 仅按既有备份恢复流程处理",
  "required_approvals": ["CO", "运维负责人"],
  "secrets_source": "/opt/cw-tc/shared/.env",
  "generated_at_utc": "$TIMESTAMP"
}
EOF

(cd "$OUT_DIR" && tar -czf "cw-tc-${RELEASE_ID}.tar.gz" "$RELEASE_ID")
(cd "$OUT_DIR" && sha256sum "cw-tc-${RELEASE_ID}.tar.gz" > "cw-tc-${RELEASE_ID}.tar.gz.sha256")
printf '%s\n' "$OUT_DIR/cw-tc-${RELEASE_ID}.tar.gz"
printf '%s\n' "$OUT_DIR/$RELEASE_ID/deployment-approval-request.json"
