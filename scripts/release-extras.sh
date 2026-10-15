#!/usr/bin/env sh
# Generates man pages and shell completions for release archives and packages.
set -eu
cd "$(dirname "$0")/.."
rm -rf dist-extra
mkdir -p dist-extra/man dist-extra/completions
go run ./cmd/acurl man dist-extra/man >/dev/null
for shell in bash zsh fish; do
  go run ./cmd/acurl completion "$shell" > "dist-extra/completions/acurl.$shell"
done
