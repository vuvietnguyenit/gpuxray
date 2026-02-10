#!/usr/bin/env bash
set -euo pipefail

BOILERPLATE="hack/boilerplate.txt"
SPDX="SPDX-License-Identifier: MIT"

FILES=$(git ls-files \
  '*.go' '*.c' '*.h' '*.sh' '*.py' \
  ':!:vendor/**' \
  ':!:third_party/**')

for f in $FILES; do
  if head -n 20 "$f" | grep -q "$SPDX"; then
    continue
  fi

  echo "Adding license to $f"

  tmp=$(mktemp)

  {
    # Preserve shebang
    if head -n1 "$f" | grep -q '^#!'; then
      head -n1 "$f"
      echo
      cat "$BOILERPLATE"
      tail -n +2 "$f"
    else
      cat "$BOILERPLATE"
      cat "$f"
    fi
  } > "$tmp"

  mv "$tmp" "$f"
done
