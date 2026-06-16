# CLI 与 Skill 升级能力迭代计划

## 背景

当前 `web-tools` 已经可以通过以下方式安装或更新本地 CLI 与 skill：

```bash
VERSION=v1.5.2 BIN_DIR="$HOME/.local/bin" SKILL_DIR="$HOME/.codex/skills" sh scripts/install.sh
web-tools skill install --force
```

但这个路径对人类和 Agent 都不够直接：

- 已安装的旧 CLI 没有 `upgrade` / `self-update` 命令。
- `skill install` 会按当前 CLI 版本下载匹配的 GitHub tag；如果本地 CLI 还是旧版本，skill 也可能被安装成旧版本。
- README、GUI Agent Guide、Skill 文档里有安装和更新命令，但没有形成“发现版本 → 下载 binary → 替换 CLI → 更新 skill → 验证”的闭环。
- GUI Agent Guide 中的 release asset 示例需要与实际 release workflow 对齐。当前 release workflow 上传的是单文件 binary，例如 `web-tools-darwin-arm64`，不是 `web-tools_Darwin_arm64.tar.gz`。

本轮目标是让“把本地 web-tools 升到最新版”成为一个明确、可测试、可被 Agent 可靠执行的流程。

## 总体目标

实现一个一等升级入口：

```bash
web-tools upgrade
```

默认行为：

1. 查询 GitHub Releases 最新版本。
2. 根据当前 `GOOS/GOARCH` 选择 release asset。
3. 下载 release asset 和 checksum manifest。
4. 校验 SHA256、可执行权限和 `--version`。
5. 原子替换当前 CLI binary。
6. 使用目标版本安装或更新 Agent skill。
7. 输出非敏感诊断摘要，提示下一步可运行 `web-tools setup --check --json`。

## 非目标

- 不在本轮实现 Homebrew、npm、apt、winget 等包管理器。
- 不在本轮实现自动后台更新或定时检查。
- 不在本轮要求用户必须使用远程 provider。
- 不把任何 secret、env file 值或 token 写入 release、日志或诊断输出。

## 设计原则

- **Agent 可执行**：默认非交互，失败时给明确建议。
- **可回滚**：替换 binary 前保留备份；替换失败时恢复旧 binary。
- **不破坏本地配置**：升级只替换 binary 和 skill，不修改 `config.json`、`.env`、cache。
- **版本一致**：升级后的 CLI 与安装的 skill 默认来自同一 tag。
- **可验证下载**：release 必须提供 checksum，upgrade 必须验证目标 binary。
- **显式覆盖**：允许 `--version` 指定目标版本；允许 `--only-skill` 只更新 skill。
- **跨平台**：macOS/Linux/Windows 都有明确 asset 映射和测试覆盖。

## 命令设计

### 推荐命令

```bash
web-tools upgrade
web-tools upgrade --version v1.5.2
web-tools upgrade --bin "$HOME/.local/bin/web-tools"
web-tools upgrade --skill-dir "$HOME/.codex/skills"
web-tools upgrade --skill-source ./skills/web-tools/SKILL.md
web-tools upgrade --only-skill
web-tools upgrade --check
web-tools upgrade --json
```

### 参数建议

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--version` | `latest` | 目标版本。支持 `latest` 或 `vX.Y.Z`。 |
| `--repo` | `koda-claw/web-tools` | GitHub repo，便于测试或 fork。 |
| `--base-url` | 空 | 可选 release asset 下载基地址，便于离线测试或企业镜像。为空时使用 GitHub Releases。 |
| `--bin` | 当前可执行路径 | 要替换的 CLI 路径。无法定位时建议传入显式路径。 |
| `--bin-dir` | 当前可执行所在目录 | 与 `--bin` 二选一；用于安装到指定目录。若目录不是当前 binary 所在目录，输出语义应为 install_to_dir。 |
| `--skill-dir` | `~/.codex/skills` | skill 根目录。 |
| `--skill-source` | 空 | 可选 SKILL.md 本地路径或 HTTP(S) URL。为空时按目标版本从 GitHub raw 下载。 |
| `--skip-skill` | `false` | 只升级 CLI，不安装 skill。 |
| `--only-skill` | `false` | 不升级 CLI，只安装当前/指定版本 skill。 |
| `--check` | `false` | 只检查本地版本、最新版本、asset 可用性，不修改文件。 |
| `--force` | `false` | 当前版本等于目标版本时仍重装。 |
| `--insecure-skip-checksum` | `false` | 仅用于本地测试；跳过 SHA256 校验。默认禁止跳过。 |
| `--json` | `false` | 输出结构化 JSON。 |

### 输出建议

Human 输出：

```text
web-tools upgrade
current: v1.5.0
target:  v1.5.2
asset:   web-tools-darwin-arm64
binary:  /Users/.../.local/bin/web-tools
skill:   /Users/.../.codex/skills/web-tools/SKILL.md

