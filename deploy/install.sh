#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_SRC="${BIN_SRC:-$ROOT/bin/watcher}"
BIN_DST="${BIN_DST:-/usr/local/bin/watcher}"
CONF_DST="${CONF_DST:-/etc/watcher/config.toml}"
DATA_DIR="${DATA_DIR:-/var/lib/watcher}"
UNIT_DST="${UNIT_DST:-/etc/systemd/system/watcher.service}"

if [[ ! -x "$BIN_SRC" ]]; then
  echo "missing $BIN_SRC; run make build first" >&2
  exit 1
fi

if ! id -u watcher >/dev/null 2>&1; then
  useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin watcher
fi

install -d -m 755 /etc/watcher
install -d -o watcher -g watcher -m 750 "$DATA_DIR"
if [[ ! -f "$DATA_DIR/watcher.db" && -f "$ROOT/data/watcher.db" ]]; then
  install -o watcher -g watcher -m 640 "$ROOT/data/watcher.db" "$DATA_DIR/watcher.db"
fi
install -m 755 "$BIN_SRC" "$BIN_DST"
install -m 755 "$ROOT/deploy/unban-ip.sh" /usr/local/sbin/watcher-unban
if [[ ! -f "$CONF_DST" ]]; then
  install -m 640 "$ROOT/deploy/watcher.toml" "$CONF_DST"
  chown root:watcher "$CONF_DST"
fi
install -m 644 "$ROOT/deploy/watcher.service" "$UNIT_DST"

for grp in adm systemd-journal; do
  if getent group "$grp" >/dev/null; then
    usermod -aG "$grp" watcher || true
  fi
done

if [[ -d /var/log/nginx ]]; then
  setfacl -m u:watcher:r-x /var/log/nginx 2>/dev/null || true
  setfacl -m u:watcher:r /var/log/nginx/access.log 2>/dev/null || true
  setfacl -m u:watcher:r /var/log/nginx/error.log 2>/dev/null || true
fi

systemctl daemon-reload
systemctl enable watcher.service
systemctl restart watcher.service

if [[ -f /etc/nginx/sites-available/proxy ]] || [[ -n "${PROXY_CONF:-}" ]]; then
  bash "$ROOT/deploy/install-nginx.sh"
fi

CATALOG_DST="${CATALOG_DST:-/root/project/homepage/catalog.d/watcher.yml}"
if [[ -d "$(dirname "$CATALOG_DST")" ]]; then
  install -m 644 "$ROOT/deploy/watcher.yml" "$CATALOG_DST"
  if command -v docker >/dev/null 2>&1; then
    docker compose -f /root/project/homepage/docker-compose.yml restart homepage >/dev/null 2>&1 || true
  fi
  echo "homepage catalog card installed: $CATALOG_DST"
fi

echo "watcher installed; UI at /Watcher/  (change admin password)"
