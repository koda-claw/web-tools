const state = {
  status: null,
  guide: null,
};

const $ = (id) => document.getElementById(id);

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
    pill(`version ${data.version || "dev"}`, "ok"),
    pill(setup.skill.installed ? "skill installed" : "skill missing", setup.skill.installed ? "ok" : "warn"),
    pill(setup.provider.auth_configured ? "auth ready" : "auth missing", setup.provider.auth_configured ? "ok" : "warn"),
    pill(setup.reader_auto.contains ? "reader auto on" : "reader auto off", setup.reader_auto.contains ? "ok" : "warn"),
  );

  $("provider-pill").textContent = setup.provider.configured ? "configured" : "missing";
  $("provider-pill").className = `pill ${setup.provider.configured ? "ok" : "warn"}`;
  $("env-pill").textContent = setup.env_file.user_exists ? setup.env_file.user_permission : "missing";
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
    `Repository: ${guide.repository_url}`,
    "",
    "Install CLI:",
    ...guide.install_cli.map((line) => `  ${line}`),
    "",
    "Install or update skill:",
    ...guide.install_skill.map((line) => `  ${line}`),
    "",
    "Check:",
    ...guide.check_commands.map((line) => `  ${line}`),
    "",
    "Usage:",
    ...guide.usage_examples.map((line) => `  ${line}`),
  ];
  if (guide.reader_auto_note) {
    lines.push("", `Reader auto: ${guide.reader_auto_note}`);
  }
  if (guide.repair_commands && guide.repair_commands.length) {
    lines.push("", "Repair:");
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

refresh().catch((err) => {
  $("output").textContent = err.message;
});