Downloaded target binary
Replaced CLI
Installed skill
Done: web-tools version v1.5.2
```

JSON 输出：

```json
{
  "ok": true,
  "current_version": "v1.5.0",
  "target_version": "v1.5.2",
  "asset": "web-tools-darwin-arm64",
  "checksum_verified": true,
  "binary_path": "/Users/example/.local/bin/web-tools",
  "binary_mode": "replace_current",
  "skill_path": "/Users/example/.codex/skills/web-tools/SKILL.md",
  "cli_updated": true,
  "skill_updated": true
}
```

失败时必须使用现有 structured error 风格，并给出建议：

- 无法定位当前 binary：提示 `--bin` 或 `--bin-dir`。
- release asset 不存在：提示检查目标版本和平台。
- 权限不足：提示使用可写目录或手动移动 binary。
- checksum 缺失或不匹配：停止升级，提示检查 release 或重试。
- 下载失败：提示网络、代理、`--base-url` 或手动 release URL。
- 替换失败：尝试恢复备份并报告结果。

### 参数组合规则

- `--bin` 与 `--bin-dir` 互斥。
- `--skip-skill` 与 `--only-skill` 互斥。
- `--base-url` 只覆盖 release asset 与 `checksums.txt` 下载地址，不影响 skill 来源。
- `--repo` 只用于默认 GitHub API / GitHub Releases 路径；如果同时传入 `--base-url`，release asset 下载以 `--base-url` 为准。
- 第一阶段 `--base-url` 必须搭配显式 `--version vX.Y.Z`。如果用户传入 `--base-url --version latest`，直接返回 input error，提示使用明确版本或默认 GitHub latest。
- `--skill-source` 优先级高于默认 GitHub raw skill URL，用于源码 checkout、企业镜像或测试环境。
- `--only-skill --version latest` 仍需要解析明确目标版本；如果无法解析 latest，不应安装 skill。

## 版本与 Asset 解析

### 目标版本

- `--version latest`：优先调用 GitHub Releases latest API 获取明确 tag，例如 `v1.5.2`。
- `--version vX.Y.Z`：下载对应 tag 下的 asset。
- 如果当前版本是 `dev`，仍允许升级到 latest。
- 如果当前版本已等于目标版本，默认跳过 CLI 下载，但仍可根据参数更新 skill；`--force` 则重新下载。
- 如果 GitHub API 不可用，允许 fallback 到 latest download URL，但必须在下载后运行临时 binary `--version` 反推出明确 tag，再用该 tag 安装 skill。
- `--check --json` 必须输出最终解析出的 `target_version`；如果无法解析，不应执行升级。

### Release checksum

当前 release workflow 只上传单文件 binary。升级命令下载可执行文件前，应先把 release workflow 扩展为上传 checksum manifest：

```text
checksums.txt
```

格式建议沿用常见 SHA256SUMS：

```text
<sha256>  web-tools-darwin-arm64
<sha256>  web-tools-linux-amd64
<sha256>  web-tools-windows-amd64.exe
```

`web-tools upgrade` 默认必须：

1. 下载目标 asset。
2. 下载同 tag 的 `checksums.txt`。
3. 找到目标 asset 的 SHA256。
4. 校验下载文件哈希。
5. 校验临时 binary `--version` 与目标 tag 一致。

如果 `checksums.txt` 不存在或缺少目标 asset，默认失败。仅本地测试允许使用 `--insecure-skip-checksum`，并且 human / JSON 输出都必须标记 `checksum_verified=false`。

注意：历史 release tag 可能没有 `checksums.txt`。`upgrade --version <old-tag>` 在默认安全模式下可以失败，但错误必须说明该 tag 缺少 checksum，并建议选择带 checksum 的新版本；`--insecure-skip-checksum` 只允许测试或临时验证使用，不能出现在 Agent 默认指引中。

### Asset 命名

当前 release workflow 上传单文件 binary：

```text
web-tools-darwin-arm64
web-tools-darwin-amd64
web-tools-linux-amd64
web-tools-linux-arm64
web-tools-linux-arm
web-tools-windows-amd64.exe
web-tools-windows-arm64.exe
web-tools-freebsd-amd64
```

升级命令应复用这套命名，不引入 tar.gz 假设。

### 平台映射

| GOOS | GOARCH | Asset |
|---|---|---|
| `darwin` | `arm64` | `web-tools-darwin-arm64` |
| `darwin` | `amd64` | `web-tools-darwin-amd64` |
| `linux` | `amd64` | `web-tools-linux-amd64` |
| `linux` | `arm64` | `web-tools-linux-arm64` |
| `linux` | `arm` | `web-tools-linux-arm` |
| `windows` | `amd64` | `web-tools-windows-amd64.exe` |
| `windows` | `arm64` | `web-tools-windows-arm64.exe` |
| `freebsd` | `amd64` | `web-tools-freebsd-amd64` |

不支持的平台返回 input error，并列出支持矩阵。

## 本地替换策略

### Binary 路径定位

默认路径解析规则建议：

1. 如果用户传入 `--bin`，使用该路径作为替换目标。
2. 如果用户传入 `--bin-dir`，目标为 `<bin-dir>/web-tools`，Windows 为 `<bin-dir>/web-tools.exe`。
3. 否则使用 `os.Executable()` 获取当前正在运行的可执行文件路径。
4. 对默认路径执行 `filepath.EvalSymlinks`：
   - 如果 resolved path 与原 path 相同，直接替换。
   - 如果原 path 是 symlink，默认不自动替换 symlink target，避免破坏包管理器或用户自定义链接；返回可诊断错误，提示用户显式传入 `--bin` 或 `--bin-dir`。
   - 如果未来要支持 symlink，可新增 `--follow-symlink`，但第一阶段不默认启用。

为什么不默认替换 PATH 命中的入口：

- 运行中的进程无法可靠知道用户下一次 shell 会命中哪个 `web-tools`。
- `PATH` 中可能有多个同名 binary。
- `os.Executable()` 更接近当前实际执行文件；不确定时要求用户显式指定路径更安全。
- 如果用户传入 `--bin-dir`，目标路径是 `<bin-dir>/web-tools` 或 `<bin-dir>/web-tools.exe`。当该路径不是当前运行 binary 时，输出必须把动作描述为“安装到指定目录”，而不是“替换当前 CLI”。

`--check --json` 应输出：

```json
{
  "binary_path": "/Users/example/.local/bin/web-tools",
  "binary_is_symlink": false,
  "binary_writable": true,
  "binary_mode": "replace_current"
}
```

### Unix/macOS

1. 下载到同目录临时文件，例如 `.web-tools.tmp.<pid>`。
2. `chmod 0755`。
3. 运行 `tmp --version`，确认输出包含目标版本。
4. 将当前 binary 重命名为 `.web-tools.bak.<timestamp>`。
5. 将临时文件 rename 到目标路径。
6. 运行新 binary `--version`。
7. 成功后可删除备份，或保留最近一次备份。

注意：当前进程不会变成新版本。升级命令完成后的最终验证必须通过执行目标路径的新 binary 完成，而不是假设当前进程的内存代码已经更新。

权限处理：

- 如果目标目录不可写，直接失败，不尝试 sudo。
- 错误建议使用用户可写目录，例如 `--bin-dir "$HOME/.local/bin"`。
- 不自动修改父目录权限。

文件系统处理：

- rename 在同一目录内执行，保证同文件系统原子性。
- 临时文件必须与目标 binary 位于同一目录。
- 如果校验失败，删除临时文件并保留原 binary。

### Windows

Windows 上正在运行的 `.exe` 可能无法覆盖自身。建议策略：

- 如果目标路径不是当前运行中的 exe，且可覆盖，则直接替换。
- 如果目标路径是当前运行中的 exe，第一阶段不保证自覆盖：
  - 下载并校验到同目录新文件，例如 `web-tools.exe.new`。
  - 输出明确建议：关闭当前进程后手动替换，或重新运行 `web-tools upgrade --bin <other-path>`。
  - JSON 输出 `cli_updated=false`、`downloaded_path=<new file>`、`manual_replace_required=true`。

第一阶段可以把 Windows 自替换作为受限能力：能下载并校验，不能保证覆盖正在运行的 exe。

Windows 后续增强：

- 可选生成 `.cmd` 或 PowerShell replace script，在当前进程退出后替换。
- 该增强涉及安全和杀进程风险，不进入第一阶段默认路径。

## Skill 更新策略

升级 CLI 后默认执行等价逻辑：

```bash
web-tools skill install --force --dir "$HOME/.codex/skills"
```

但实现上不要再 shell out 给旧 binary。建议：

- 将现有 `cmd/skill` 中的安装逻辑下沉到 `internal/skillinstall`。
- `cmd/skill` 和 `cmd/upgrade` 都调用 `internal/skillinstall`，避免 `internal/upgrade` 反向依赖 Cobra command 包。
- 目标版本用 upgrade 的 `target_version`。
- 默认从 `https://raw.githubusercontent.com/koda-claw/web-tools/<target>/skills/web-tools/SKILL.md` 下载。
- 如果传入 `--skill-source`，则从该本地路径或 HTTP(S) URL 读取 SKILL.md。
- `--only-skill` 可用于本地 CLI 已经是最新但 skill 旧的场景。

