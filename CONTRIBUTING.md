# Contributing to acurl

acurl is an open-source project by [tragent.io](https://tragent.io). Thanks for helping make it better. Bug reports, new agent profiles, detection
signatures and docs fixes are all welcome.

## Ground rules

- Be kind. This project follows the [Code of Conduct](CODE_OF_CONDUCT.md).
- Open an issue before starting large changes so we can agree on the approach.
- Keep pull requests focused: one feature or fix per PR.

## Development setup

You need Go 1.26 or newer.

```bash
git clone https://github.com/tragent-io/acurl.git
cd acurl
make build      # builds ./acurl
make test       # go test -race ./...
make lint       # golangci-lint v2
```

## Project layout

| Path | What lives there |
|---|---|
| `cmd/acurl` | CLI entry point |
| `internal/fetcher` | HTTP client with Chrome TLS fingerprint |
| `internal/detect` | WAF / bot-challenge signatures |
| `internal/robots` | robots.txt fetching and matching |
| `internal/classifier` | allowed / blocked / challenged classification |
| `internal/output` | table, JSON and Markdown renderers |
| `internal/profiles` | embedded agent profile catalog |
| `internal/tap` | `acurl tap` proxy, redaction and shell wrappers |
| `internal/cli` | cobra commands |
| `testdata/fixtures` | captured challenge pages and robots files |

## Adding an agent profile

Profiles live in `internal/profiles/profiles.yaml`. Only add agents whose operator
publicly documents the user agent string, and link that documentation in your PR.

```yaml
- name: "ExampleBot"
  user_agent: "Mozilla/5.0 (compatible; ExampleBot/1.0; +https://example.com/bot)"
  robots_key: "ExampleBot"
```

Then check it against a real site:

```bash
make build && ./acurl --agents ExampleBot https://example.com
```

## Adding a CLI to acurl tap

`acurl tap` supports a CLI when the CLI reads its provider's base URL from an environment
variable. Add the provider to `DefaultRoutes` and the CLI to `CLIs` in
`internal/tap/providers.go`:

```go
{Name: "example", Prefix: "/example", Upstream: "https://api.example.com"},
```
```go
{
    Name:     "example",
    Bindings: []Binding{{Env: "EXAMPLE_BASE_URL", Route: "example", Suffix: "/v1"}},
    Note:     "API key login only",
},
```

Link the CLI's documentation for the variable in your PR, and say which login modes you
checked. Then run it for real and paste the redacted output:

```bash
make build
./acurl tap                                # terminal 1
eval "$(./acurl tap env example)"; example # terminal 2
```

Never commit captured sessions: they contain prompts and, with `--keep-credentials`, keys.

## Adding a detection signature

1. Save a real challenge or block page under `testdata/fixtures/` (strip any personal data).
2. Add or extend a `Signature` in `internal/detect/detect.go`.
3. Add both positive and negative cases to `internal/detect/detect_test.go`.

False positives are worse than misses: a normal page must never be reported as a challenge.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(robots): parse Content-Signal directives
fix(detect): match HTML-encoded Akamai reference IDs
docs: explain exit codes
```

Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `build`, `chore`, `perf`.

## Pull request checklist

- [ ] `make test` and `make lint` pass
- [ ] New behavior has tests
- [ ] User-facing changes are documented in README or `docs/`
- [ ] `CHANGELOG.md` has an entry under **Unreleased**
