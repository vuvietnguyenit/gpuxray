#!/usr/bin/env bash
set -euo pipefail

SPDX="SPDX-License-Identifier: MIT"

FILES=$(git ls-files \
  '*.go' '*.c' '*.h' '*.sh' '*.py' \
  ':!:vendor/**' \
  ':!:gen/**' \
  ':!:third_party/**')

missing=0

for f in $FILES; do
  # Read only the first 20 lines (header region)
  if ! head -n 20 "$f" | grep -q "$SPDX"; then
    echo "Missing license header: $f"
    missing=1
  fi
done

if [[ $missing -eq 1 ]]; then
  exit 1
fi
