#!/usr/bin/env sh
# Regenerates the generated parts of acurl.dev from the CLI:
#   - the command reference in site/docs/index.html
#   - the agent and profile counts quoted across the site
# Usage: scripts/build-site.sh
set -eu
cd "$(dirname "$0")/.."

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

go build -o "$tmp/acurl" ./cmd/acurl

# Command reference, between the reference markers.
page=site/docs/index.html
"$tmp/acurl" reference > "$tmp/ref.html"
awk -v ref="$tmp/ref.html" '
  /<!-- reference:start -->/ { print; while ((getline line < ref) > 0) print line; skip = 1; next }
  /<!-- reference:end -->/   { skip = 0 }
  !skip
' "$page" > "$tmp/page.html"
mv "$tmp/page.html" "$page"

# Counts: every profile, the AI agents and crawlers (not the browser or curl
# baselines), and the number of categories.
"$tmp/acurl" profiles list -f json > "$tmp/profiles.json"
sed -n 's/.*"category": *"\([^"]*\)".*/\1/p' "$tmp/profiles.json" > "$tmp/categories"
profiles=$(wc -l < "$tmp/categories" | tr -d ' ')
agents=$(grep -vc -e '^browser$' -e '^other$' "$tmp/categories")
cats=$(sort -u "$tmp/categories" | wc -l | tr -d ' ')
case "$cats" in 4) word=four ;; 5) word=five ;; 6) word=six ;; 7) word=seven ;; *) word=$cats ;; esac

for f in site/index.html site/docs/index.html site/llms.txt; do
  [ -f "$f" ] || continue
  perl -pi -e "s/\\b\\d+ AI agents and crawlers/$agents AI agents and crawlers/g; s/with \\d+ profiles\\.\\.\\./with $profiles profiles.../g; s/has \\d+ profiles in \\w+ categories/has $profiles profiles in $word categories/g" "$f"
done
echo "Updated the reference and counts: $profiles profiles, $agents agents and crawlers, $cats categories"
