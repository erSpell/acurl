#!/usr/bin/env sh
# Adds ?v=<checksum> to the stylesheet and script links in the site's pages, so
# browsers fetch a changed file instead of reusing a cached copy next to new
# HTML. GitHub Pages lets browsers cache assets for 10 minutes. The deploy
# workflow runs this; the HTML in the repo keeps plain links.
# Usage: scripts/stamp-assets.sh
set -eu
cd "$(dirname "$0")/../site"

for asset in assets/*.css assets/*.js; do
  v=$(cksum "$asset" | cut -d' ' -f1)
  name=$(basename "$asset")
  for page in index.html docs/index.html 404.html; do
    perl -pi -e "s{(assets/\Q$name\E)(\?v=\w+)?\"}{\$1?v=$v\"}g" "$page"
  done
done
grep -ho 'assets/[a-z.-]*?v=[0-9]*' index.html docs/index.html 404.html | sort -u
