#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
mkdir -p dist/assets
MODE="${1:-development}"
ESBUILD="$ROOT/tools/esbuild"
if [[ ! -x "$ESBUILD" ]]; then
  echo "missing $ESBUILD; run python3 scripts/vendor.py" >&2
  exit 1
fi
if [[ "$MODE" == "--watch" ]]; then
  exec "$ESBUILD" src/main.tsx \
    --bundle \
    --outfile=dist/assets/main.js \
    --jsx=automatic \
    --format=esm \
    --target=es2022 \
    --sourcemap \
    --define:process.env.NODE_ENV='"development"' \
    --public-path=/Watcher/assets/ \
    --watch=forever
fi
NODE_ENV_VALUE="production"
if [[ "$MODE" == "development" ]]; then
  NODE_ENV_VALUE="development"
fi
args=(
  "$ESBUILD" src/main.tsx
  --bundle
  --outfile=dist/assets/main.js
  --jsx=automatic
  --format=esm
  --target=es2022
  --define:process.env.NODE_ENV="\"$NODE_ENV_VALUE\""
  --public-path=/Watcher/assets/
)
if [[ "$NODE_ENV_VALUE" == "production" ]]; then
  args+=(--minify)
else
  args+=(--sourcemap)
fi
"${args[@]}"
cp "$ROOT/favicon.svg" "$ROOT/dist/favicon.svg"
ASSET_V="$(cat "$ROOT/dist/assets/main.js" "$ROOT/dist/assets/main.css" 2>/dev/null | md5sum | cut -c1-12)"
sed "s/__ASSET_V__/${ASSET_V}/g" "$ROOT/index.html" > "$ROOT/dist/index.html"
echo "bundled web/dist/assets/main.js v=${ASSET_V}"
