# CLI 与 Skill 升级能力测试验收计划

## 目标

本文档用于验收 `web-tools upgrade` 迭代，和 `docs/upgrade-iteration-plan.md` 配套使用。

验收目标：

- 证明 CLI 能从 release asset 安全升级到目标版本。
- 证明升级后的 skill 与 CLI 默认来自同一 tag。
- 证明失败路径不会破坏原 binary、配置、env file、cache。
- 证明 Agent 可以按文档和 skill 指引完成安装、升级、检查。
- 证明 Linux、macOS、Windows 的核心差异都有测试或明确降级行为。

## 发布闸门

以下任一项失败，都不能发布：

- `go test ./...` 失败。
- `go vet ./...` 失败。
- `git diff --check` 失败。
- `./scripts/smoke.sh` 失败。
- upgrade 单元测试或集成测试失败。
- release workflow 未上传目标平台 binary 或 `checksums.txt`。
- `web-tools upgrade --check --json` 会修改文件。
- checksum 缺失、hash mismatch、版本不匹配时仍替换 binary。
- 失败路径泄漏 secret、修改 `config.json`、`.env` 或 cache。

以下为可降级但必须记录的项：

- Windows 自覆盖当前运行中的 `.exe` 失败，可以返回 `manual_replace_required=true`。
- 历史 tag 缺少 `checksums.txt`，可以默认失败，但错误必须可诊断。
- 受限文件权限导致无法替换 binary，可以失败，但原 binary 必须保留。

## 测试环境

### 本地基础环境

- macOS 或 Linux 开发机。
- Go 版本与 CI 一致。
- 可执行 `go test ./...`、`go vet ./...`、`./scripts/smoke.sh`。
- 不依赖真实 provider API key。
- 测试必须使用临时 HOME、临时 binary 目录、临时 skill 目录。

### CI 环境

CI 需要至少覆盖：

- `ubuntu-latest`
- `macos-latest`
- `windows-latest`

跨平台差异：

- Unix/macOS 覆盖真实 rename 替换、备份恢复、权限失败。
- Windows 覆盖 asset mapping、URL/checksum、下载校验、`manual_replace_required`。
- 不适用的文件权限断言使用 build tags 或运行时条件跳过。

### Fake release server

集成测试应使用 `httptest` 或本地文件 server，避免依赖 GitHub 网络状态。

最小路由：

```text
/releases/download/v9.9.9/web-tools-darwin-arm64
/releases/download/v9.9.9/web-tools-linux-amd64
/releases/download/v9.9.9/web-tools-windows-amd64.exe
/releases/download/v9.9.9/checksums.txt
/repos/koda-claw/web-tools/releases/latest
```

skill 来源可使用：

- `--skill-source <tmp>/SKILL.md`
- 或本地 HTTP URL。

## 单元测试清单

| ID | 模块 | 场景 | 期望 |
|---|---|---|---|
| UT-001 | Resolver | `--version latest` 解析 GitHub latest API | 输出明确 `target_version` |
| UT-002 | Resolver | `--version v9.9.9` | 不调用 latest API，直接使用目标 tag |
| UT-003 | Resolver | 当前版本为 `dev` | 允许升级到 latest |
| UT-004 | Resolver | `--base-url --version latest` | 返回 input error |
| UT-005 | Resolver | `--repo` 与 `--base-url` 同时传入 | release asset 使用 `--base-url`，GitHub API 仅默认路径使用 `--repo` |
| UT-006 | Asset mapping | darwin/arm64 | `web-tools-darwin-arm64` |
| UT-007 | Asset mapping | linux/amd64 | `web-tools-linux-amd64` |
| UT-008 | Asset mapping | windows/amd64 | `web-tools-windows-amd64.exe` |
| UT-009 | Asset mapping | unsupported platform | 返回 input error 并列出支持矩阵 |
| UT-010 | URL builder | 默认 GitHub URL | 生成 release asset、checksum、latest API URL |
| UT-011 | URL builder | `--base-url` | 生成 `<base-url>/<tag>/<asset>` 与 `<base-url>/<tag>/checksums.txt` |
| UT-012 | Checksum | manifest 包含目标 asset | hash match 通过 |
| UT-013 | Checksum | manifest 缺少目标 asset | 失败，不替换 binary |
| UT-014 | Checksum | hash mismatch | 失败，不替换 binary |
| UT-015 | Checksum | tag 缺少 `checksums.txt` | 默认失败，错误提示选择带 checksum 的版本 |
| UT-016 | Verifier | 临时 binary `--version` 匹配 | 通过 |
| UT-017 | Verifier | 临时 binary `--version` 不匹配 | 失败，不替换 binary |
| UT-018 | Verifier | 临时 binary 不可执行 | 失败，不替换 binary |
| UT-019 | Installer | 正常替换 | 新 binary 生效，旧 binary 备份或清理符合策略 |
| UT-020 | Installer | replace 失败 | 恢复旧 binary |
| UT-021 | Installer | 默认路径是 symlink | 拒绝并提示 `--bin` 或 `--bin-dir` |
| UT-022 | Installer | 目标目录不可写 | 失败，不修改原 binary |
| UT-023 | Installer | `--bin-dir` 指向其他目录 | JSON 输出 `binary_mode=install_to_dir` |
| UT-024 | Installer | Windows 当前 exe 自覆盖 | JSON 输出 `manual_replace_required=true` |
| UT-025 | SkillUpdater | 默认安装 target version skill | skill 来源包含 target tag |
| UT-026 | SkillUpdater | `--skill-source` 本地路径 | 从本地 SKILL.md 安装 |
| UT-027 | SkillUpdater | `--skill-source` HTTP URL | 从 HTTP URL 安装 |
| UT-028 | SkillUpdater | `--only-skill` | 不下载、不替换 binary |
| UT-029 | SkillUpdater | `--skip-skill` | 只升级 CLI，不写 skill |
| UT-030 | Output | `--json` 成功输出 | 包含 `ok`、`current_version`、`target_version`、`asset`、`binary_mode` |
| UT-031 | Output | 错误输出 | 使用 structured error 风格，建议不包含 secret |

