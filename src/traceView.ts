import { requestsByStep, type Trace, type TraceServer } from "./traceModel";

// What the page needs from the editor: the webview's source for the image
// policy, a fresh nonce, and the two readers for a step's files. Both readers
// answer undefined for a file that is missing or would leave the trace's
// directory; the page never builds a path itself.
export interface TraceViewOptions {
  readonly cspSource: string;
  readonly nonce: string;
  readonly image: (relative: string) => string | undefined;
  readonly dom: (relative: string) => string | undefined;
}

// traceContentSecurityPolicy is the only policy the page runs under: nothing
// loads unless named here, images come from the trace through the webview, and
// the one style and one script are the ones carrying this page's nonce. No
// inline handler, no eval, no remote origin, no form, no frame.
export function traceContentSecurityPolicy(cspSource: string, nonce: string): string {
  return [
    "default-src 'none'",
    `img-src ${cspSource}`,
    `style-src 'nonce-${nonce}'`,
    `script-src 'nonce-${nonce}'`,
    "base-uri 'none'",
    "form-action 'none'",
  ].join("; ");
}

// renderTraceHtml draws a trace: the steps as a timeline, and for the selected
// step its screenshot, its DOM as source text, the network with the step's
// requests marked, the console, and the server's side -- the log, the SQL and
// the Policy decisions for each request the step caused. Every value that came
// from the trace is escaped; the DOM is shown, never rendered.
export function renderTraceHtml(trace: Trace, options: TraceViewOptions): string {
  const nonce = escapeAttribute(options.nonce);
  const byStep = requestsByStep(trace);
  const stepsOfRequest = new Map<number, number[]>();
  for (const [step, positions] of byStep) {
    for (const position of positions) {
      stepsOfRequest.set(position, [...(stepsOfRequest.get(position) ?? []), step]);
    }
  }
  const stepsOfServer = new Map<string, number[]>();
  trace.network.forEach((request, position) => {
    if (request.serverRequestId === "") {
      return;
    }
    const steps = stepsOfServer.get(request.serverRequestId) ?? [];
    stepsOfServer.set(request.serverRequestId, [...steps, ...(stepsOfRequest.get(position) ?? [])]);
  });
  const failing = trace.steps.find((step) => step.error !== "");
  const selected = failing?.index ?? trace.steps.at(-1)?.index ?? 0;
  const duration = trace.started !== undefined && trace.finished !== undefined
    ? formatDuration(trace.finished - trace.started)
    : "";
  const engine = [trace.driver, trace.browser].filter((part) => part !== "").join(" · ");

  const parts: string[] = [];
  parts.push(
    "<!DOCTYPE html>",
    "<html lang=\"en\">",
    "<head>",
    "<meta charset=\"utf-8\">",
    `<meta http-equiv="Content-Security-Policy" content="${escapeAttribute(traceContentSecurityPolicy(options.cspSource, options.nonce))}">`,
    "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
    `<title>${escapeText(`Trace: ${trace.test}`)}</title>`,
    `<style nonce="${nonce}">${styles}</style>`,
    "</head>",
    `<body data-selected="${selected}">`,
    "<header>",
    `<h1>${escapeText(trace.test || "Unnamed test")}</h1>`,
    `<p class="meta">${escapeText([engine, trace.started === undefined ? "" : new Date(trace.started).toISOString(), duration].filter((part) => part !== "").join(" · "))}</p>`,
    `<span class="badge ${trace.failed ? "failed" : "passed"}">${trace.failed ? "Failed" : "Passed"}</span>`,
    trace.failed ? "<button type=\"button\" data-command=\"explain\">Explain failure</button>" : "",
    "</header>",
  );

  if (trace.failures.length > 0) {
    parts.push("<section class=\"failures\" aria-label=\"Failures\"><h2>Failures</h2><ul>");
    for (const failure of trace.failures) {
      parts.push(`<li><span class="kind">${escapeText(failure.kind)}</span> ${escapeText(failure.text)}</li>`);
    }
    parts.push("</ul></section>");
  }

  parts.push("<main class=\"layout\">", "<nav class=\"steps\" aria-label=\"Steps\">");
  if (trace.steps.length === 0) {
    parts.push("<p class=\"empty\">The test recorded no step.</p>");
  } else {
    parts.push("<ol>");
    for (const step of trace.steps) {
      const classes = ["step", ...(step.error !== "" ? ["error"] : [])].join(" ");
      parts.push(
        `<li><button type="button" class="${classes}" data-step="${step.index}" aria-pressed="${step.index === selected}">`,
        `<span class="index">${step.index}</span>`,
        `<span class="action">${escapeText(step.action)}</span>`,
        step.target === "" ? "" : `<span class="target">${escapeText(step.target)}</span>`,
        `<span class="duration">${escapeText(formatDuration(step.durationMs))}</span>`,
        "</button></li>",
      );
    }
    parts.push("</ol>");
  }
  parts.push("</nav>", "<div class=\"detail\">");

  const tabs: Array<[string, string]> = [
    ["screenshot", "Screenshot"],
    ["dom", "DOM"],
    ["network", `Network (${trace.network.length})`],
    ["console", `Console (${trace.console.length})`],
    ["server", `Server (${trace.server.size})`],
  ];
  parts.push("<div class=\"tabs\" role=\"tablist\">");
  tabs.forEach(([id, label], position) => {
    parts.push(`<button type="button" role="tab" data-tab="${id}" aria-selected="${position === 0}">${escapeText(label)}</button>`);
  });
  parts.push("</div>");

  // Screenshot, with the step's address and error above it.
  parts.push("<section role=\"tabpanel\" data-panel=\"screenshot\">");
  for (const step of trace.steps) {
    parts.push(`<div data-step-panel="${step.index}"${step.index === selected ? "" : " hidden"}>`);
    if (step.url !== "") {
      parts.push(`<p class="url">${escapeText(step.url)}</p>`);
    }
    if (step.error !== "") {
      parts.push(`<pre class="step-error">${escapeText(step.error)}</pre>`);
    }
    const source = step.screenshot === "" ? undefined : options.image(step.screenshot);
    if (source === undefined) {
      parts.push("<p class=\"empty\">No screenshot was taken after this step.</p>");
    } else {
      parts.push(`<img src="${escapeAttribute(source)}" alt="${escapeAttribute(`Screenshot after step ${step.index}: ${step.action}`)}">`);
    }
    parts.push("</div>");
  }
  parts.push("</section>");

  // DOM, as text.
  parts.push("<section role=\"tabpanel\" data-panel=\"dom\" hidden>");
  for (const step of trace.steps) {
    const dom = step.dom === "" ? undefined : options.dom(step.dom);
    parts.push(`<div data-step-panel="${step.index}"${step.index === selected ? "" : " hidden"}>`);
    parts.push(dom === undefined
      ? "<p class=\"empty\">No DOM was captured after this step.</p>"
      : `<pre class="source"><code>${escapeText(dom)}</code></pre>`);
    parts.push("</div>");
  }
  parts.push("</section>");

  // Network.
  parts.push("<section role=\"tabpanel\" data-panel=\"network\" hidden>");
  if (trace.network.length === 0) {
    parts.push("<p class=\"empty\">The page made no request.</p>");
  } else {
    parts.push("<table><thead><tr><th>Method</th><th>URL</th><th>Status</th><th>Type</th><th>Time</th><th>Server</th></tr></thead><tbody>");
    trace.network.forEach((request, position) => {
      const steps = stepsOfRequest.get(position) ?? [];
      const classes = [
        request.status >= 500 ? "status-5xx" : request.status >= 400 ? "status-4xx" : "",
        request.error !== "" ? "request-error" : "",
        steps.includes(selected) ? "current" : "",
      ].filter((name) => name !== "").join(" ");
      const time = request.started !== undefined && request.finished !== undefined
        ? formatDuration(request.finished - request.started)
        : "";
      const server = request.serverRequestId !== "" && trace.server.has(request.serverRequestId)
        ? `<button type="button" data-show-server="${escapeAttribute(request.serverRequestId)}">${escapeText(request.serverRequestId)}</button>`
        : escapeText(request.serverRequestId);
      parts.push(
        `<tr data-steps="${steps.join(" ")}"${classes === "" ? "" : ` class="${classes}"`}>`,
        `<td>${escapeText(request.method)}</td>`,
        `<td class="wrap">${escapeText(request.url)}${request.error === "" ? "" : `<div class="request-error-text">${escapeText(request.error)}</div>`}</td>`,
        `<td>${request.status === 0 ? "" : request.status}</td>`,
        `<td>${escapeText(request.type)}</td>`,
        `<td>${escapeText(time)}</td>`,
        `<td>${server}</td>`,
        "</tr>",
      );
    });
    parts.push("</tbody></table>");
  }
  parts.push("</section>");

  // Console.
  parts.push("<section role=\"tabpanel\" data-panel=\"console\" hidden>");
  if (trace.console.length === 0) {
    parts.push("<p class=\"empty\">The console stayed empty.</p>");
  } else {
    parts.push("<ul class=\"console\">");
    for (const message of trace.console) {
      parts.push(
        `<li class="level-${escapeAttribute(safeClass(message.level))}">`,
        `<span class="level">${escapeText(message.level)}</span> `,
        `<span class="text">${escapeText(message.text)}</span>`,
        message.url === "" ? "" : ` <span class="source-url">${escapeText(message.url)}</span>`,
        "</li>",
      );
    }
    parts.push("</ul>");
  }
  parts.push("</section>");

  // Server.
  parts.push("<section role=\"tabpanel\" data-panel=\"server\" hidden>");
  if (trace.server.size === 0) {
    parts.push("<p class=\"empty\">The trace has no server section: the application was not served through the test kit's server trace.</p>");
  } else {
    parts.push(
      "<p class=\"server-filter\"><span data-server-scope>Requests of the selected step.</span> ",
      "<button type=\"button\" data-server-all>Show every request</button></p>",
      "<p class=\"empty\" data-server-none hidden>The selected step caused no request the server answered.</p>",
    );
    for (const [id, section] of trace.server) {
      const request = trace.network.find((candidate) => candidate.serverRequestId === id);
      const steps = stepsOfServer.get(id) ?? [];
      const heading = request === undefined
        ? id
        : `${request.method} ${request.url}${request.status === 0 ? "" : ` → ${request.status}`}`;
      parts.push(
        `<article data-server="${escapeAttribute(id)}" data-steps="${[...new Set(steps)].join(" ")}"${steps.includes(selected) ? "" : " hidden"}>`,
        `<h3>${escapeText(heading)} <span class="request-id">${escapeText(id)}</span></h3>`,
        renderServer(section),
        "</article>",
      );
    }
  }
  parts.push("</section>");

  parts.push("</div>", "</main>", `<script nonce="${nonce}">${script}</script>`, "</body>", "</html>");
  return parts.join("\n");
}

