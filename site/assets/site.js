"use strict";

// Tabs: <div class="tabs" role="tablist"> buttons with aria-controls.
document.querySelectorAll('[role="tablist"]').forEach((list) => {
  const tabs = [...list.querySelectorAll('[role="tab"]')];
  const select = (tab) => {
    tabs.forEach((t) => {
      const on = t === tab;
      t.setAttribute("aria-selected", on);
      t.tabIndex = on ? 0 : -1;
      document.getElementById(t.getAttribute("aria-controls")).hidden = !on;
    });
  };
  tabs.forEach((t, i) => {
    t.addEventListener("click", () => select(t));
    t.addEventListener("keydown", (e) => {
      const d = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
      if (d) { const n = tabs[(i + d + tabs.length) % tabs.length]; n.focus(); select(n); }
    });
  });
});

// Copy buttons copy the text of their target element.
document.querySelectorAll("[data-copy]").forEach((b) => {
  b.addEventListener("click", async () => {
    const panel = b.closest("[data-copy-scope]") || document;
    const visible = [...panel.querySelectorAll(b.dataset.copy)].find((n) => !n.closest("[hidden]"));
    if (!visible) return;
    try {
      await navigator.clipboard.writeText(visible.textContent.trim());
      b.textContent = "Copied";
    } catch {
      b.textContent = "Press Ctrl+C";
    }
    setTimeout(() => { b.textContent = "Copy"; }, 1600);
  });
});

// Play the terminal demo once, when it scrolls into view.
const term = document.querySelector(".term");
if (term) {
  term.querySelectorAll(".row").forEach((r, i) => r.style.setProperty("--i", i));
  const io = new IntersectionObserver((entries) => {
    if (entries.some((e) => e.isIntersecting)) { term.classList.add("play"); io.disconnect(); }
  }, { threshold: 0.2 });
  io.observe(term);
}

// Docs: highlight the sidebar link for the section in view.
const links = [...document.querySelectorAll(".side a[href^='#']")];
if (links.length) {
  const byId = new Map(links.map((a) => [a.getAttribute("href").slice(1), a]));
  const spy = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (e.isIntersecting && byId.has(e.target.id)) {
        links.forEach((a) => a.classList.remove("active"));
        byId.get(e.target.id).classList.add("active");
      }
    }
  }, { rootMargin: "-80px 0px -70% 0px" });
  byId.forEach((_, id) => { const el = document.getElementById(id); if (el) spy.observe(el); });
}
