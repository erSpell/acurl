#!/usr/bin/env sh
# Fails when the Chrome baseline profile is more than two major versions
# behind stable Chrome, so the browser profile never drifts into looking
# like a bot. Run monthly by .github/workflows/chrome-version.yml.
set -eu
cd "$(dirname "$0")/.."

ours=$(sed -n 's/.*Chrome\/\([0-9]*\)\.0\.0\.0 Safari.*/\1/p' internal/profiles/profiles.yaml | head -n1)
stable=$(curl -fsSL "https://versionhistory.googleapis.com/v1/chrome/platforms/win/channels/stable/versions?pageSize=1" \
  | sed -n 's/.*"version": *"\([0-9]*\)\..*/\1/p' | head -n1)

[ -n "$ours" ] && [ -n "$stable" ] || { echo "could not read the versions (ours=$ours stable=$stable)"; exit 2; }
echo "profile: Chrome $ours, stable: Chrome $stable"
if [ $((stable - ours)) -gt 2 ]; then
  echo "The Chrome (Browser) profile is $((stable - ours)) versions behind. Update the user agent and"
  echo "Sec-Ch-Ua in internal/profiles/profiles.yaml to Chrome $stable."
  exit 1
fi
