#!/usr/bin/env bash
# cw-tc 服务器端构建+部署：后端(Go) + 管理端(Vue) + 学生端(Flutter Web) + 门户静态站
set -euo pipefail
export PATH=/usr/local/go/bin:$PATH GOPROXY=https://goproxy.cn,direct
export FLUTTER_STORAGE_BASE_URL=https://storage.flutter-io.cn PUB_HOSTED_URL=https://pub.flutter-io.cn
REPO=/opt/cw-tc-build/repo; ROOT=/opt/cw-tc
cd "$REPO"; git fetch --quiet origin; git checkout --quiet main; git reset --hard --quiet origin/main
SHA=$(git rev-parse --short=12 HEAD)

# --- 后端 ---
( cd packages/server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags "-s -w -X github.com/cw-tc/server/internal/api.Version=$SHA" -o /tmp/cw-tc-server ./cmd/server )
RID="$(date -u +%Y%m%dT%H%M%SZ)-$SHA"; RD="$ROOT/releases/$RID"
mkdir -p "$RD/bin" "$RD/migrations"
install -m 0755 /tmp/cw-tc-server "$RD/bin/cw-tc-server"
cp -R "$REPO/packages/server/migrations/postgres" "$RD/migrations/"
printf 'release_id=%s\ncommit=%s\nenvironment=production\ndatabase_driver=postgres\n' "$RID" "$SHA" > "$RD/RELEASE-MANIFEST.txt"
ln -sfn "$RD" "$ROOT/current"
sudo systemctl restart cw-tc-server

# --- 管理端 SPA ---
if command -v npm >/dev/null 2>&1 && [ -d "$REPO/packages/admin" ]; then
  ( cd "$REPO/packages/admin" && npm install --no-audit --no-fund --silent && npx vite build --base=/admin/ )
  mkdir -p /var/www/cw-tc-admin && rm -rf /var/www/cw-tc-admin/*
  tar -C "$REPO/packages/admin/dist" -cf - . | tar -C /var/www/cw-tc-admin -xf -
  echo "管理端已更新"
fi

# --- 学生端 Web（服务器有 flutter 时构建）---
if [ -x /opt/flutter/bin/flutter ]; then
  export PATH=/opt/flutter/bin:$PATH
  ( cd "$REPO/packages/app" && flutter pub get >/dev/null && flutter build web --release --base-href=/student/ )
  mkdir -p /var/www/cw-tc-student && rm -rf /var/www/cw-tc-student/*
  tar -C "$REPO/packages/app/build/web" -cf - . | tar -C /var/www/cw-tc-student -xf -
  echo "学生端已更新"
fi

# --- 门户静态站 ---
if [ -d "$REPO/deploy/static" ]; then
  sudo mkdir -p /var/www/cw-tc-portal; sudo rm -rf /var/www/cw-tc-portal/*
  sudo cp -r "$REPO/deploy/static/." /var/www/cw-tc-portal/
  sudo chown -R ubuntu:ubuntu /var/www/cw-tc-portal
  echo "门户已更新"
fi

# --- 健康检查 ---
ok=0; for _ in $(seq 1 20); do curl -fsS --max-time 3 http://127.0.0.1:8071/api/v1/health >/dev/null && ok=1 && break; sleep 2; done
[ "$ok" = 1 ] && echo "部署成功 RID=$RID" || { echo "健康检查失败 $RID" >&2; sudo journalctl -u cw-tc-server -n 20 --no-pager >&2; exit 1; }
