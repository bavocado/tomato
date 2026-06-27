#!/bin/sh
# Install tomato's git hooks into .git/hooks by symlinking from scripts/git-hooks.
# Run once per clone: scripts/install-hooks.sh
#
# Hooks live under version control in scripts/git-hooks/ so they travel with the
# repo; git does not auto-run committed hooks (for safety), so this one-time
# install step links them into .git/hooks.
set -e

repo_root=$(git rev-parse --show-toplevel)
src_dir="$repo_root/scripts/git-hooks"
dst_dir="$repo_root/.git/hooks"

mkdir -p "$dst_dir"
for hook in "$src_dir"/*; do
	name=$(basename "$hook")
	ln -sf "../../scripts/git-hooks/$name" "$dst_dir/$name"
	chmod +x "$hook"
	echo "installed hook: $name"
done
