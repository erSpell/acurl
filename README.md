# acurl

> See your website the way AI agents see it.

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
acurl -H "Cookie: region=us" --timeout 10s https://example.com
```

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
