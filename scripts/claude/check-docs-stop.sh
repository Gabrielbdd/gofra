#!/usr/bin/env bash
#
# check-docs-stop.sh — enforce the docs-as-code contract.
#
# Hook mode (default):  checks working-tree diff against HEAD.
# CI mode (--ci):       same check, human-readable stderr output, exits non-zero on violation.
#
# Public-surface paths whose changes require a docs/framework/ update:
#   runtime/**  cmd/gofra/**  proto/**  internal/scaffold/starter/full/**

set -euo pipefail

CI_MODE=false
if [[ "${1:-}" == "--ci" ]]; then
  CI_MODE=true
fi

# Collect changed files (staged + unstaged) relative to HEAD,
# plus new untracked files (so new runtime/ or docs/ files are caught).
changed_files=$(git diff --name-only HEAD 2>/dev/null || true)
staged_files=$(git diff --cached --name-only 2>/dev/null || true)
untracked_files=$(git ls-files --others --exclude-standard 2>/dev/null || true)

# Merge and deduplicate.
all_changed=$(printf '%s\n%s\n%s' "$changed_files" "$staged_files" "$untracked_files" | sort -u | grep -v '^$' || true)

if [[ -z "$all_changed" ]]; then
  # Nothing changed — allow.
  exit 0
fi

# Check for public-surface changes.
public_changed=false
while IFS= read -r file; do
  case "$file" in
    runtime/*|cmd/gofra/*|proto/*|internal/scaffold/starter/full/*)
      public_changed=true
      break
      ;;
  esac
done <<< "$all_changed"

if [[ "$public_changed" == "false" ]]; then
  # Only internal changes — allow.
  exit 0
fi

# Check for docs/framework/ changes.
docs_changed=false
while IFS= read -r file; do
  case "$file" in
    docs/framework/*)
      docs_changed=true
      break
      ;;
  esac
done <<< "$all_changed"

if [[ "$docs_changed" == "true" ]]; then
  # Public surfaces changed AND docs were updated — allow.
  exit 0
fi

# Violation: public surfaces changed but no docs/framework/ update.
msg="Public framework surfaces changed but no docs/framework/ files were updated.

Changed public-surface files:
$(echo "$all_changed" | grep -E '^(runtime/|cmd/gofra/|proto/|internal/scaffold/starter/full/)' || true)

Before finishing, update the appropriate Diataxis docs under docs/framework/:
  - Reference pages: docs/framework/reference/
  - How-to guides:   docs/framework/how-to/
  - Tutorials:       docs/framework/tutorials/
  - Explanations:    docs/framework/explanation/

See .claude/rules/documentation-contract.md for the full policy."

if [[ "$CI_MODE" == "true" ]]; then
  echo "$msg" >&2
  exit 1
else
  # Hook mode: print blocking reason for Claude to read.
  echo "BLOCKED: $msg"
  exit 1
fi