## 实现分层建议

为保证可测试性，建议新增 `internal/upgrade`，CLI 层只做参数解析和输出编排。

```text
cmd/upgrade
  └── parses flags, renders human/json, maps errors

internal/skillinstall
  └── installs SKILL.md from version, local path, or HTTP(S) URL

internal/upgrade
  ├── Resolver      # current version, target version, asset name, URLs
  ├── Downloader    # downloads asset/checksums through interface
  ├── Verifier      # SHA256 + executable --version validation
  ├── Installer     # binary path resolution, replace, rollback
  └── SkillUpdater  # calls internal/skillinstall with target version/source
```

### Downloader interface

不要把 GitHub 网络调用写死在核心逻辑里。建议使用小接口：

```go
type Downloader interface {
    Get(ctx context.Context, url string) ([]byte, error)
    DownloadFile(ctx context.Context, url string, dst string) error
}
```

测试中使用 httptest server 或 fake downloader。真实实现再拼接 GitHub Releases URL。

### URL 生成

默认 GitHub URL：

```text
https://github.com/<repo>/releases/download/<tag>/<asset>
https://github.com/<repo>/releases/download/<tag>/checksums.txt
https://api.github.com/repos/<repo>/releases/latest
```

如果设置 `--base-url`，则用于测试或企业镜像：

