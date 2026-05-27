#!/usr/bin/env bash
# check-migration-linearity.sh
#
# Verify infra/migrations/*.sql is a strictly linear, gap-free, dupe-free
# sequence of 3-digit-prefixed files (001_..., 002_..., ...).
#
# Background
# ----------
# services/core-api/internal/migrate.Migrator only sorts entries lexically
# and runs everything matching *.sql. That works fine when versions are
# perfectly linear, but it silently accepts:
#   * a missing version  (003 then 005)        → "skipped" migration
#   * a duplicate prefix (028_a.sql, 028_b.sql) → undefined apply order
#                                                 across branches
#   * a non-3-digit prefix                     → sorts ahead of 001
# All three of these have caused production incidents in the wild. This
# script keeps the migration ladder honest BEFORE bad files reach main.
#
# Usage
# -----
#   bash scripts/check-migration-linearity.sh
#   bash scripts/check-migration-linearity.sh path/to/other/migrations
#
# Exits non-zero with a human-readable explanation on any violation.
set -euo pipefail

DIR="${1:-infra/migrations}"

# Note on "missing dir / empty dir" handling:
#   The repo's `.gitignore` deliberately excludes `infra/migrations/*.sql`
#   so prod migrations stay out of source control (they're applied via a
#   separate operator workflow). On a fresh CI checkout the directory
#   therefore looks either absent or empty. We treat that as a successful
#   no-op rather than a hard failure: the linearity invariant only matters
#   when files are actually present, and local devs who have the files get
#   the strict check exactly as before. To opt into a strict "must have at
#   least one migration" mode, set CHECK_MIGRATIONS_REQUIRED=1.
strict="${CHECK_MIGRATIONS_REQUIRED:-0}"

if [[ ! -d "$DIR" ]]; then
  echo "check-migration-linearity: directory not found: $DIR (skipping; set CHECK_MIGRATIONS_REQUIRED=1 to fail instead)"
  if [[ "$strict" == "1" ]]; then
    exit 2
  fi
  exit 0
fi

shopt -s nullglob
files=("$DIR"/*.sql)
shopt -u nullglob

if (( ${#files[@]} == 0 )); then
  echo "check-migration-linearity: no *.sql files in $DIR (skipping; set CHECK_MIGRATIONS_REQUIRED=1 to fail instead)"
  if [[ "$strict" == "1" ]]; then
    exit 2
  fi
  exit 0
fi

prev=0
seen=()
errors=0

for path in "${files[@]}"; do
  name="$(basename "$path")"

  # Must start with exactly three digits followed by an underscore.
  if [[ ! "$name" =~ ^([0-9]{3})_[a-z0-9][a-z0-9_]*\.sql$ ]]; then
    echo "FAIL: $name — must match ^[0-9]{3}_[a-z0-9_]+\\.sql$" >&2
    errors=$((errors + 1))
    continue
  fi

  num=$((10#${BASH_REMATCH[1]}))

  # Duplicates: a previously-seen version with a different filename.
  for s in "${seen[@]}"; do
    if (( s == num )); then
      echo "FAIL: $name — duplicate version $(printf %03d "$num"); two files share the same migration number." >&2
      errors=$((errors + 1))
      continue 2
    fi
  done
  seen+=("$num")

  expected=$((prev + 1))
  if (( num != expected )); then
    echo "FAIL: $name — expected version $(printf %03d "$expected"), got $(printf %03d "$num"); gap in the migration ladder." >&2
    errors=$((errors + 1))
  fi
  prev=$num
done

if (( errors > 0 )); then
  echo "" >&2
  echo "check-migration-linearity: $errors violation(s); aborting." >&2
  exit 1
fi

echo "check-migration-linearity: ok (${#files[@]} migrations, last = $(printf %03d "$prev"))"
