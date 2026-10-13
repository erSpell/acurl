#!/usr/bin/env sh
# Regenerates the command reference in site/docs/index.html from the CLI.
# Usage: scripts/build-site.sh
set -eu
cd "$(dirname "$0")/.."

page=site/docs/index.html
ref=$(mktemp)
trap 'rm -f "$ref" "$ref.page"' EXIT

go run ./cmd/acurl reference > "$ref"

awk -v ref="$ref" '
  /<!-- reference:start -->/ { print; while ((getline line < ref) > 0) print line; skip = 1; next }
  /<!-- reference:end -->/   { skip = 0 }
  !skip
' "$page" > "$ref.page"
mv "$ref.page" "$page"
echo "Updated $page"