```text
<base-url>/<tag>/<asset>
<base-url>/<tag>/checksums.txt
```

`--base-url` 模式下，第一阶段必须搭配显式 `--version vX.Y.Z`，降低实现复杂度。后续如果企业镜像提供 latest endpoint，可再扩展 `--latest-url` 或 mirror manifest。

## 与现有命令的关系

- `scripts/install.sh`：继续服务源码 checkout 安装。
- `make install-local`：继续服务开发者从本地源码安装。
- `web-tools skill install`：继续服务只更新 skill。
- `web-tools setup`：继续服务配置 provider/env/doctor，不负责下载新 CLI。
- `web-tools upgrade`：负责 release binary 与 skill 的版本一致升级。

## 文档更新范围

本轮需要同步：

- `.github/workflows/release.yml`：生成并上传 `checksums.txt`。
- `.github/workflows/ci.yml`：增加 macOS/Windows/Linux 测试矩阵，至少覆盖跨平台可验证逻辑。
- `README.md`：新增 Upgrade 小节。
- `README.zh-CN.md`：新增中文升级说明。
- `skills/web-tools/SKILL.md`：Agent 遇到旧版本时优先运行 `web-tools upgrade`。
- `docs/gui-iteration-plan.md` 或 GUI guide：修正 release asset 示例。
- `internal/gui/guide.go`：Agent Guide 的安装命令必须使用真实 asset 名称。
- `docs/upgrade-test-acceptance.md`：独立测试验收清单，覆盖单元、集成、跨平台、发布前和发布后验收。
- `CHANGELOG.md`：新增版本条目。

