const state = {
  status: null,
  guide: null,
  lang: detectLanguage(),
  searchResult: null,
  readerResult: null,
};

const $ = (id) => document.getElementById(id);
const UI_STATE_KEY = "web-tools-gui-state-v1";

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
    clearState: "Clear State",
    read: "Read",
    open: "Open",
    copyContent: "Copy content",
    noResults: "No results",
    searchResults: "Search results",
    readerPreview: "Reader preview",
    words: "words",
    source: "source",
    engine: "engine",
    contentType: "content type",
    extractMode: "extract mode",
    lastError: "Last error",
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
    clearState: "清空状态",
    read: "读取",
    open: "打开",
    copyContent: "复制正文",
    noResults: "没有结果",
    searchResults: "搜索结果",
    readerPreview: "读取预览",
    words: "词",
    source: "来源",
    engine: "引擎",
    contentType: "内容类型",
    extractMode: "提取模式",
    lastError: "最近错误",
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

function loadUIState() {
  try {
    const saved = JSON.parse(localStorage.getItem(UI_STATE_KEY) || "{}");
    if (saved.searchForm) {
      setFormValues($("search-form"), saved.searchForm);
    }
    if (saved.readerForm) {
      setFormValues($("reader-form"), saved.readerForm);
    }
    state.searchResult = saved.searchResult || null;
    state.readerResult = saved.readerResult || null;
  } catch {
    state.searchResult = null;
    state.readerResult = null;
  }
}

function saveUIState() {
  const payload = {
    searchForm: formJSON($("search-form")),
    readerForm: formJSON($("reader-form")),
    searchResult: state.searchResult,
    readerResult: state.readerResult,
  };
  localStorage.setItem(UI_STATE_KEY, JSON.stringify(payload));
}

function clearUIState() {
  localStorage.removeItem(UI_STATE_KEY);
  state.searchResult = null;
  state.readerResult = null;
  $("search-results").innerHTML = "";
  $("reader-result").innerHTML = "";
  $("output").textContent = "";
}