## 集成测试清单

| ID | 场景 | 命令形态 | 期望 |
|---|---|---|---|
| IT-001 | check 模式 | `web-tools upgrade --check --json` | 输出 current/target/asset，不修改 binary、skill、config、env、cache |
| IT-002 | 显式版本升级 | `web-tools upgrade --version v9.9.9 --base-url <server> --bin <tmp-bin> --skill-dir <tmp-skills> --skill-source <tmp-skill> --json` | binary 版本变为 v9.9.9，skill 写入目标目录 |
| IT-003 | only skill | `web-tools upgrade --only-skill --version v9.9.9 --skill-dir <tmp-skills> --skill-source <tmp-skill> --json` | 只写 skill，不修改 binary |
| IT-004 | skip skill | `web-tools upgrade --skip-skill --version v9.9.9 --base-url <server> --bin <tmp-bin> --json` | 只替换 binary，不写 skill |
| IT-005 | checksum mismatch | 修改 manifest hash 后升级 | 失败，原 binary hash 不变 |
| IT-006 | missing checksum | 不提供 `checksums.txt` | 默认失败，原 binary hash 不变 |
| IT-007 | version mismatch | 新 binary 输出 v8.8.8 | 失败，原 binary hash 不变 |
| IT-008 | HTTP 404 | asset URL 返回 404 | 失败，错误建议检查版本和平台 |
| IT-009 | network failure | server 关闭或超时 | 失败，错误建议检查网络、代理或 `--base-url` |
| IT-010 | symlink binary | `--bin` 或默认路径为 symlink | 默认拒绝，提示显式路径 |
| IT-011 | install_to_dir | `--bin-dir <tmp-dir>` | 安装到目录，JSON 标记 `binary_mode=install_to_dir` |
| IT-012 | no secret leak | 临时 env 中设置 fake token | stdout/stderr/JSON 都不包含 token |

## CLI 冒烟测试

开发完成后，本地至少执行：

```bash
go test ./...
go vet ./...
./scripts/smoke.sh
git diff --check
```

新增或扩展 upgrade smoke：

```bash
./scripts/upgrade_smoke.sh
```

`upgrade_smoke.sh` 应覆盖：

- fake release server。
- fake old/new binary。
- `--check --json`。
- 正常升级。
- `--only-skill`。
- checksum mismatch。
- 原 binary hash 不变断言。

## 文档与 Agent 验收

| ID | 文件 | 验收点 |
|---|---|---|
| DOC-001 | `README.md` | 有英文 Upgrade 小节，命令与真实 release asset 一致 |
| DOC-002 | `README.zh-CN.md` | 有中文升级说明，和英文 README 不冲突 |
| DOC-003 | `skills/web-tools/SKILL.md` | 英文 skill 指导 Agent 优先使用 `web-tools upgrade` |
| DOC-004 | `internal/gui/guide.go` | Agent Guide 不出现不存在的 `.tar.gz` asset |
| DOC-005 | `CHANGELOG.md` | 记录 upgrade、checksum、skill 同版本安装、GUI guide 修正 |