## 实施任务

### Wave 1：设计与文档

#### Task 1：冻结升级语义

**文件：** `docs/upgrade-iteration-plan.md`

**实现：**

- 明确 `upgrade` 命令默认行为、参数、失败策略。
- 明确 release asset 命名与平台矩阵。

**验证：**

- 文档覆盖 CLI、skill、版本来源、回滚、测试用例。

#### Task 2：同步用户文档

**文件：** `README.md`、`README.zh-CN.md`、`skills/web-tools/SKILL.md`

**实现：**

- 新增升级说明。
- Agent skill 中写明旧版本升级流程。
- 保留源码安装路径作为 fallback。

**验证：**

- `rg -n "web-tools upgrade|VERSION=.*install.sh|skill install" README.md README.zh-CN.md skills/web-tools/SKILL.md`
- 中英文文档不混写。

### Wave 2：核心 CLI

#### Task 3：为 release 增加 checksum manifest

**文件：** `.github/workflows/release.yml`

**实现：**

- 在 `dist/` 生成所有 release asset 后执行 SHA256。
- 输出 `dist/checksums.txt`。
- release upload 继续使用 `dist/*`，确保 checksum manifest 被上传。
- checksum 生成前删除旧 `checksums.txt`，并只匹配 `web-tools-*` asset，避免 manifest 把自己算进去。
- checksum 生成命令需要兼容 Linux/macOS：优先使用 `sha256sum`，不存在时 fallback 到 `shasum -a 256`。

**验证：**

- 本地模拟：

```bash
mkdir -p /tmp/web-tools-dist
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=v0.0.0-test" -o /tmp/web-tools-dist/web-tools-darwin-arm64 .
(cd /tmp/web-tools-dist && rm -f checksums.txt && for f in web-tools-*; do if command -v sha256sum >/dev/null 2>&1; then sha256sum "$f"; else shasum -a 256 "$f"; fi; done > checksums.txt)
grep web-tools-darwin-arm64 /tmp/web-tools-dist/checksums.txt
```

> 后续任务编号如果实现时调整，以依赖关系为准。

#### Task 4：新增 `cmd/upgrade`

**文件：** `cmd/upgrade/main.go`、`main.go`

**实现：**

- 注册 `web-tools upgrade`。
- 支持 `--version`、`--repo`、`--base-url`、`--bin`、`--bin-dir`、`--skill-dir`、`--skill-source`、`--skip-skill`、`--only-skill`、`--check`、`--force`、`--json`。
- 校验互斥参数和 `--base-url --version latest` 等非法组合。
- 输出 human 与 JSON 两种格式。

**验证：**

- `go test ./cmd/upgrade`
- `web-tools upgrade --help`

#### Task 5：实现 release 解析与下载

**文件：** `cmd/upgrade/main.go` 或 `internal/upgrade/*`

**实现：**

- 根据 `runtime.GOOS/GOARCH` 生成 asset 名。
- 支持 latest 与指定 tag。
- 下载 release asset 到临时文件。
- 下载并解析 `checksums.txt`。
- 校验 HTTP 状态码、大小、SHA256、目标版本。
- 明确旧 tag 缺少 checksum 时默认失败，并给出建议。

**验证：**

- 单元测试：平台 asset 映射。
- 单元测试：latest/tag URL 生成。
- 单元测试：checksum manifest 解析。
- 单元测试：checksum mismatch 必须失败。
- httptest：模拟 release asset 下载。
- 错误测试：404、网络失败、unsupported platform。

#### Task 6：实现本地 binary 替换与回滚

**文件：** `internal/upgrade/*`

**实现：**

- 定位当前 binary 或使用 `--bin`。
- 识别 symlink、不确定路径、目标目录权限。
- 下载到同目录临时文件。
- 校验 `--version`。
- rename 替换并保留备份。
- 失败时恢复备份。

**验证：**

