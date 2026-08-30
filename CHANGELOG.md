# Changelog

All notable changes to acurl are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-08-30

First public preview.

### Added
- `acurl <url>` fetches a URL once per agent profile with a Chrome TLS fingerprint
- Default profiles: Chrome, ChatGPT-User, OAI-SearchBot, Claude-Web, anthropic-ai,
  PerplexityBot, Googlebot, Bingbot and curl
- Challenge detection for Cloudflare, Akamai, DataDome, PerimeterX, Imperva, Kasada,
  AWS WAF, Sucuri and generic CAPTCHAs
- RFC 9309 robots.txt checks per agent, with `--no-robots` to skip them
- Classification of each result: allowed, blocked, challenged, rate-limited,
  robots-disallowed or error
- Table, JSON and Markdown output (`--format`, `--output`)
- CI-friendly exit codes: 0 allowed, 1 blocked or challenged, 2 errors
- Custom profiles via `--profiles file.yaml`, extra headers via `-H`

[Unreleased]: https://github.com/tragent-io/acurl/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tragent-io/acurl/releases/tag/v0.1.0
