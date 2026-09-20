# acurl

> See your website the way AI agents see it.

**acurl** is a free, open-source tool by [tragent.io](https://tragent.io).

`acurl` fetches a URL several times, once per agent profile: a real Chrome browser, AI agents
such as ChatGPT-User and Claude-Web, search crawlers, and plain curl. It then shows how each one was
treated side by side.

It is built for store owners and developers who want to know whether AI shopping and
search agents can actually read their pages.

## Install

```bash
go install github.com/tragent-io/acurl/cmd/acurl@latest
```

## Usage

```bash
acurl https://example.com
acurl --agents ChatGPT-User,Googlebot https://yourstore.com/products/1
acurl --category ai-assistant,ai-search https://yourstore.com
acurl --diff GPTBot https://yourstore.com     # what text did GPTBot not see?
acurl robots https://yourstore.com/products/1  # robots.txt, content signals, llms.txt
acurl profiles list
```

Each result is classified as `allowed`, `blocked`, `challenged` (WAF or captcha page),
`rate-limited`, `cloaked` (visible text differs a lot from what Chrome got),
`robots-disallowed` or `error`.

Every run ends with an **AI readiness score** (0-100) and concrete fixes for anything that fails.

More: [profiles](docs/profiles.md) · [robots.txt and llms.txt](docs/robots.md) · [scoring](docs/scoring.md)

Output formats: `--format table` (default), `json` or `markdown`. Use `-o report.md` to write to a file.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every profile was allowed |
| 1 | At least one profile was blocked or challenged |
| 2 | A request failed |

## Status

Early development. Expect breaking changes until v1.0.

## License

Apache 2.0. See [LICENSE](LICENSE).

## About

acurl is built and maintained by [tragent.io](https://tragent.io), which helps online stores
get found, understood and bought from by AI shopping agents. acurl is the free, open-source
diagnostic we use ourselves, and contributions are welcome.
