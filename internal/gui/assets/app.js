const $ = (id) => document.getElementById(id);
const UI_STATE_KEY = "web-tools-gui-state-v1";
const THEME_KEY = "web-tools-theme";
const METRICS_RANGE_KEY = "web-tools-metrics-range";

const state = {
  status: null,
  guide: null,
  metrics: null,
  metricsRange: localStorage.getItem(METRICS_RANGE_KEY) || "24h",
  lang: detectLanguage(),
  theme: detectTheme(),
  searchResult: null,
  readerResult: null,
  charts: {},
};

const messages = {
  en: {
    appTitle: "Local Console",
    languageLabel: "Language",
    themeLabel: "Theme",
    themeSystem: "System",
    themeLight: "Light",
    themeDark: "Dark",
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
    metricsDashboard: "Metrics Dashboard",
    metricsRange: "Range",
    resetMetrics: "Reset Metrics",
    resetMetricsConfirm: "Reset local metrics?",
    commandsChart: "Commands",
    providersChart: "Providers",
    readerQualityChart: "Reader Quality",
    durationChart: "Recent Duration",
    total: "Total",
    success: "Success",
    error: "Error",
    avgDuration: "Avg duration",
    fallback: "Fallback",
    exportJson: "Export JSON",
    clearState: "Clear State",
    read: "Read",
    open: "Open",
    copyContent: "Copy content",
    viewText: "Text",
    viewMarkdown: "Markdown",
    preview: "Preview",
    fullContent: "Full",
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
    themeLabel: "主题",
    themeSystem: "跟随系统",
    themeLight: "浅色",
    themeDark: "深色",
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
    metricsDashboard: "指标看板",
    metricsRange: "时间范围",
    resetMetrics: "重置指标",
    resetMetricsConfirm: "确定重置本地指标？",
    commandsChart: "命令",
    providersChart: "Provider",
    readerQualityChart: "读取质量",
    durationChart: "最近耗时",
    total: "总数",
    success: "成功",
    error: "失败",
    avgDuration: "平均耗时",
    fallback: "Fallback",
    exportJson: "导出 JSON",
    clearState: "清空状态",
    read: "读取",
    open: "打开",
    copyContent: "复制正文",
    viewText: "文本",
    viewMarkdown: "Markdown",
    preview: "预览",
    fullContent: "完整",
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

function detectTheme() {
  const saved = localStorage.getItem(THEME_KEY);
  return saved === "light" || saved === "dark" || saved === "system" ? saved : "system";
}

function resolvedTheme() {
  if (state.theme === "light" || state.theme === "dark") return state.theme;
  return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyTheme() {
  document.documentElement.dataset.theme = resolvedTheme();
  const selector = $("theme-select");
  if (selector) selector.value = state.theme;
  renderMetrics();
}

function applyLanguage() {
  document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";
  document.title = state.lang === "zh" ? "web-tools 控制台" : "web-tools console";
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  $("refresh").title = t("refresh");
  $("reset-metrics").textContent = t("resetMetrics");
  const selector = $("language-select");
  selector.value = state.lang;
  renderMetrics();
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
  const [status, guide, metricsData] = await Promise.all([
    api("/api/status"),
    api("/api/agent-guide"),
    loadMetrics(),
  ]);
  state.status = status;
  state.guide = guide.guide;
  state.metrics = metricsData.metrics;
  renderStatus(status);
  renderGuide(guide.guide);
  updateReaderProviderOptions(status.setup);
  renderMetrics();
}

async function loadMetrics() {
  const range = encodeURIComponent(state.metricsRange || "24h");
  return api(`/api/metrics?range=${range}&bucket=auto`);
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
  $("reader-provider-pill").textContent = setup.reader_auto.contains ? "auto+" : "builtin";

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

function updateReaderProviderOptions(setup) {
  const select = $("reader-form").elements.provider;
  const bigmodel = [...select.options].find((option) => option.value === "bigmodel");
  if (!bigmodel) return;
  const ready = Boolean(setup.provider.configured && setup.provider.auth_configured);
  bigmodel.disabled = !ready;
  bigmodel.textContent = ready ? "bigmodel" : "bigmodel (auth required)";
  if (select.value === "bigmodel" && !ready) {
    select.value = "builtin-reader";
  }
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
  const textPreview = previewText(content, 1200);
  const fullAvailable = content.length > textPreview.length;
  container.innerHTML = `
    <div class="reader-card">
      <div class="result-head">
        <strong>${escapeHTML(result.title || result.source || t("readerPreview"))}</strong>
        <div class="button-row compact">
          <button class="secondary-button view-mode active" type="button" data-view-mode="markdown">${escapeHTML(t("viewMarkdown"))}</button>
          <button class="secondary-button view-mode" type="button" data-view-mode="text">${escapeHTML(t("viewText"))}</button>
          <button class="secondary-button content-size active" type="button" data-content-size="preview">${escapeHTML(t("preview"))}</button>
          <button class="secondary-button content-size" type="button" data-content-size="full" ${fullAvailable ? "" : "disabled"}>${escapeHTML(t("fullContent"))}</button>
          <button class="secondary-button" id="copy-reader-content" type="button">${escapeHTML(t("copyContent"))}</button>
        </div>
      </div>
      <div class="meta-row wrap">
        <span>${escapeHTML(t("source"))}: ${escapeHTML(result.source || result.url || "")}</span>
        <span>${escapeHTML(t("provider"))}: ${escapeHTML(result.provider || "")}</span>
        <span>${escapeHTML(t("words"))}: ${escapeHTML(result.word_count || 0)}</span>
        <span>${escapeHTML(t("contentType"))}: ${escapeHTML(result.content_type || "")}</span>
        <span>${escapeHTML(t("extractMode"))}: ${escapeHTML(result.extract_mode || "")}</span>
      </div>
      <div class="reader-preview markdown-view" id="reader-preview"></div>
    </div>
  `;
  const preview = $("reader-preview");
  const renderContent = () => {
    const mode = container.querySelector(".view-mode.active")?.dataset.viewMode || "markdown";
    const size = container.querySelector(".content-size.active")?.dataset.contentSize || "preview";
    const visibleContent = size === "full" ? content : textPreview;
    preview.className = `reader-preview ${mode === "markdown" ? "markdown-view" : ""}`;
    preview.dataset.contentSize = size;
    if (mode === "markdown") {
      preview.innerHTML = renderMarkdown(visibleContent);
    } else {
      preview.textContent = visibleContent;
    }
  };
  container.querySelectorAll(".view-mode").forEach((button) => {
    button.addEventListener("click", () => {
      container.querySelectorAll(".view-mode").forEach((item) => item.classList.remove("active"));
      button.classList.add("active");
      renderContent();
    });
  });
  container.querySelectorAll(".content-size").forEach((button) => {
    button.addEventListener("click", () => {
      if (button.disabled) return;
      container.querySelectorAll(".content-size").forEach((item) => item.classList.remove("active"));
      button.classList.add("active");
      renderContent();
    });
  });
  const copy = $("copy-reader-content");
  copy.addEventListener("click", async () => {
    await navigator.clipboard.writeText(content);
    copy.textContent = t("copied");
    setTimeout(() => {
      copy.textContent = t("copyContent");
    }, 1200);
  });
  renderContent();
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

function renderMetrics() {
  const snap = state.metrics;
  syncMetricsRangeControl();
  const cards = $("metric-cards");
  if (!cards) return;
  const commands = snap && snap.commands ? snap.commands : {};
  const providers = snap && snap.providers ? snap.providers : {};
  const errors = snap && snap.errors ? snap.errors : {};
  const quality = snap && snap.reader_quality ? snap.reader_quality : {};
  const total = sumCounters(commands, "total");
  const success = sumCounters(commands, "success");
  const error = sumCounters(commands, "error");
  const avg = averageDuration(commands);
  cards.innerHTML = [
    metricCard(t("total"), total),
    metricCard(t("success"), success),
    metricCard(t("error"), error),
    metricCard(t("avgDuration"), `${avg}ms`),
    metricCard(t("fallback"), quality.fallback_recommended || 0),
  ].join("");

  drawBarChart("commands-chart", Object.entries(commands).map(([name, c]) => ({
    name,
    success: c.success || 0,
    error: c.error || 0,
  })));
  drawBarChart("providers-chart", Object.entries(providers).map(([name, c]) => ({
    name,
    success: c.success || 0,
    error: c.error || 0,
  })));
  drawDonutChart("quality-chart", [
    { name: "high", value: quality.high || 0 },
    { name: "medium", value: quality.medium || 0 },
    { name: "low", value: quality.low || 0 },
  ]);
  drawLineChart("duration-chart", (snap && snap.recent_events ? snap.recent_events : []).map((event, index) => ({
    name: event.command || String(index + 1),
    value: event.duration_ms || 0,
  })));
  $("output").dataset.metricsErrors = Object.keys(errors).join(",");
}

function syncMetricsRangeControl() {
  const selected = state.metricsRange || "24h";
  document.querySelectorAll('input[name="metrics_range"]').forEach((input) => {
    input.checked = input.value === selected;
    input.closest("label")?.classList.toggle("active", input.checked);
  });
}

function metricCard(label, value) {
  return `<div class="metric-card"><span>${escapeHTML(label)}</span><strong>${escapeHTML(value)}</strong></div>`;
}

function sumCounters(counters, field) {
  return Object.values(counters || {}).reduce((sum, item) => sum + Number(item[field] || 0), 0);
}

function averageDuration(counters) {
  let total = 0;
  let duration = 0;
  Object.values(counters || {}).forEach((item) => {
    total += Number(item.total || 0);
    duration += Number(item.total_duration_ms || 0);
  });
  return total ? Math.round(duration / total) : 0;
}

function chartPalette() {
  return resolvedTheme() === "dark"
    ? ["#58c58b", "#ff7b72", "#73d0d8", "#d89b3d"]
    : ["#197447", "#b42318", "#0b6670", "#9b5800"];
}

function drawBarChart(id, rows) {
  const el = $(id);
  if (!el) return;
  const labels = rows.map((row) => row.name);
  const colors = chartPalette();
  if (window.echarts) {
    const chart = state.charts[id] || window.echarts.init(el);
    state.charts[id] = chart;
    chart.setOption({
      color: colors,
      tooltip: { trigger: "axis" },
      legend: { textStyle: { color: "var(--muted)" } },
      grid: { left: 36, right: 12, top: 28, bottom: 36 },
      xAxis: { type: "category", data: labels, axisLabel: { color: getCSSVar("--muted"), interval: 0, rotate: labels.length > 3 ? 20 : 0 } },
      yAxis: { type: "value", axisLabel: { color: getCSSVar("--muted") }, splitLine: { lineStyle: { color: getCSSVar("--line") } } },
      series: [
        { name: t("success"), type: "bar", stack: "total", data: rows.map((row) => row.success) },
        { name: t("error"), type: "bar", stack: "total", data: rows.map((row) => row.error) },
      ],
    });
    return;
  }
  fallbackBars(el, rows);
}

function drawDonutChart(id, rows) {
  const el = $(id);
  if (!el) return;
  const colors = chartPalette();
  if (window.echarts) {
    const chart = state.charts[id] || window.echarts.init(el);
    state.charts[id] = chart;
    chart.setOption({
      color: colors,
      tooltip: { trigger: "item" },
      legend: { bottom: 0, textStyle: { color: getCSSVar("--muted") } },
      series: [{ type: "pie", radius: ["46%", "70%"], center: ["50%", "42%"], data: rows }],
    });
    return;
  }
  fallbackBars(el, rows.map((row) => ({ name: row.name, success: row.value, error: 0 })));
}

function drawLineChart(id, rows) {
  const el = $(id);
  if (!el) return;
  if (window.echarts) {
    const chart = state.charts[id] || window.echarts.init(el);
    state.charts[id] = chart;
    chart.setOption({
      color: chartPalette(),
      tooltip: { trigger: "axis" },
      grid: { left: 36, right: 12, top: 20, bottom: 36 },
      xAxis: { type: "category", data: rows.map((row, index) => `${index + 1}`), axisLabel: { color: getCSSVar("--muted") } },
      yAxis: { type: "value", axisLabel: { color: getCSSVar("--muted") }, splitLine: { lineStyle: { color: getCSSVar("--line") } } },
      series: [{ name: "ms", type: "line", smooth: true, data: rows.map((row) => row.value), areaStyle: {} }],
    });
    return;
  }
  fallbackBars(el, rows.map((row) => ({ name: row.name, success: row.value, error: 0 })));
}

function fallbackBars(el, rows) {
  const max = Math.max(1, ...rows.map((row) => Number(row.success || row.value || 0) + Number(row.error || 0)));
  el.innerHTML = `<div class="fallback-chart">${rows.length ? rows.map((row) => {
    const ok = Number(row.success || row.value || 0);
    const err = Number(row.error || 0);
    return `<div class="fallback-row">
      <span>${escapeHTML(row.name)}</span>
      <div class="fallback-track">
        <i style="width:${Math.round((ok / max) * 100)}%"></i>
        <b style="width:${Math.round((err / max) * 100)}%"></b>
      </div>
    </div>`;
  }).join("") : `<div class="empty-state">${escapeHTML(t("noResults"))}</div>`}</div>`;
}

function getCSSVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
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
    content: previewText(content, 1200),
    word_count: result.word_count,
    content_type: result.content_type,
    extract_mode: result.extract_mode,
    provider: result.provider,
  };
}

function previewText(value, limit) {
  const text = String(value || "").trim();
  if (text.length <= limit) return text;
  return `${text.slice(0, limit).trim()}\n\n...`;
}

function renderMarkdown(markdown) {
  const lines = String(markdown || "").split(/\r?\n/);
  const html = [];
  let inCode = false;
  let paragraph = [];
  const flushParagraph = () => {
    if (!paragraph.length) return;
    html.push(`<p>${inlineMarkdown(paragraph.join(" "))}</p>`);
    paragraph = [];
  };
  for (const line of lines) {
    if (line.trim().startsWith("```")) {
      if (inCode) {
        html.push("</code></pre>");
        inCode = false;
      } else {
        flushParagraph();
        html.push("<pre><code>");
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      html.push(escapeHTML(line) + "\n");
      continue;
    }
    const trimmed = line.trim();
    if (!trimmed) {
      flushParagraph();
      continue;
    }
    const heading = trimmed.match(/^(#{1,3})\s+(.+)$/);
    if (heading) {
      flushParagraph();
      const level = heading[1].length + 2;
      html.push(`<h${level}>${inlineMarkdown(heading[2])}</h${level}>`);
      continue;
    }
    if (/^[-*]\s+/.test(trimmed)) {
      flushParagraph();
      html.push(`<p class="md-list">• ${inlineMarkdown(trimmed.replace(/^[-*]\s+/, ""))}</p>`);
      continue;
    }
    if (trimmed.startsWith(">")) {
      flushParagraph();
      html.push(`<blockquote>${inlineMarkdown(trimmed.replace(/^>\s?/, ""))}</blockquote>`);
      continue;
    }
    paragraph.push(trimmed);
  }
  flushParagraph();
  if (inCode) {
    html.push("</code></pre>");
  }
  return html.join("");
}

function inlineMarkdown(value) {
  return escapeHTML(value)
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>');
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
$("theme-select").addEventListener("change", (event) => {
  const value = event.target.value;
  state.theme = value === "light" || value === "dark" ? value : "system";
  localStorage.setItem(THEME_KEY, state.theme);
  applyTheme();
});
window.addEventListener("resize", () => {
  Object.values(state.charts).forEach((chart) => chart && chart.resize && chart.resize());
});
if (window.matchMedia) {
  const colorSchemeQuery = window.matchMedia("(prefers-color-scheme: dark)");
  const onColorSchemeChange = () => {
    if (state.theme === "system") applyTheme();
  };
  if (colorSchemeQuery.addEventListener) {
    colorSchemeQuery.addEventListener("change", onColorSchemeChange);
  } else if (colorSchemeQuery.addListener) {
    colorSchemeQuery.addListener(onColorSchemeChange);
  }
}
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
$("metrics-range").addEventListener("change", async (event) => {
  const input = event.target.closest('input[name="metrics_range"]');
  if (!input) return;
  state.metricsRange = input.value || "24h";
  localStorage.setItem(METRICS_RANGE_KEY, state.metricsRange);
  syncMetricsRangeControl();
  try {
    const data = await loadMetrics();
    state.metrics = data.metrics;
    renderMetrics();
  } catch (err) {
    $("output").textContent = err.message;
  }
});
$("reset-metrics").addEventListener("click", async () => {
  if (!window.confirm(t("resetMetricsConfirm"))) return;
  try {
    await api("/api/metrics/reset", { method: "POST", body: "{}" });
    const data = await loadMetrics();
    state.metrics = data.metrics;
    renderMetrics();
  } catch (err) {
    $("output").textContent = err.message;
  }
});

applyTheme();
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