文档检查命令：

```bash
rg -n "web-tools upgrade|skill install|checksums.txt|WEB_TOOLS_ENV" README.md README.zh-CN.md skills/web-tools/SKILL.md docs/*.md
rg -n "tar.gz|web-tools_Darwin" internal/gui docs README.md README.zh-CN.md
```

第二条命令允许历史说明中解释“不是 tar.gz”，但 GUI guide 和安装指引不能推荐不存在的 asset。

## 发布前验收

发布前按顺序执行：

1. 确认工作区只包含本版本相关修改。
2. 执行 `go test ./...`。
3. 执行 `go vet ./...`。
4. 执行 `./scripts/smoke.sh`。
5. 执行 `./scripts/upgrade_smoke.sh`。
6. 执行 `git diff --check`。
7. 本地模拟 release 构建和 checksum 生成：

```bash
tmp_dist="$(mktemp -d)"
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=vX.Y.Z" -o "$tmp_dist/web-tools-darwin-arm64" .
(cd "$tmp_dist" && rm -f checksums.txt && for f in web-tools-*; do if command -v sha256sum >/dev/null 2>&1; then sha256sum "$f"; else shasum -a 256 "$f"; fi; done > checksums.txt)
grep web-tools-darwin-arm64 "$tmp_dist/checksums.txt"
```

8. 用临时目录执行本地构建冷启动：

```bash
tmp_home="$(mktemp -d)"
HOME="$tmp_home" "$tmp_dist/web-tools-darwin-arm64" --version
HOME="$tmp_home" "$tmp_dist/web-tools-darwin-arm64" setup --check --json
```

发布前尚未创建 GitHub tag 时，不用本地构建 binary 访问真实 GitHub latest。升级链路由 `./scripts/upgrade_smoke.sh` 的 fake release server 覆盖；真实 GitHub release 验收放到发布后执行。

## 发布后验收

tag 发布并上传 release assets 后执行：

```bash
tmp_dir="$(mktemp -d)"
curl -L "https://github.com/koda-claw/web-tools/releases/download/vX.Y.Z/web-tools-darwin-arm64" -o "$tmp_dir/web-tools"
curl -L "https://github.com/koda-claw/web-tools/releases/download/vX.Y.Z/checksums.txt" -o "$tmp_dir/checksums.txt"
grep web-tools-darwin-arm64 "$tmp_dir/checksums.txt"
chmod +x "$tmp_dir/web-tools"
"$tmp_dir/web-tools" --version
"$tmp_dir/web-tools" upgrade --check --json
"$tmp_dir/web-tools" skill install --dir "$tmp_dir/skills" --force --json
```

如果存在可用旧版 binary，再执行真实升级：

```bash
"$tmp_dir/web-tools-old" upgrade --version vX.Y.Z --bin "$tmp_dir/web-tools-old" --skill-dir "$tmp_dir/skills" --json
"$tmp_dir/web-tools-old" --version
```

验收通过条件：

- release asset 可下载。
- `checksums.txt` 包含目标 asset。
- 下载后 binary 可执行并输出目标版本。
- `upgrade --check --json` 不修改本地文件。
- skill 可以安装到临时目录。
- 从旧版升级后输出目标版本。

## 回归范围

`web-tools upgrade` 不应破坏现有功能。发布前仍需要确认：

- `web-tools web-search "Go readability library" --limit 2 --json`
- `web-tools web-reader https://example.com --json`
- `web-tools setup --check --json`
- `web-tools doctor --json`
- `web-tools gui --no-open --port 0` 的 server smoke

## 验收记录模板

```markdown
## Upgrade 验收记录

- 版本：vX.Y.Z
- 日期：
- 操作人：
- commit：
- tag：

### 本地验证

- [ ] go test ./...
- [ ] go vet ./...
- [ ] ./scripts/smoke.sh
- [ ] ./scripts/upgrade_smoke.sh
- [ ] git diff --check

### CI

- [ ] ubuntu-latest
- [ ] macos-latest
- [ ] windows-latest

### 发布后

- [ ] release asset 可下载
- [ ] checksums.txt 可下载并包含目标 asset
- [ ] 临时目录冷启动通过
- [ ] 旧版到新版真实升级通过
- [ ] skill 安装到临时目录通过

### 风险备注

- Windows self-replace：
- 历史 tag checksum：
- 其他：
```
