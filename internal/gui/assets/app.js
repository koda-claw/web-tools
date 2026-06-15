const state = {
  status: null,
  guide: null,
  lang: detectLanguage(),
};

const $ = (id) => document.getElementById(id);

const messages = {
  en: {
    appTitle: "Local Console",
    languageLabel: "Language",
    setupStatus: "Setup Status",
    refresh: "Refresh",
    provider: "Provider",
    pending: "pending",
    authEnv: "Auth env",
    searchAuto: "Search auto",
    readerFallback: "Reader fallback",
    saveProvider: "Save Provider",
    envFile: "Env File",
    key: "Key",
    value: "Value",
    overwrite: "Overwrite",
    saveEnv: "Save Env",
    searchTest: "Search Test",
    query: "Query",
    runSearch: "Run Search",
    readerTest: "Reader Test",
    runReader: "Run Reader",
    agentGuide: "Agent Guide",
    copy: "Copy",
    copied: "Copied",
    diagnostics: "Diagnostics",
    exportJson: "Export JSON",
    version: "version",
    skillInstalled: "skill installed",
    skillMissing: "skill missing",
    authReady: "auth ready",
    authMissing: "auth missing",
    readerAutoOn: "reader auto on",
    readerAutoOff: "reader auto off",
    configured: "configured",
    missing: "missing",
    repository: "Repository",
    installCLI: "Install CLI",
    installSkill: "Install or update skill",
    check: "Check",
    usage: "Usage",
    readerAuto: "Reader auto",
    repair: "Repair",
  },
  zh: {
    appTitle: "本地控制台",
    languageLabel: "语言",
    setupStatus: "设置状态",
    refresh: "刷新",
    provider: "Provider",
    pending: "待检查",
    authEnv: "认证环境变量",
    searchAuto: "搜索自动链",
    readerFallback: "读取 fallback",
    saveProvider: "保存 Provider",
    envFile: "Env 文件",
    key: "键",
    value: "值",
    overwrite: "覆盖已有值",
    saveEnv: "保存 Env",
    searchTest: "搜索测试",
    query: "查询",
    runSearch: "运行搜索",
    readerTest: "读取测试",
    runReader: "运行读取",
    agentGuide: "Agent 指引",
    copy: "复制",
    copied: "已复制",
    diagnostics: "诊断",
    exportJson: "导出 JSON",
    version: "版本",
    skillInstalled: "skill 已安装",
    skillMissing: "skill 缺失",
    authReady: "认证可用",
    authMissing: "认证缺失",
    readerAutoOn: "reader auto 已启用",
    readerAutoOff: "reader auto 未启用",
    configured: "已配置",
    missing: "缺失",
    repository: "仓库",
    installCLI: "安装 CLI",
    installSkill: "安装或更新 skill",
    check: "检查",
    usage: "使用示例",
    readerAuto: "Reader auto",
    repair: "修复建议",
  },
};

function t(key) {
  return messages[state.lang][key] || messages.en[key] || key;
}

function detectLanguage() {
  const saved = localStorage.getItem("web-tools-language");
  if (saved === "zh" || saved === "en") return saved;
  const langs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || "en"];
  return langs.some((lang) => String(lang).toLowerCase().startsWith("zh")) ? "zh" : "en";
}

function applyLanguage() {
  document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";
  document.title = state.lang === "zh" ? "web-tools 控制台" : "web-tools console";
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  $("refresh").title = t("refresh");
  const selector = $("language-select");
  selector.value = state.lang;
}

async function api(path, options = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const data = await res.json();
  if (!res.ok || data.ok === false) {
    const err = data.error || { message: "request failed", detail: res.statusText };
    throw new Error(`${err.category || "error"}: ${err.message || ""} ${err.detail || ""}`.trim());
  }
  return data;
}

async function refresh() {
  const [status, guide] = await Promise.all([api("/api/status"), api("/api/agent-guide")]);
  state.status = status;
  state.guide = guide.guide;
  renderStatus(status);
  renderGuide(guide.guide);
}

