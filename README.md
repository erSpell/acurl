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

## Status

Early development. Expect breaking changes until v1.0.

## License

Apache 2.0. See [LICENSE](LICENSE).
