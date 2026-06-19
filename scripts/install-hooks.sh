#!/usr/bin/env bash
# Enable the version-controlled git hooks under scripts/git-hooks/.
#
# Idempotent: safe to re-run.

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git config core.hooksPath scripts/git-hooks
chmod +x scripts/git-hooks/*

echo "Hooks enabled. git will now use scripts/git-hooks/ for hook lookup."
echo ""
echo "To disable later:"
echo "    git config --unset core.hooksPath"
