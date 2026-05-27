#!/bin/sh
HOOKS_DIR="${HOOKS_DIR:-/hooks}"
DATA_DIR="${DATA_DIR:-/data}"
SCRIPTS_DIR="$DATA_DIR/scripts"
mkdir -p "$HOOKS_DIR" "$SCRIPTS_DIR"

# Always overwrite built-in hook YAMLs (system-managed, users edit scripts not YAMLs)
for f in /builtin-hooks/*.yaml; do
  [ -f "$f" ] || continue
  cp "$f" "$HOOKS_DIR/$(basename "$f")"
done

# Copy built-in scripts to data volume ONLY if not already present (preserve user edits)
for f in /notify/*.sh; do
  [ -f "$f" ] || continue
  dest="$SCRIPTS_DIR/$(basename "$f")"
  if [ ! -f "$dest" ]; then
    cp "$f" "$dest"
    chmod +x "$dest"
  fi
done

exec /usr/bin/webhook "$@"
