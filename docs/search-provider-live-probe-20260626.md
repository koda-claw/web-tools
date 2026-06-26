# Bing / Baidu / Sogou Search Provider Live Probe - 2026-06-26

## Context

- Date: 2026-06-26
- Git base before this implementation: `8852a70`
- Binary source: local `go run .`
- Network: current developer network from macOS workspace
- Scope: explicit providers only, not default auto chain

## Summary

| Provider | Network | Query | HTTP Probe | JSON Smoke | Parsed | First domain | Risk |
|----------|---------|-------|------------|------------|--------|--------------|------|
| bing | current-dev | `golang readability` | `HEAD` returned 200 | pass | 5 | `golang.google.cn` | ok |
| baidu | current-dev | `人工智能 最新进展` | `HEAD` returned captcha 302 | pass | 5 | `mp.weixin.qq.com` | GET ok, HEAD noisy |
| baidu | current-dev | `xyzzy_no_results_ever_12345` | not repeated | pass | >= 0 | not asserted | no contract break |
| sogou | current-dev | `人工智能 最新进展` | `HEAD` returned 200 | pass | 1 | `baike.sogou.com` | captcha risk; earlier browser-like GET returned anti-bot page |

## Commands

```bash
curl -I --max-time 10 "https://www.bing.com/search?q=golang+readability"
curl -I --max-time 10 "https://www.baidu.com/s?wd=golang+readability"
curl -I --max-time 10 "https://www.sogou.com/web?query=%E4%BA%BA%E5%B7%A5%E6%99%BA%E8%83%BD"

go run . web-search "golang readability" --provider bing --json --no-cache \
  | jq -e '.ok == true and .result.query and .result.engine == "bing" and .result.provider == "bing" and (.result.results | type == "array") and all(.result.results[]; .rank and .title and .url and .source and (.engines | type == "array"))'

go run . web-search "人工智能 最新进展" --provider baidu --json --no-cache \
  | jq -e '.ok == true and .result.query and .result.engine == "baidu" and .result.provider == "baidu" and (.result.results | type == "array") and all(.result.results[]; .rank and .title and .url and .source and (.engines | type == "array"))'

go run . web-search "xyzzy_no_results_ever_12345" --provider baidu --json --no-cache \
  | jq -e '.ok == true and .result.total >= 0 and (.result.results | type == "array")'

go run . web-search "人工智能 最新进展" --provider sogou --json --no-cache
```

## Findings

- Bing returned 5 parsed results for an English technical query.
- Baidu returned 5 parsed results for a Chinese current-events query when using the CLI GET path with browser-like headers.
- Baidu `HEAD` was redirected to a captcha page in the current network, so HEAD-only checks are not reliable for this provider.
- Sogou `HEAD` returned 200 and one CLI JSON smoke returned 1 parsed result. A prior browser-like GET from the same developer network returned a captcha/anti-bot page, so this network still does not justify treating Sogou as a stable fallback.
- JSON stdout kept the expected `{ ok, result }` envelope and `SearchResult` fields.
- No evidence supports adding Bing, Baidu, or Sogou to the default `auto` chain yet.
- Implementation now spaces explicit Bing/Baidu/Sogou requests by provider and retries only temporary 502/503/504 failures once; captcha, 403, and 429 still return structured `ErrRateLimited`.

## Remaining Gate

Before recommending Baidu/Bing/Sogou as default fallback providers, still require:

- China mainland direct network smoke.
- Repeated smoke over multiple runs/days.
- Reader follow-up quality check for the top 5 URLs.
- Latency and captcha/blocked rate tracking.
- Separate default-chain decision.
