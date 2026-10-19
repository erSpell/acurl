# Using acurl in CI

acurl is built for pipelines: exit codes are configurable, and output comes as JSON,
JUnit XML, SARIF and CSV.

## Exit codes and `--fail-on`

| Code | Meaning |
|---|---|
| 0 | Nothing matched the `--fail-on` policy |
| 1 | A result matched the policy (for example an AI agent was blocked) |
| 2 | A request failed, or acurl was used incorrectly |

The default policy is `--fail-on blocked,challenged,error`. Combine any of:

| Condition | Fails when |
|---|---|
| `blocked` | An agent got a 4xx/5xx status |
| `challenged` | An agent got a WAF or CAPTCHA page |
| `rate-limited` | An agent got a 429 |
| `cloaked` | An agent saw substantially different content |
| `robots` | robots.txt disallows an agent |
| `error` | A request failed at the network level (exit 2) |
| `score<N` | The AI readiness score is below N. Quote it in a shell (`'score<80'`), or `<` is read as a redirect. |
| `none` | Never fail on results |

```bash
# Fail the build if any AI assistant or search bot is blocked, or the score drops below 80
acurl --category ai-assistant,ai-search --fail-on 'blocked,challenged,score<80' https://staging.shop.example
```

## GitHub Actions

```yaml
name: AI agent access
on:
  pull_request:
  schedule:
    - cron: "0 6 * * *"

jobs:
  acurl:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write   # for SARIF upload
    steps:
      - uses: actions/setup-go@v6
        with:
          go-version: stable
      - run: go install github.com/tragent-io/acurl/cmd/acurl@latest

      - name: Check key pages
        run: acurl batch urls.txt -f sarif -o acurl.sarif --fail-on blocked,challenged

      - name: Upload to code scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: acurl.sarif
          category: acurl
```

JUnit output works with most CI test reporters (GitLab, Jenkins, CircleCI, Buildkite):

```bash
acurl batch urls.txt -f junit -o acurl-junit.xml
```

Each URL is a test suite and each agent profile a test case. Blocked, challenged, rate-limited
and cloaked results are failures. Network errors are errors. robots.txt-disallowed results are
skipped, because blocking an agent in robots.txt is a deliberate choice.

## Batch runs

```bash
acurl batch urls.txt                       # one URL per line, # comments allowed
acurl batch products.csv                   # "url" column, or the first column
acurl batch --sitemap https://shop.example/sitemap.xml --limit 100
acurl batch --sitemap auto:shop.example    # every sitemap listed in robots.txt
```

`--parallel` controls how many URLs run at once (default 2). `--delay` is the minimum gap
between checks of the same host (default 500ms), so a batch never hammers one store.

## Configuration file

Run `acurl init` to create `.acurl.yaml`. Any flag can be set there by its long name:

```yaml
category: [ai-assistant, ai-search]
fail-on: blocked,challenged,score<75
timeout: 20s
header:
  - "Cookie: country=US"
```

Environment variables override the file: `ACURL_FAIL_ON`, `ACURL_TIMEOUT`, `ACURL_AGENTS`, and so on.
Command-line flags override both.

## Tips

- Run from the same network as your users would see. Results from a cloud runner can
  differ from results on your laptop, because many WAFs score datacenter IP ranges.
- If the browser baseline itself is challenged, acurl does not score the page. The site
  is blocking the runner, not the agents.
- Use `-q` to keep logs clean and `-v` to trace every request when debugging.

---

acurl is an open-source project by [tragent.io](https://tragent.io).
