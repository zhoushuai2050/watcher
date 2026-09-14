#!/usr/bin/env bash
set -euo pipefail
ip="${1:-}"
if [[ ! "$ip" =~ ^[0-9a-fA-F:.]+$ ]]; then
  echo "usage: $0 <ip>" >&2
  exit 2
fi
nft delete element inet watcher permanent4 "{ $ip }" 2>/dev/null || \
  nft delete element inet watcher permanent6 "{ $ip }" 2>/dev/null || true
if command -v sqlite3 >/dev/null; then
  sqlite3 /var/lib/watcher/watcher.db "DELETE FROM ip_bans WHERE ip='$ip';"
fi
echo "unbanned $ip"
