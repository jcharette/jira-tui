#!/usr/bin/env sh
set -eu

roadmap="${1:-docs/roadmap.md}"

awk '
  /^### M[0-9]+:/ { milestone = $0 }
  /^Status:/ && milestone != "" {
    print milestone " - " $0
  }
' "$roadmap"

