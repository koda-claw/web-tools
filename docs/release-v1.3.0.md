# v1.3.0 发布说明草案

## 发布定位

`v1.3.0` 面向 Agent research 场景，重点是让 web-tools 从“可用的搜索/读取 CLI”升级为“更适合 Agent 编排、诊断和证据保留的本地优先工具链”。

本版本不新增 `web-research` 组合命令。当前推荐路径仍是通过 skill 编排 `web-search` 与 `web-reader`，由 Agent 负责来源选择、质量判断、交叉验证和最终表达。

## 主要变化

- 新增 `web-tools doctor`，用于检查配置、缓存目录、MarkItDown、agent-browser 和 SearXNG。
- `web-search` 支持 `--include-domain` 与 `--exclude-domain`，并对结果 URL 做规范化与去重。
- `web-search --engine auto` 在前置引擎返回空结果时，会继续尝试后续引擎；如果所有引擎都返回空结果，则返回最后一个引擎的空结果。
- `web-reader --json` 增加 `quality` metadata，用于判断正文提取质量和是否建议 fallback。
- `web-reader` 在 Markdown 输出中包含质量注释，并把稀疏内容警告写入 stderr。
- HTTP 4xx/5xx 不再触发浏览器 fallback；浏览器 fallback 只用于网络/提取失败或显式 `--browser`。
- `skills/web-tools/SKILL.md` 增加 Agent research workflow 和 policy，指导 Agent 以显式、可调试的步骤组合搜索和读取。
- 新增 `scripts/install.sh`，支持从源码 checkout 安装 CLI，并可通过 `SKILL_DIR` 同时安装 Agent skill。
- 新增 `docs/research-workflow-design.md`，明确当前只做 Agent research workflow 设计，不做 CLI 内置 summarizer。

## 兼容性

- 保持 `web-search` 与 `web-reader` 的主命令形态不变。
- JSON 输出新增字段保持向后兼容；既有字段不删除。
- `--engine auto` 的行为更积极：当前置引擎返回空结果时，会尝试后续引擎。这会提升默认 research 成功率，但可能让最终 `engine` 字段从 `searxng` 变为 `duckduckgo`。
- `web-research` 仍未实现。任何依赖组合研究命令的上层 Agent 都应继续使用 skill 中的显式编排流程。

## 验证记录

发布前需要通过：

```bash
go vet ./...
go test ./...
./scripts/smoke.sh
```

还应模拟 release workflow 的多平台构建：

```bash
GOOS=darwin  GOARCH=arm64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-darwin-arm64 .
GOOS=darwin  GOARCH=amd64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-darwin-amd64 .
GOOS=linux   GOARCH=amd64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-linux-amd64 .
GOOS=linux   GOARCH=arm64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-linux-arm64 .
GOOS=linux   GOARCH=arm   go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-linux-arm .
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-windows-amd64.exe .
GOOS=windows GOARCH=arm64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-windows-arm64.exe .
GOOS=freebsd GOARCH=amd64 go build -ldflags "-X main.version=v1.3.0" -o /tmp/web-tools-freebsd-amd64 .
```

建议再做一条真实 Agent research 冒烟：

```bash
web-tools doctor --json
web-tools web-search "Go readability library" --limit 2 --json
web-tools web-reader "https://github.com/go-shiori/go-readability" --json
```

还应验证源码安装脚本：

```bash
BIN_DIR="$(mktemp -d)" SKILL_DIR="$(mktemp -d)" sh scripts/install.sh
```

## 发版步骤

1. 确认工作区干净。
2. 确认本地 `main` 包含本版本全部提交。
3. 创建 tag：`git tag v1.3.0`。
4. 推送提交和 tag：`git push origin main && git push origin v1.3.0`。
5. GitHub Actions 会根据 `.github/workflows/release.yml` 构建并上传 release assets。

## 暂不发布的内容

- 不发布 `web-research` 命令。
- 不引入 CDP 或新的浏览器后端。
- 不把总结、引用生成或可信度判断固化进 CLI。