function renderServer(section: TraceServer): string {
  const parts: string[] = [];
  parts.push("<h4>Policy decisions</h4>");
  if (section.decisions.length === 0) {
    parts.push("<p class=\"empty\">No Policy was asked.</p>");
  } else {
    parts.push("<table><thead><tr><th>Policy</th><th>Action</th><th>Decision</th><th>Grant or reason</th></tr></thead><tbody>");
    for (const decision of section.decisions) {
      parts.push(
        `<tr class="${decision.allowed ? "allowed" : "denied"}">`,
        `<td>${escapeText(decision.policy)}</td>`,
        `<td>${escapeText(decision.action)}</td>`,
        `<td>${decision.allowed ? "allowed" : "denied"}</td>`,
        `<td class="wrap">${escapeText(decision.allowed ? decision.grant : decision.reason)}</td>`,
        "</tr>",
      );
    }
    parts.push("</tbody></table>");
  }
  parts.push("<h4>SQL</h4>");
  if (section.queries.length === 0) {
    parts.push("<p class=\"empty\">No statement ran.</p>");
  } else {
    parts.push("<ol class=\"queries\">");
    for (const query of section.queries) {
      parts.push(
        `<li${query.error === "" ? "" : " class=\"query-error\""}>`,
        `<pre class="sql"><code>${escapeText(query.sql)}</code></pre>`,
        `<span class="duration">${escapeText(formatDuration(query.durationMs))}</span>`,
        query.error === "" ? "" : ` <span class="error-text">${escapeText(query.error)}</span>`,
        "</li>",
      );
    }
    parts.push("</ol>");
  }
  parts.push("<h4>Logs</h4>");
  if (section.logs.length === 0) {
    parts.push("<p class=\"empty\">Nothing was logged.</p>");
  } else {
    parts.push("<table class=\"logs\"><tbody>");
    for (const line of section.logs) {
      const attrs = Object.keys(line.attrs).length === 0 ? "" : JSON.stringify(line.attrs);
      parts.push(
        `<tr class="level-${escapeAttribute(safeClass(line.level))}">`,
        `<td>${escapeText(line.time === undefined ? "" : new Date(line.time).toISOString().slice(11, 23))}</td>`,
        `<td>${escapeText(line.level)}</td>`,
        `<td class="wrap">${escapeText(line.message)}${attrs === "" ? "" : ` <code class="attrs">${escapeText(attrs)}</code>`}</td>`,
        "</tr>",
      );
    }
    parts.push("</tbody></table>");
  }
  return parts.join("\n");
}

