# Agent profiles

A profile is one way of requesting a page: a user agent string, optional headers and the
robots.txt token the agent obeys. acurl ships with a catalog of documented AI agents and
crawlers. See them all with:

```bash
acurl profiles list
acurl profiles list --category ai-assistant
acurl profiles show GPTBot
```

## Categories

| Category | What it means | Examples |
|---|---|---|
| `browser` | The real-browser baseline every other result is compared with | Chrome |
| `ai-assistant` | Fetches a page because a user asked an assistant about it | ChatGPT-User, Claude-User, Perplexity-User |
| `ai-search` | Builds the index behind an AI search product | OAI-SearchBot, Claude-SearchBot, PerplexityBot |
| `ai-training` | Collects data for model training | GPTBot, ClaudeBot, CCBot, Google-Extended |
| `search` | Classic search engine crawler | Googlebot, Bingbot, Applebot |
| `other` | Anything else, such as the curl baseline | curl |

Run a subset with `--category`. The browser baseline is always included:

```bash
acurl --category ai-assistant,ai-search https://shop.example/products/1
```

## Robots.txt-only tokens

Some tokens, like `Google-Extended` and `Applebot-Extended`, control how content may be used
for AI. They are never sent as a user agent. acurl checks them against robots.txt and does
not fetch anything for them. In the table their status shows as `-`.

## How requests are made

- **Browser profiles** (`browser: true`) send Chrome's TLS fingerprint, HTTP/2 settings and
  client hints, so they look like a real Chrome visit.
- **Bot profiles** send only their own user agent and headers, using Go's TLS stack, as most real
  crawlers do. Pass `--tls chrome` to give bot profiles Chrome's TLS fingerprint too. Then the
  user agent is the only thing that differs from the baseline.

## Custom profiles

Write a YAML file and pass it with `--profiles`. It replaces the built-in catalog:

```yaml
profiles:
  - name: "Chrome (Browser)"
    browser: true
    category: browser
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

  - name: "InternalBot"
    category: other
    operator: Example Inc.
    user_agent: "InternalBot/2.0 (+https://example.com/bot)"
    robots_key: "InternalBot"
    headers:
      X-Team: "growth"

  - name: "Example-Extended"
    robots_only: true
    category: ai-training
    robots_key: "Example-Extended"
```

| Field | Required | Description |
|---|---|---|
| `name` | yes | Unique display name, used by `--agents` |
| `user_agent` | yes, unless `robots_only` | User-Agent header to send |
| `robots_key` | no | robots.txt product token to check |
| `headers` | no | Extra request headers |
| `browser` | no | Marks the comparison baseline |
| `robots_only` | no | Check robots.txt only, never fetch |
| `category`, `operator`, `docs` | no | Metadata shown by `acurl profiles` |

---

acurl is an open-source project by [tragent.io](https://tragent.io).
