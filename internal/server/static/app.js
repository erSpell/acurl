"use strict";

const $ = (sel) => document.querySelector(sel);
const statusEl = $("#status");
let lastReport = null;

fetch("api/version").then((r) => r.json()).then((v) => { $("#version").textContent = v.version; }).catch(() => {});

function setStatus(text, isError) {
  statusEl.textContent = text;
  statusEl.classList.toggle("error", Boolean(isError));
}

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") node.className = v;
    else node.setAttribute(k, v);
  }
  for (const c of children) node.append(c);
  return node;
}

function details(r) {
  const parts = [];
  if (r.robots_only) parts.push("robots.txt token, not fetched");
  if (r.bot_challenge) parts.push("challenge page: " + r.bot_challenge);
  if (r.similarity != null && r.similarity < 0.95) parts.push(Math.round(r.similarity * 100) + "% similar to the browser");
  if (r.product_issue) parts.push("product: " + r.product_issue);
  if (r.robots_disallowed) parts.push("disallowed in robots.txt");
  if (r.js && r.js.client_rendered) parts.push("needs JavaScript" + (r.js.framework ? " (" + r.js.framework + ")" : ""));
  if (r.rate_limited) parts.push("rate limited");
  if (r.redirect_chain && r.redirect_chain.length) parts.push(r.redirect_chain.length + " redirect(s)");
  if (r.error) parts.push(r.error);
  if (!parts.length && r.classification === "blocked") parts.push("HTTP " + r.status_code + " error page");
  if (parts.length) return parts.join("; ");
  return r.browser && r.title ? r.title : "Same page as the browser";
}

function render(rep) {
  lastReport = rep;
  $("#result").hidden = false;
  $("#target").textContent = rep.url;
  $("#when").textContent = "Checked " + new Date(rep.timestamp).toLocaleString() + " with " + rep.results.length + " agents";

  const s = rep.score;
  const scored = s && !s.note;
  $("#score").textContent = scored ? s.value : "–";
  const grade = $("#grade");
  grade.textContent = scored ? s.grade : "not scored";
  grade.className = "grade g-" + (scored ? s.grade : "x");

  const rows = $("#rows");
  rows.replaceChildren();
  for (const r of rep.results) {
    rows.append(el("tr", {},
      el("td", {}, r.profile, el("span", { class: "cat" }, r.category || "")),
      el("td", {}, r.robots_only ? "–" : String(r.status_code || "ERR")),
      el("td", {}, el("span", { class: "verdict v-" + r.classification }, r.classification)),
      el("td", {}, details(r))));
  }

  const checks = $("#checks");
  checks.replaceChildren();
  if (s && s.note) {
    checks.append(el("h2", {}, "Not scored"), el("p", {}, s.note));
  } else if (s) {
    checks.append(el("h2", {}, "AI readiness checks"));
    for (const c of s.checks.filter((c) => c.applies)) {
      const row = el("div", { class: "check" },
        el("span", { class: c.passed ? "ok" : "no", "aria-label": c.passed ? "passed" : "failed" }, c.passed ? "✓" : "✗"),
        el("b", {}, c.title),
        el("span", { class: "pts" }, c.points + "/" + c.weight),
        el("span", { class: "detail" }, c.detail || ""));
      if (c.recommendation) row.append(el("span", { class: "fix" }, c.recommendation));
      checks.append(row);
    }
  }
}

let stream = null;

$("#check").addEventListener("submit", (e) => {
  e.preventDefault();
  const url = $("#url").value.trim();
  if (!url) { setStatus("Enter a page URL first.", true); return; }
  const categories = [...document.querySelectorAll("#cats input:checked")].map((i) => i.value);
  const button = $("#go");
  const finish = () => { button.disabled = false; button.textContent = "Check"; if (stream) stream.close(); stream = null; };
  if (stream) stream.close();
  button.disabled = true;
  button.textContent = "Checking";
  setStatus("Fetching " + url + " as each agent...");

  const params = new URLSearchParams({ url, categories: categories.join(",") });
  stream = new EventSource("api/check/stream?" + params);
  stream.addEventListener("progress", (ev) => {
    const p = JSON.parse(ev.data);
    setStatus(p.done + " of " + p.total + " agents done. Last: " + p.profile + " (" + (p.error ? "failed" : p.status) + ")");
  });
  stream.addEventListener("report", (ev) => { render(JSON.parse(ev.data)); setStatus(""); finish(); });
  stream.addEventListener("error", (ev) => {
    let msg = "The connection to acurl was lost. Is acurl serve still running?";
    if (ev.data) { try { msg = JSON.parse(ev.data).error; } catch (_) { /* keep default */ } }
    setStatus(msg, true);
    finish();
  });
});

document.querySelectorAll(".downloads button").forEach((b) => b.addEventListener("click", async () => {
  if (!lastReport) return;
  const format = b.dataset.format;
  let blob;
  if (format === "json") {
    blob = new Blob([JSON.stringify(lastReport, null, 2)], { type: "application/json" });
  } else {
    const res = await fetch("api/report?format=" + format, { method: "POST", body: JSON.stringify(lastReport) });
    if (!res.ok) { setStatus("Could not build the " + format + " report.", true); return; }
    blob = await res.blob();
  }
  const ext = { html: "html", markdown: "md", csv: "csv", json: "json" }[format];
  const a = el("a", { href: URL.createObjectURL(blob), download: "acurl-report." + ext });
  a.click();
  URL.revokeObjectURL(a.href);
}));

// ?url=... prefills the form and runs the check, so results can be linked.
const initial = new URLSearchParams(location.search).get("url");
if (initial) {
  $("#url").value = initial;
  $("#check").requestSubmit();
}