// formatDuration writes milliseconds the way a person reads a test's timing.
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return "";
  }
  if (ms < 1) {
    return `${ms.toFixed(2)} ms`;
  }
  if (ms < 1000) {
    return `${Math.round(ms)} ms`;
  }
  return `${(ms / 1000).toFixed(2)} s`;
}

// escapeText escapes a value for an element's content.
export function escapeText(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// escapeAttribute escapes a value for a double- or single-quoted attribute.
export function escapeAttribute(value: string): string {
  return escapeText(value).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

function safeClass(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9-]/g, "") || "log";
}

const styles = `
[hidden] { display: none !important; }
body { margin: 0; padding: 12px 16px; color: var(--vscode-foreground); background: var(--vscode-editor-background); font-family: var(--vscode-font-family); font-size: var(--vscode-font-size); }
header { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; margin-bottom: 12px; }
h1 { font-size: 1.25em; margin: 0; word-break: break-all; }
h2 { font-size: 1.05em; margin: 0 0 6px; }
h3 { font-size: 1em; margin: 16px 0 6px; word-break: break-all; }
h4 { font-size: 0.9em; margin: 10px 0 4px; color: var(--vscode-descriptionForeground); text-transform: uppercase; letter-spacing: 0.04em; }
.meta, .empty, .url, .request-id, .duration, .source-url { color: var(--vscode-descriptionForeground); }
.meta { margin: 0; flex-basis: 100%; order: 3; }
.badge { padding: 1px 8px; border-radius: 10px; font-size: 0.85em; }
.badge.failed { background: var(--vscode-testing-iconFailed, #c72e0f); color: var(--vscode-editor-background); }
.badge.passed { background: var(--vscode-testing-iconPassed, #388a34); color: var(--vscode-editor-background); }
button { font: inherit; color: var(--vscode-button-secondaryForeground); background: var(--vscode-button-secondaryBackground); border: 1px solid transparent; border-radius: 2px; padding: 2px 8px; cursor: pointer; }
button:hover { background: var(--vscode-button-secondaryHoverBackground); }
button:focus-visible { outline: 1px solid var(--vscode-focusBorder); outline-offset: 1px; }
button[data-command="explain"] { color: var(--vscode-button-foreground); background: var(--vscode-button-background); margin-left: auto; }
.failures { border-left: 3px solid var(--vscode-testing-iconFailed, #c72e0f); padding: 4px 10px; margin-bottom: 12px; background: var(--vscode-inputValidation-errorBackground, transparent); }
.failures ul { margin: 0; padding-left: 18px; }
.kind { font-weight: 600; }
.layout { display: grid; grid-template-columns: minmax(180px, 280px) 1fr; gap: 16px; align-items: start; }
@media (max-width: 640px) { .layout { grid-template-columns: 1fr; } }
.steps ol { list-style: none; margin: 0; padding: 0; }
.step { display: grid; grid-template-columns: 2.2em 1fr auto; gap: 2px 6px; width: 100%; text-align: left; margin-bottom: 2px; color: var(--vscode-foreground); background: transparent; }
.step[aria-pressed="true"] { background: var(--vscode-list-activeSelectionBackground); color: var(--vscode-list-activeSelectionForeground); }
.step.error .index { color: var(--vscode-testing-iconFailed, #c72e0f); font-weight: 700; }
.step .target { grid-column: 2 / 4; color: var(--vscode-descriptionForeground); overflow-wrap: anywhere; }
.tabs { display: flex; flex-wrap: wrap; gap: 4px; border-bottom: 1px solid var(--vscode-panel-border); margin-bottom: 10px; }
.tabs button { background: transparent; color: var(--vscode-foreground); border-radius: 0; border-bottom: 2px solid transparent; }
.tabs button[aria-selected="true"] { border-bottom-color: var(--vscode-focusBorder); }
img { max-width: 100%; border: 1px solid var(--vscode-panel-border); }
pre { white-space: pre-wrap; overflow-wrap: anywhere; margin: 0; font-family: var(--vscode-editor-font-family); font-size: var(--vscode-editor-font-size); }
.source { max-height: 70vh; overflow: auto; padding: 8px; background: var(--vscode-textCodeBlock-background); }
.step-error, .error-text, .request-error-text { color: var(--vscode-errorForeground); }
.step-error { margin-bottom: 8px; }
table { border-collapse: collapse; width: 100%; }
th, td { text-align: left; padding: 3px 6px; border-bottom: 1px solid var(--vscode-panel-border); vertical-align: top; }
td.wrap { overflow-wrap: anywhere; }
tr.current { background: var(--vscode-list-inactiveSelectionBackground); }
tr.status-5xx td, tr.request-error td, tr.denied td, li.query-error .sql { color: var(--vscode-errorForeground); }
tr.status-4xx td { color: var(--vscode-editorWarning-foreground); }
.console { list-style: none; padding: 0; margin: 0; }
.console li { padding: 3px 0; border-bottom: 1px solid var(--vscode-panel-border); overflow-wrap: anywhere; }
.console .level { display: inline-block; min-width: 4em; color: var(--vscode-descriptionForeground); }
.level-error .level, .level-error td { color: var(--vscode-errorForeground); }
.level-warning .level, .level-warn td { color: var(--vscode-editorWarning-foreground); }
.queries { padding-left: 20px; margin: 0; }
.queries li { margin-bottom: 6px; }
.attrs { color: var(--vscode-descriptionForeground); overflow-wrap: anywhere; }
`;

