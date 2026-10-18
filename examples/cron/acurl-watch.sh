#!/bin/sh
# Hourly check of key pages; posts to Slack when an agent's access changes.
#   crontab:  0 * * * *  /opt/acurl/acurl-watch.sh
set -eu

STATE=/var/lib/acurl/state.json
mkdir -p "$(dirname "$STATE")"

acurl watch \
  https://shop.example/ \
  https://shop.example/products/best-seller \
  --interval 0 \
  --state "$STATE" \
  --category ai-assistant,ai-search \
  --webhook "$ACURL_SLACK_WEBHOOK" --webhook-format slack \
  --quiet
