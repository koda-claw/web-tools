# Spec: DuckDuckGo Lite 作为默认搜索引擎 + SearXNG 作为高级选项

## 背景

当前 web-search 强依赖 SearXNG Docker 容器（localhost:8888），社区用户没有 Docker 环境就无法使用搜索功能。需要让搜索开箱即用。

## 设计决策

**默认引擎改为 DuckDuckGo Lite HTML 抓取**（零依赖，只要有网就能用），SearXNG 降级为 opt-in 高级选项。

降级链路：`auto` 模式下先尝试 SearXNG，不可用则自动降级到 DuckDuckGo Lite。

## 改动清单

### 1. 新增 `internal/search/engine.go` — Engine 接口

```go
type Engine interface {
    Name() string
    HealthCheck() error
    Query(query string, opts SearchOptions) ([]RawResult, error)
}

type RawResult struct {
    Title   string
    URL     string
    Snippet string
    Source  string
    Extra   map[string]string // 引擎特有字段，如 published_date
}
```

### 2. 新增 `internal/search/duckduckgo.go` — DuckDuckGo Lite 引擎

实现 `Engine` 接口：

- 请求 `https://lite.duckduckgo.com/lite?q=<query>&kl=<locale>`
- 解析 HTML 表格提取搜索结果（title、url、snippet）
- 支持的 options：Limit、Locale（映射到 DDG 的 `kl` 参数，如 `zh-cn`、`en-us`、`wt-wt`）
- 不支持：Category、TimeRange（DDG Lite 不支持这些参数，静默忽略）
- User-Agent 需要模拟浏览器，否则 DDG 可能返回空结果或 captcha
- HTTP 超时 10s

HTML 解析要点：
- DDG Lite 返回 `<table>` 结构，每个 `<tr>` 是一条结果
- 第一个链接是 title+url，第二个是 snippet
- 需要处理反爬：检测到 captcha 页面时返回明确错误
- 使用 `golang.org/x/net/html` 做解析，不引入第三方 HTML 库

### 3. 重构 `internal/search/searxng.go` — 实现 Engine 接口

- `SearXNGClient` 改为 `SearXNGEngine`，实现 `Engine` 接口
- `Query` 返回 `[]RawResult`
- `HealthCheck` 逻辑不变
- 内部结构体 `SearXNGResult`、`SearXNGOptions` 保持私有

### 4. 重构 `internal/search/search.go` — 多引擎调度

改动点：
- `Search` struct 持有 `[]Engine` 而不是单个 `*SearXNGClient`
- `NewSearch` 根据 config 构建引擎列表
- `Do` 方法改为：
  - `--engine auto`（默认）：遍历引擎列表，第一个成功的就用
  - `--engine duckduckgo`：只用 DDG
  - `--engine searxng`：只用 SearXNG
- `SearchResponse.Engine` 字段记录实际使用的引擎名称
- `Do` 方法不再自己做 HealthCheck，由 engine 内部处理
- HealthCheck 失败的引擎在 auto 模式下静默跳过，日志输出到 stderr

### 5. 修改 `internal/config/config.go` — 新增引擎配置

```go
type SearchConfig struct {
    SearXNGURL    string `json:"searxng_url"`
    DefaultLimit  int    `json:"default_limit"`
    DefaultLocale string `json:"default_locale"`
    DefaultEngine string `json:"default_engine"` // "auto" / "duckduckgo" / "searxng"，默认 "auto"
}
```

### 6. 修改 `cmd/web-search/main.go` — 更新 flag 和 help 文案

- `--engine` 默认值从 `searxng` 改为 `auto`
- flag description 更新为 `Search engine: auto / duckduckgo / searxng`
- Short/Long 描述更新
- `run` 函数中移除 engine 非 searxng 就报错的硬编码检查

### 7. 新增 `internal/search/duckduckgo_test.go` — 单元测试

- TestParseResults：测试 HTML 解析正确性
- TestCaptchaDetection：测试 captcha 页面识别
- TestEmptyResults：测试空结果处理
- 用固定 HTML fixture 做测试，不依赖网络

### 8. 更新 `internal/search/search_test.go`

- 现有测试适配新的 Engine 接口
- 新增 auto 模式降级测试：SearXNG 不可用时降级到 DDG

### 9. 更新 SKILL.md 文档

- Prerequisites 中 SearXNG 从 required 改为 optional
- `--engine` 说明更新三个选项
- 新增 auto 模式行为说明
- 新增 DuckDuckGo 已知限制

## 不改的东西

- `web-reader` 完全不受影响
- `SearchResult` 和 `SearchResponse` 公开类型不变（下游消费者无需改动）
- `RenderMarkdown` / `RenderJSON` 不变
- error 处理框架不变
- config 文件格式向后兼容（新字段有默认值）

## DuckDuckGo Lite 已知限制

| 功能 | 支持 | 说明 |
|------|------|------|
| 基础搜索 | ✅ | 标题、URL、摘要 |
| 中文搜索 | ✅ | 通过 kl 参数 |
| 图片搜索 | ❌ | DDG Lite 不支持 |
| 新闻搜索 | ❌ | DDG Lite 不支持 |
| 时间过滤 | ❌ | DDG Lite 不支持 |
| 结果数量 | 有限 | 最多约 30 条，通常 20 条左右 |

这些限制在 `--engine duckduckgo` 时静默忽略相关 flag，在 `--engine auto` 降级到 DDG 时在 stderr 输出警告。

## 实施顺序

1. `engine.go` — 定义接口
2. `searxng.go` — 改为实现接口
3. `duckduckgo.go` — 新引擎实现 + 测试
4. `search.go` — 多引擎调度
5. `config.go` — 新增配置字段
6. `cmd/web-search/main.go` — 更新入口
7. SKILL.md — 更新文档
8. 集成测试：auto 模式下 SearXNG 挂掉 → DDG 兜底
