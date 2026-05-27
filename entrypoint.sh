#!/bin/sh
# Copy built-in hook YAML files into HOOKS_DIR if not already present.
# This lets the mounted volume persist custom hooks while still shipping
# the built-in hooks (feishu, dingtalk, etc.) inside the image.
HOOKS_DIR="${HOOKS_DIR:-/hooks}"
DATA_DIR="${DATA_DIR:-/data}"
mkdir -p "$HOOKS_DIR" "$DATA_DIR"
for f in /builtin-hooks/*.yaml; do
  [ -f "$f" ] || continue
  name=$(basename "$f")
  cp "$f" "$HOOKS_DIR/$name"
done

exec /usr/bin/webhook "$@"