- 单元测试：临时目录替换成功。
- 单元测试：替换失败恢复备份。
- 单元测试：symlink 默认拒绝并提示 `--bin`。
- 单元测试：目标目录无写权限失败且不改原文件。
- 单元测试：版本校验失败不替换。
- 单元测试：下载文件不是可执行文件不替换。
- 集成测试：构造 fake old/new binary 执行升级流程。

### Wave 3：Skill 与集成

#### Task 7：升级后安装同版本 skill

**文件：** `cmd/upgrade/main.go`、`cmd/skill/main.go`、`internal/skillinstall/*`

**实现：**

- 将现有 `cmd/skill` 安装逻辑下沉到 `internal/skillinstall`。
- `cmd/skill` 继续保持原 CLI 行为。
- `cmd/upgrade` 通过 `internal/skillinstall` 安装 `targetVersion` 的 skill。
- 支持 `--skill-source` 覆盖默认 GitHub raw URL。
- `--only-skill` 不下载 binary，只更新 skill。
- `--skip-skill` 只更新 binary。

**验证：**

- 单元测试：`--only-skill` 调用目标版本 source。
- 单元测试：`--skill-source` 本地路径和 HTTP(S) URL。
- 集成测试：临时 skill dir 中写入新 `SKILL.md`。
- 检查 skill 包含目标版本新增内容，例如 `WEB_TOOLS_ENV` 指引。

#### Task 8：补齐跨平台 CI 矩阵

**文件：** `.github/workflows/ci.yml`

**实现：**

- 将 `go vet ./...` 和 `go test ./...` 放入 `ubuntu-latest`、`macos-latest`、`windows-latest` matrix。
- 对文件权限、Unix rename、Windows 自覆盖等平台差异测试使用 build tags 或运行时条件跳过不适用断言。
- 保留 Ubuntu 作为完整 release workflow smoke 的主路径。

**验证：**

- GitHub Actions matrix 能在三类 runner 上运行。
- Windows job 覆盖 asset mapping、URL/checksum、manual_replace_required。
- Unix job 覆盖可写目录替换和回滚。

#### Task 9：修正 GUI Agent Guide 安装命令

**文件：** `internal/gui/guide.go`、`internal/gui/server_test.go`

**实现：**

- 不再输出不存在的 tar.gz asset。
- 新增或优先展示：

```bash
web-tools upgrade
web-tools skill install --force
```

- 若仍展示 curl，使用实际 asset 名称，例如 `web-tools-darwin-arm64`。

**验证：**

- GUI guide 测试断言不包含 `.tar.gz`。
- `go test ./internal/gui`

### Wave 4：验收与发布

#### Task 10：端到端 smoke

**文件：** `scripts/smoke.sh` 或新增 `scripts/upgrade_smoke.sh`、`docs/upgrade-test-acceptance.md`

**实现：**

- 用 httptest 或本地文件 server 模拟 release asset。
- 临时安装旧 binary。
- 执行 `upgrade --version vX.Y.Z --base-url ... --bin ... --skill-dir ... --skill-source ...`。
- 覆盖 `--check --json`，确认不修改文件。
- 覆盖 `--only-skill`，确认不替换 binary。
- 覆盖 checksum mismatch，确认失败且原 binary 保留。

**验证：**

- `go test ./...`
- `go vet ./...`
- `./scripts/smoke.sh`
- upgrade smoke 通过。
- `docs/upgrade-test-acceptance.md` 中的发布前阻塞项全部通过或有明确记录。

#### Task 11：发布记录

**文件：** `CHANGELOG.md`、release docs

**实现：**

- 记录 `web-tools upgrade`、版本一致 skill 更新、GUI guide 修正。
- 发布前确认 release workflow assets 与文档一致。

**验证：**

- `git diff --check`
- tag 后下载 release asset 冷启动验收：

```bash
web-tools --version
web-tools upgrade --check --json
web-tools setup --check --json
```

## 测试矩阵

### 单元测试

