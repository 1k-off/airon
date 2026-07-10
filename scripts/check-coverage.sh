#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-80}"
failed=0
checked=0

output="$(go test -cover ./internal/... 2>&1)" || {
  echo "$output"
  exit 1
}

while IFS= read -r line; do
  pkg=$(echo "$line" | awk '{print $2}')
  pct=$(echo "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')
  if [[ -z "$pct" ]]; then
    continue
  fi
  checked=$((checked + 1))
  if awk -v p="$pct" -v t="$threshold" 'BEGIN { exit !(p+0 < t+0) }'; then
    echo "FAIL $pkg ${pct}% < ${threshold}%"
    failed=1
  else
    echo "OK   $pkg ${pct}%"
  fi
done < <(echo "$output" | grep 'coverage:')

if [[ "$checked" -eq 0 ]]; then
  echo "Coverage gate failed: no packages measured"
  echo "$output"
  exit 1
fi

if [[ "$failed" -ne 0 ]]; then
  echo "Coverage gate failed (threshold ${threshold}%)"
  exit 1
fi

echo "Coverage gate passed (threshold ${threshold}%)"