// The page's behaviour: choosing a step, a tab, a request's server section, and
// asking the editor to explain the failure. It reads everything from the
// document the extension rendered; no value from the trace is ever turned into
// markup here.
const script = `
(function () {
  const vscode = acquireVsCodeApi();
  const body = document.body;
  let selected = body.dataset.selected || "";
  let serverRequest = "";
  let serverAll = false;
  const has = (list, value) => (" " + (list || "") + " ").indexOf(" " + value + " ") >= 0;
  function showServer() {
    let visible = 0;
    document.querySelectorAll("[data-server]").forEach(function (article) {
      const show = serverAll || (serverRequest !== "" ? article.dataset.server === serverRequest : has(article.dataset.steps, selected));
      article.hidden = !show;
      if (show) { visible++; }
    });
    const none = document.querySelector("[data-server-none]");
    if (none) { none.hidden = visible > 0; }
    const scope = document.querySelector("[data-server-scope]");
    if (scope) { scope.textContent = serverAll ? "Every request." : serverRequest !== "" ? "One request." : "Requests of the selected step."; }
    const toggle = document.querySelector("[data-server-all]");
    if (toggle) { toggle.textContent = serverAll || serverRequest !== "" ? "Show the selected step" : "Show every request"; }
  }
  function selectStep(index) {
    selected = String(index);
    serverRequest = "";
    serverAll = false;
    document.querySelectorAll("[data-step]").forEach(function (button) {
      button.setAttribute("aria-pressed", String(button.dataset.step === selected));
    });
    document.querySelectorAll("[data-step-panel]").forEach(function (panel) {
      panel.hidden = panel.dataset.stepPanel !== selected;
    });
    document.querySelectorAll("tr[data-steps]").forEach(function (row) {
      row.classList.toggle("current", has(row.dataset.steps, selected));
    });
    showServer();
  }
  function selectTab(id) {
    document.querySelectorAll("[data-tab]").forEach(function (tab) {
      tab.setAttribute("aria-selected", String(tab.dataset.tab === id));
    });
    document.querySelectorAll("[data-panel]").forEach(function (panel) {
      panel.hidden = panel.dataset.panel !== id;
    });
  }
  document.addEventListener("click", function (event) {
    const target = event.target instanceof Element ? event.target.closest("button") : null;
    if (!target) { return; }
    if (target.dataset.step) { selectStep(target.dataset.step); return; }
    if (target.dataset.tab) { selectTab(target.dataset.tab); return; }
    if (target.dataset.showServer) { serverRequest = target.dataset.showServer; serverAll = false; selectTab("server"); showServer(); return; }
    if (target.hasAttribute("data-server-all")) {
      if (serverAll || serverRequest !== "") { serverAll = false; serverRequest = ""; } else { serverAll = true; }
      showServer();
      return;
    }
    if (target.dataset.command === "explain") { vscode.postMessage({ type: "explain" }); }
  });
  document.addEventListener("keydown", function (event) {
    const current = document.activeElement;
    if (!(current instanceof HTMLElement) || !current.dataset.step) { return; }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") { return; }
    const buttons = Array.prototype.slice.call(document.querySelectorAll("[data-step]"));
    const next = buttons[buttons.indexOf(current) + (event.key === "ArrowDown" ? 1 : -1)];
    if (next) { event.preventDefault(); next.focus(); selectStep(next.dataset.step); }
  });
  showServer();
})();
`;
