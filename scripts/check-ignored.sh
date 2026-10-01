#!/usr/bin/env sh
# Fails if any file in a source location is being ignored by git.
# A too-broad .gitignore rule (e.g. "lib/") can silently drop source files
# from commits; this catches that before it reaches a commit or CI.
cd "$(git rev-parse --show-toplevel)" || exit 1
# Everything except folders that are meant to be ignored (dependencies,
# build output, scratch space, binaries, local tool config, OS files, secrets).
ignored=$(git ls-files --others --ignored --exclude-standard -- . \
  ':!frontend/node_modules' ':!frontend/dist' ':!tmp' ':!.claude' \
  ':!tscm-change-detection' ':!tscm-change-detection.exe' \
  ':(exclude,glob)**/.DS_Store' ':(exclude,glob)**/.env' ':(exclude,glob)**/.env.*')
if [ -n "$ignored" ]; then
  echo "These source files are ignored by .gitignore and would not be committed:" >&2
  echo "$ignored" >&2
  exit 1
fi
echo "ok: no source files are ignored"
