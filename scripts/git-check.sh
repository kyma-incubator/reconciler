#!/bin/bash
OUT=$(git status --porcelain)
AMOUNT=$(echo -n "$OUT" | wc -l)

if [ "${AMOUNT}" -ne 0 ]; then
  echo "following files was changed after generating models from Open API specs"
  echo "$OUT"
  exit "${AMOUNT}"
fi
echo "No files have changed"
exit 0
