# Changelog

All notable changes to acurl are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.0] - 2026-09-27

### Added
- AI readiness score (0-100, A-F) with weighted checks and a recommendation for each failure
- Product data extraction from JSON-LD (including `@graph`, ProductGroup variants and
  AggregateOffer), microdata and OpenGraph, plus per-agent price and availability comparison
- Client-rendered (SPA) page detection, and `--render` to measure JavaScript-only text in headless Chrome
- `acurl batch` for URL lists, CSV files, stdin and sitemaps (`--sitemap`, `--sitemap auto:<site>`),
  with `--parallel`, `--limit` and a per-host `--delay`
- `--fail-on` policy: blocked, challenged, rate-limited, cloaked, robots, error, `score<N`, none
- CSV, JUnit XML and SARIF output formats
- `.acurl.yaml` config files, `ACURL_*` environment variables and `acurl init`
- `--quiet`, `--verbose` request tracing, man pages and shell completions
- Public Go API in `pkg/acurl`

### Fixed
- Small static pages that load one script are no longer flagged as client-rendered
- Pages are not scored when the browser baseline itself was blocked


## [0.2.0] - 2026-09-13

### Added
- `acurl robots <url>`: per-agent robots.txt audit with Content-Signal directives,
  sitemaps and llms.txt / llms-full.txt validation
- `acurl profiles list|show` with `--category` filtering
- 13 new profiles: GPTBot, ClaudeBot, Claude-User, Claude-SearchBot, Perplexity-User,
  MistralAI-User, DuckAssistBot, Meta-ExternalAgent, Amazonbot, Bytespider, CCBot, Applebot,
  plus robots.txt-only tokens Google-Extended and Applebot-Extended
- Profile metadata: `category`, `operator`, `docs`, `browser`, `robots_only`
- `--category` to run one class of agents
- Cloaking detection that compares visible text with the browser baseline, `--cloak-threshold`
- `--diff <profile>` prints a visible-text diff against the browser
- X-Robots-Tag and meta robots directives per agent, including agent-specific tags
- Redirect chains and final URL in results
- `--retries` with exponential backoff for network and gateway errors
- `--proxy`, `--insecure`, `--http 1.1|2`, `-X`, `-I/--head`, `--max-body`, `--tls go|chrome`

### Fixed
- Bot profiles no longer send Chrome client hints and a zh-CN Accept-Language
- `duration_ms` is now milliseconds, not nanoseconds
- Akamai block pages with HTML-encoded reference IDs are detected
- Huge Retry-After values no longer overflow (found by fuzzing)
- URLs without a scheme default to https


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

[Unreleased]: https://github.com/tragent-io/acurl/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/tragent-io/acurl/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tragent-io/acurl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tragent-io/acurl/releases/tag/v0.1.0