function renderStatus(data) {
  const setup = data.setup;
  $("summary").innerHTML = "";
  $("summary").append(
    pill(`${t("version")} ${data.version || "dev"}`, "ok"),
    pill(setup.skill.installed ? t("skillInstalled") : t("skillMissing"), setup.skill.installed ? "ok" : "warn"),
    pill(setup.provider.auth_configured ? t("authReady") : t("authMissing"), setup.provider.auth_configured ? "ok" : "warn"),
    pill(setup.reader_auto.contains ? t("readerAutoOn") : t("readerAutoOff"), setup.reader_auto.contains ? "ok" : "warn"),
  );

  $("provider-pill").textContent = setup.provider.configured ? t("configured") : t("missing");
  $("provider-pill").className = `pill ${setup.provider.configured ? "ok" : "warn"}`;
  $("env-pill").textContent = setup.env_file.user_exists ? setup.env_file.user_permission : t("missing");
  $("env-pill").className = `pill ${setup.env_file.user_permission === "ok" ? "ok" : "warn"}`;

  $("checks").innerHTML = setup.checks
    .map((check) => {
      const detail = check.details
        ? Object.entries(check.details).map(([k, v]) => `${k}: ${v}`).join("\n")
        : "";
      return `<div class="check ${check.status}">
        <strong>${escapeHTML(check.name)} · ${escapeHTML(check.status)}</strong>
        <p>${escapeHTML(check.message)}</p>
        <p>${escapeHTML(detail)}</p>
      </div>`;
    })
    .join("");
}

function renderGuide(guide) {
  const lines = [
    `${t("repository")}: ${guide.repository_url}`,
    "",
    `${t("installCLI")}:`,
    ...guide.install_cli.map((line) => `  ${line}`),
    "",
    `${t("installSkill")}:`,
    ...guide.install_skill.map((line) => `  ${line}`),
    "",
    `${t("check")}:`,
    ...guide.check_commands.map((line) => `  ${line}`),
    "",
    `${t("usage")}:`,
    ...guide.usage_examples.map((line) => `  ${line}`),
  ];
  if (guide.reader_auto_note) {
    lines.push("", `${t("readerAuto")}: ${guide.reader_auto_note}`);
  }
  if (guide.repair_commands && guide.repair_commands.length) {
    lines.push("", `${t("repair")}:`);
    guide.repair_commands.forEach((item) => {
      lines.push(`  # ${item.message}`, `  ${item.command}`);
    });
  }
  lines.push("", guide.recommended_mode);
  $("agent-guide").textContent = lines.join("\n");
}

function pill(text, status) {
  const span = document.createElement("span");
  span.className = `pill ${status}`;
  span.textContent = text;
  return span;
}

function formJSON(form) {
  const data = new FormData(form);
  const out = {};
  for (const [key, value] of data.entries()) {
    out[key] = value;
  }
  form.querySelectorAll("input[type='checkbox']").forEach((input) => {
    out[input.name] = input.checked;
  });
  return out;
}

async function submitForm(form, path, transform = (v) => v) {
  const button = form.querySelector("button[type='submit']");
  button.disabled = true;
  try {
    const data = await api(path, {
      method: "POST",
      body: JSON.stringify(transform(formJSON(form))),
    });
    $("output").textContent = JSON.stringify(redact(data), null, 2);
    await refresh();
  } catch (err) {
    $("output").textContent = err.message;
  } finally {
    button.disabled = false;
  }
}

function redact(value) {
  return JSON.parse(JSON.stringify(value, (key, val) => {
    if (key === "value" || key === "token" || key === "api_key") return "<redacted>";
    return val;
  }));
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

$("refresh").addEventListener("click", refresh);
$("language-select").addEventListener("change", (event) => {
  state.lang = event.target.value === "zh" ? "zh" : "en";
  localStorage.setItem("web-tools-language", state.lang);
  applyLanguage();
  if (state.status) renderStatus(state.status);
  if (state.guide) renderGuide(state.guide);
});
$("provider-form").addEventListener("submit", (event) => {
  event.preventDefault();
  submitForm(event.currentTarget, "/api/setup/provider");
});
$("env-form").addEventListener("submit", (event) => {
  event.preventDefault();
  submitForm(event.currentTarget, "/api/env");
});
$("search-form").addEventListener("submit", (event) => {
  event.preventDefault();
  submitForm(event.currentTarget, "/api/test/search", (data) => ({ ...data, limit: 3 }));
});
$("reader-form").addEventListener("submit", (event) => {
  event.preventDefault();
  submitForm(event.currentTarget, "/api/test/reader");
});
$("copy-guide").addEventListener("click", async () => {
  await navigator.clipboard.writeText($("agent-guide").textContent);
  const button = $("copy-guide");
  button.textContent = t("copied");
  setTimeout(() => {
    button.textContent = t("copy");
  }, 1200);
});
$("download-diagnostics").addEventListener("click", async () => {
  const data = await api("/api/diagnostics");
  const blob = new Blob([JSON.stringify(redact(data), null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "web-tools-diagnostics.json";
  a.click();
  URL.revokeObjectURL(url);
});

applyLanguage();
refresh().catch((err) => {
  $("output").textContent = err.message;
});