function setFormValues(form, values) {
  Object.entries(values || {}).forEach(([key, value]) => {
    if (key === "value") return;
    const field = form.elements[key];
    if (!field) return;
    if (field.type === "checkbox") {
      field.checked = Boolean(value);
    } else {
      field.value = value;
    }
  });
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

function renderSearchResults(data) {
  const result = data && data.result ? data.result : data;
  const results = result && Array.isArray(result.results) ? result.results : [];
  const container = $("search-results");
  if (!results.length) {
    container.innerHTML = `<div class="empty-state">${escapeHTML(t("noResults"))}</div>`;
    return;
  }
  container.innerHTML = `
    <div class="result-head">
      <strong>${escapeHTML(t("searchResults"))}</strong>
      <span>${escapeHTML(result.engine || result.provider || "")} · ${results.length}</span>
    </div>
    <div class="search-list">
      ${results.map((item) => renderSearchItem(item, result)).join("")}
    </div>
  `;
}

function renderSearchItem(item, result) {
  const url = item.url || "";
  const engines = Array.isArray(item.engines) ? item.engines.join(", ") : result.engine || "";
  return `
    <div class="search-item">
      <div class="result-rank">${escapeHTML(item.rank || "")}</div>
      <div class="result-body">
        <a class="result-title" href="${escapeAttr(url)}" target="_blank" rel="noreferrer">${escapeHTML(item.title || url)}</a>
        <div class="result-url">${escapeHTML(url)}</div>
        <p>${escapeHTML(item.snippet || "")}</p>
        <div class="meta-row">
          <span>${escapeHTML(t("source"))}: ${escapeHTML(item.source || "")}</span>
          <span>${escapeHTML(t("engine"))}: ${escapeHTML(engines)}</span>
        </div>
      </div>
      <button class="secondary-button read-result" type="button" data-url="${escapeAttr(url)}">${escapeHTML(t("read"))}</button>
    </div>
  `;
}

function renderReaderResult(data) {
  const result = data && data.result ? data.result : data;
  const container = $("reader-result");
  if (!result) {
    container.innerHTML = "";
    return;
  }
  const content = result.content || result.text_content || "";
  const preview = previewText(content, 1200);
  container.innerHTML = `
    <div class="reader-card">
      <div class="result-head">
        <strong>${escapeHTML(result.title || result.source || t("readerPreview"))}</strong>
        <button class="secondary-button" id="copy-reader-content" type="button">${escapeHTML(t("copyContent"))}</button>
      </div>
      <div class="meta-row wrap">
        <span>${escapeHTML(t("source"))}: ${escapeHTML(result.source || result.url || "")}</span>
        <span>${escapeHTML(t("words"))}: ${escapeHTML(result.word_count || 0)}</span>
        <span>${escapeHTML(t("contentType"))}: ${escapeHTML(result.content_type || "")}</span>
        <span>${escapeHTML(t("extractMode"))}: ${escapeHTML(result.extract_mode || "")}</span>
      </div>
      <div class="reader-preview">${escapeHTML(preview)}</div>
    </div>
  `;
  const copy = $("copy-reader-content");
  copy.addEventListener("click", async () => {
    await navigator.clipboard.writeText(content);
    copy.textContent = t("copied");
    setTimeout(() => {
      copy.textContent = t("copyContent");
    }, 1200);
  });
}

function renderInlineError(targetID, err) {
  $(targetID).innerHTML = `
    <div class="inline-error">
      <strong>${escapeHTML(t("lastError"))}</strong>
      <p>${escapeHTML(err.message || String(err))}</p>
    </div>
  `;
}

function renderPersistedResults() {
  if (state.searchResult) {
    renderSearchResults({ result: state.searchResult });
  }
  if (state.readerResult) {
    renderReaderResult({ result: state.readerResult });
  }
}

function persistSearchResult(data) {
  const result = data.result || data;
  state.searchResult = {
    query: result.query,
    engine: result.engine,
    provider: result.provider,
    total: result.total,
    results: (result.results || []).slice(0, 10).map((item) => ({
      rank: item.rank,
      title: item.title,
      url: item.url,
      snippet: item.snippet,
      source: item.source,
      engines: item.engines,
    })),
  };
}

function persistReaderResult(data) {
  const result = data.result || data;
  const content = result.content || result.text_content || "";
  state.readerResult = {
    source: result.source,
    url: result.url,
    title: result.title,
    content: previewText(content, 4000),
    word_count: result.word_count,
    content_type: result.content_type,
    extract_mode: result.extract_mode,
  };
}

function previewText(value, limit) {
  const text = String(value || "").trim();
  if (text.length <= limit) return text;
  return `${text.slice(0, limit).trim()}\n\n...`;
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

async function submitForm(form, path, transform = (v) => v, onSuccess, onError) {
  const button = form.querySelector("button[type='submit']");
  button.disabled = true;
  try {
    const data = await api(path, {
      method: "POST",
      body: JSON.stringify(transform(formJSON(form))),
    });
    $("output").textContent = JSON.stringify(redact(data), null, 2);
    if (onSuccess) onSuccess(data);
    saveUIState();
    await refresh();
  } catch (err) {
    $("output").textContent = err.message;
    if (onError) onError(err);
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

function escapeAttr(value) {
  return escapeHTML(value).replaceAll("`", "&#096;");
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
  submitSearch(event.currentTarget);
});
$("reader-form").addEventListener("submit", (event) => {
  event.preventDefault();
  submitReader(event.currentTarget);
});
$("search-results").addEventListener("click", (event) => {
  const button = event.target.closest(".read-result");
  if (!button) return;
  const url = button.dataset.url;
  if (!url) return;
  const readerForm = $("reader-form");
  readerForm.elements.url.value = url;
  saveUIState();
  submitReader(readerForm);
});
$("copy-guide").addEventListener("click", async () => {
  await navigator.clipboard.writeText($("agent-guide").textContent);
  const button = $("copy-guide");
  button.textContent = t("copied");
  setTimeout(() => {
    button.textContent = t("copy");
  }, 1200);
});
$("clear-state").addEventListener("click", clearUIState);
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
loadUIState();
renderPersistedResults();
refresh().catch((err) => {
  $("output").textContent = err.message;
});

function submitSearch(form) {
  submitForm(
    form,
    "/api/test/search",
    (data) => ({ ...data, limit: 3 }),
    (data) => {
      persistSearchResult(data);
      renderSearchResults(data);
    },
    (err) => renderInlineError("search-results", err),
  );
}

function submitReader(form) {
  submitForm(
    form,
    "/api/test/reader",
    (data) => data,
    (data) => {
      persistReaderResult(data);
      renderReaderResult(data);
    },
    (err) => renderInlineError("reader-result", err),
  );
}
