#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SNIPPET_SRC="$ROOT/deploy/watcher-proxy-params.conf"
SNIPPET_DST="${SNIPPET_DST:-/etc/nginx/snippets/watcher-proxy-params.conf}"
PROXY_CONF="${PROXY_CONF:-/etc/nginx/sites-available/proxy}"

install -m 644 "$SNIPPET_SRC" "$SNIPPET_DST"

python3 - "$PROXY_CONF" << 'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
if not path.exists():
    print(f"{path} not found; skip nginx patch", file=sys.stderr)
    raise SystemExit(0)
text = path.read_text()
block = """    # BEGIN Watcher
    location = /Watcher {
        return 301 /Watcher/;
    }
    location /Watcher/ {
        proxy_pass http://127.0.0.1:8020/;
        include /etc/nginx/snippets/watcher-proxy-params.conf;
    }
    # END Watcher
"""
pattern = r"    # BEGIN Watcher\n.*?    # END Watcher\n"
if re.search(pattern, text, flags=re.S):
    text, n = re.subn(pattern, block, text, count=1, flags=re.S)
    print("updated Watcher nginx locations" if n else "Watcher nginx locations unchanged")
else:
    for anchor in ("    # END GrowTrack\n", "    # END homepage-tools\n"):
        if anchor in text:
            text = text.replace(anchor, anchor + "\n" + block, 1)
            print("patched Watcher locations")
            break
    else:
        print("no nginx anchor found; refusing to patch", file=sys.stderr)
        raise SystemExit(2)
path.write_text(text)
print(f"wrote {path}")
PY

nginx -t
systemctl reload nginx
echo "nginx reloaded; Watcher is at /Watcher/"
