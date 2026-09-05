#!/usr/bin/env bash
# ==============================================================================
# sync_upstream.sh - Sync upstream changes from masterking32/MasterDnsVPN into
# this downstream fork's main branch.
#
# Usage:
#   bash ./scripts/sync_upstream.sh [ref]     # default ref: upstream/main
# ==============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

REMOTE="upstream"
UPSTREAM_URL="https://github.com/masterking32/MasterDnsVPN.git"
REF="${1:-${REMOTE}/main}"
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "ERROR: Not inside a git work tree." >&2
  exit 1
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "ERROR: Remote '$REMOTE' is missing. One-time setup:" >&2
  echo "  git remote add $REMOTE $UPSTREAM_URL" >&2
  exit 1
fi

# Ensure git rerere is enabled
if [ "$(git config --get rerere.enabled || true)" != "true" ]; then
  echo "Enabling git rerere (Reuse Recorded Resolution)..."
  git config rerere.enabled true
  git config rerere.autoupdate true
fi

# Verify clean working tree
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "ERROR: Working tree has uncommitted changes." >&2
  echo "Please commit or stash changes before syncing." >&2
  exit 1
fi

echo "Fetching $REMOTE..."
git fetch "$REMOTE" --no-tags

if ! git rev-parse --verify --quiet "$REF" >/dev/null; then
  echo "ERROR: Ref '$REF' not found on remote '$REMOTE'." >&2
  exit 1
fi

UPSTREAM_HASH="$(git rev-parse --short "$REF")"

# Check if main is already up to date with upstream ref
if git merge-base --is-ancestor "$REF" main; then
  echo "Fork branch 'main' is already up to date with $REF ($UPSTREAM_HASH)."
  exit 0
fi

echo "Switching to 'main'..."
git checkout main

echo ""
echo "Merging $REF ($UPSTREAM_HASH) into main..."
if git merge --no-ff "$REF" -m "chore(upstream): sync upstream main ($UPSTREAM_HASH)"; then
  echo "Successfully merged $REF."
else
  echo ""
  echo "======================================================================" >&2
  echo "CONFLICT: Merge conflicts detected!" >&2
  echo "Resolve the conflicts, run tests, and commit with:" >&2
  echo "  git commit" >&2
  echo "(git rerere will record your resolutions for future merges)" >&2
  echo "======================================================================" >&2
  exit 1
fi

echo ""
echo "Running Go tests..."
go test ./...

echo ""
echo "======================================================================"
echo "Upstream sync completed successfully!"
echo "Next steps:"
echo "  1. Push updated main to your fork:"
echo "     git push origin main"
echo ""
echo "  2. Update your Android client (DNStunnel-mobile):"
echo "     cd ../DNStunnel-mobile"
echo "     & \"\$env:LOCALAPPDATA\\Programs\\Git\\bin\\bash.exe\" ./scripts/sync_core.sh"
echo "======================================================================"
