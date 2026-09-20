# AI readiness score

Every `acurl <url>` run ends with a score from 0 to 100 and a letter grade. It summarises how
well AI agents can read and use the page. It is calculated only from what acurl observed in
that run, so it is reproducible and easy to explain.

```
AI readiness 64/100 (D)
  ✗ AI assistants can fetch the page               12.5/25  1 of 2 allowed: ChatGPT-User (challenged)
  ✗ AI search crawlers can fetch the page           0.0/20  0 of 1 allowed: OAI-SearchBot (blocked)
  ...
```

## Checks

| Check | Weight | Passes when |
|---|---:|---|
| AI assistants can fetch the page | 25 | Every `ai-assistant` profile (ChatGPT-User, Claude-User, Perplexity-User, ...) is `allowed` |
| AI search crawlers can fetch the page | 20 | Every `ai-search` profile (OAI-SearchBot, Claude-SearchBot, PerplexityBot, ...) is `allowed` |
| No bot challenges for AI agents | 10 | No AI profile got a WAF or CAPTCHA challenge |
| AI agents see the same content as browsers | 10 | No AI profile was classified `cloaked` |
| Content is in the HTML without JavaScript | 15 | The raw HTML is not a client-rendered shell. With `--render`, less than 20% of the text needs JavaScript |
| Product structured data is present | 10 | The browser saw a JSON-LD `Product` with price and availability |
| AI agents see the same price and availability | 10 | Every allowed AI agent got the same product data as the browser |

Checks earn partial credit. For example, 3 of 4 assistants allowed earns 75% of that check's weight.

## Checks that do not apply

A check is skipped, and the remaining weights re-normalised to 100, when it cannot be judged:

- No profiles of that category were in the run (for example with `--category ai-search`)
- The browser baseline failed, so there is nothing to compare against
- The page has no product data at all. Product checks only apply to product pages.

## What the score does not penalise

**Blocking training crawlers is a business decision, not a defect.** GPTBot, ClaudeBot, CCBot and
Google-Extended being disallowed in robots.txt costs no points. Training crawlers only count
against the challenge and cloaking checks, which measure whether agents get the real page.

## Grades

| Score | Grade |
|---|---|
| 90-100 | A |
| 80-89 | B |
| 65-79 | C |
| 50-64 | D |
| 0-49 | F |

## Using the score in CI

The JSON report includes the full breakdown under `score`:

```bash
acurl -f json https://shop.example/products/1 | jq '.score.value'
```

---

acurl is an open-source project by [tragent.io](https://tragent.io).