| 模块 | 用例 |
|---|---|
| Resolver | latest API 解析、显式 tag、dev current version、unsupported platform |
| Asset mapping | darwin/linux/windows/freebsd 支持矩阵 |
| URL builder | GitHub 默认 URL、`--base-url` URL |
| Checksum | manifest 解析、缺少 asset、hash mismatch、hash match |
| Verifier | 临时 binary `--version` 匹配、不匹配、无法执行 |
| Installer | 正常替换、备份恢复、无写权限、symlink 拒绝、非可执行文件 |
| SkillUpdater | target version 传递、`--only-skill`、`--skip-skill` |
| Output | human 摘要、JSON 字段、错误建议不泄漏 secret |

### 集成测试

- 构造 fake release server：
  - `/releases/download/v9.9.9/web-tools-darwin-arm64`
  - `/releases/download/v9.9.9/checksums.txt`
  - `/repos/koda-claw/web-tools/releases/latest`
- 构造 fake old binary 和 fake new binary：
  - old 输出 `web-tools version v0.0.1`
  - new 输出 `web-tools version v9.9.9`
- 临时 skill dir，确认升级后 `SKILL.md` 来自 target version。
- `upgrade --check --json` 前后比较目标 binary hash，确认不修改。

### 跨平台测试

- Linux/macOS：CI 可覆盖正常替换流程。
- Windows：CI 至少覆盖 asset mapping、URL/checksum、下载和“当前 exe 不可覆盖时返回 manual_replace_required”的逻辑。
- 文件权限测试在 Windows 与 Unix 行为不同，应按 build tags 或条件跳过不适用断言。

### 真实 release 冷启动验收

tag 发布后，用 release asset 重新安装到临时目录：

```bash
tmp_dir="$(mktemp -d)"
curl -L https://github.com/koda-claw/web-tools/releases/download/vX.Y.Z/web-tools-darwin-arm64 -o "$tmp_dir/web-tools"
chmod +x "$tmp_dir/web-tools"
"$tmp_dir/web-tools" --version
"$tmp_dir/web-tools" upgrade --check --json
"$tmp_dir/web-tools" skill install --dir "$tmp_dir/skills" --force --json
```

如果已发布旧版本可用于测试，再执行旧版到新版的真实升级：

```bash
"$tmp_dir/web-tools-old" upgrade --version vX.Y.Z --bin "$tmp_dir/web-tools-old" --skill-dir "$tmp_dir/skills" --json
"$tmp_dir/web-tools-old" --version
```

## 验收标准

本轮完成后必须满足：

- 旧版用户能用一条命令升级到最新 CLI 和 skill。
- Agent 能在 skill 中找到升级路径。
- `web-tools upgrade --check --json` 不修改文件，但能报告 current/target/asset。
- `web-tools upgrade --version vX.Y.Z --json` 能输出结构化结果。
- 升级后 `web-tools --version` 与目标 tag 一致。
- 升级后 `web-tools skill install` 或 upgrade 内置 skill 安装使用同一 tag。
- 失败不破坏原 binary；需要时能恢复备份。
- 不修改 `config.json`、`.env`、cache，不泄漏 secret。
- README、README.zh-CN、Skill、GUI Agent Guide 与实际 release asset 命名一致。
- `docs/upgrade-test-acceptance.md` 中的单元、集成、跨平台、发布前和发布后验收路径可执行。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 覆盖正在运行的 binary 在 Windows 上失败 | 第一阶段给明确降级提示；Unix/macOS 先完整支持。 |
| GitHub API rate limit | 支持直接 latest download URL；指定版本不依赖 API。 |
| 用户安装在无写权限目录 | 返回权限错误，并建议 `--bin-dir "$HOME/.local/bin"`。 |
| release asset 名称与 workflow 漂移 | 单测固定 asset 映射；文档从同一映射生成或同步检查。 |
| skill 与 CLI 版本不一致 | upgrade 默认按目标版本安装 skill；`setup --check` 报告 skill 状态。 |
| dev build 行为不明确 | `dev` 默认升级 latest；本地源码安装仍用 `scripts/install.sh`。 |

## 建议版本

建议作为 `v1.6.0` 发布。理由：

- 新增一等 CLI 命令 `web-tools upgrade`。
- 改善 Agent 安装/升级闭环。
- 涉及 release asset、文档、GUI guide、skill 的跨模块行为。

如果只先修文档和 GUI guide，不实现命令，则可作为 `v1.5.3` patch；但从可用性闭环看，建议直接推进 `v1.6.0`。
