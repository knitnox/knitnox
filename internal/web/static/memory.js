// Knitnox memory page (loaded only there).
//   - the form: "Add another connection" row
//   - the map: a small force-directed drawing of things and their
//     connections. It starts with a handful of things and brings in the
//     neighbours of whatever you click, so it never loads the whole graph.
// CSP forbids inline styles, so the map is SVG positioned with attributes.
(() => {
  "use strict";
  const $ = (sel, root = document) => root.querySelector(sel);
  const SVG_NS = "http://www.w3.org/2000/svg";
  function svgEl(name, attrs, parent) {
    const n = document.createElementNS(SVG_NS, name);
    for (const [k, v] of Object.entries(attrs || {})) n.setAttribute(k, v);
    if (parent) parent.append(n);
    return n;
  }
  function htmlEl(name, props, parent) {
    const n = Object.assign(document.createElement(name), props || {});
    if (parent) parent.append(n);
    return n;
  }

  // ---------- form: one more empty connection row ----------
  document.addEventListener("click", (e) => {
    const add = e.target.closest("[data-rel-add]");
    if (!add) return;
    const rows = $("[data-rel-rows]", add.parentElement);
    const row = rows.lastElementChild.cloneNode(true);
    for (const input of row.querySelectorAll("input")) input.value = "";
    rows.append(row);
    $("input", row).focus();
  });

  // ---------- the map ----------
  const box = $("#mem-graph");
  if (!box) return;
  const info = $("#mem-graph-info");
  const src = box.dataset.src;
  const home = Number(box.dataset.focus) || 0; // thing to start from (0 = overview)
  const still = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const MAX_NODES = 250;

  const svg = svgEl("svg", { focusable: "false" });
  const gEdges = svgEl("g", null, svg), gLabels = svgEl("g", null, svg), gNodes = svgEl("g", null, svg);
  let nodes = new Map(), edges = [], edgeKeys = new Set(), pairCount = new Map();
  let selected = null, W = 600, H = 420, heat = 0, raf = 0;

  function size() {
    W = box.clientWidth || 600; H = box.clientHeight || 420;
    svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  }
  const radius = (d) => 9 + Math.min(9, Math.sqrt(d.links) * 3);
  const shorten = (s) => (s.length > 22 ? s.slice(0, 21) + "…" : s);
  const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;
  const clampX = (x) => Math.max(30, Math.min(W - 30, x));
  const clampY = (y) => Math.max(24, Math.min(H - 34, y));

  function addNode(d, near) {
    let n = nodes.get(d.id);
    if (n) { n.memories = d.memories; n.links = d.links; return n; }
    if (nodes.size >= MAX_NODES) return null;
    const angle = Math.random() * Math.PI * 2;
    const dist = near ? 70 : Math.min(W, H) * (0.1 + 0.3 * Math.random());
    n = { id: d.id, name: d.name, memories: d.memories, links: d.links, vx: 0, vy: 0, held: false,
      x: clampX((near ? near.x : W / 2) + Math.cos(angle) * dist), y: clampY((near ? near.y : H / 2) + Math.sin(angle) * dist) };
    n.g = svgEl("g", { class: "g-node", tabindex: "0", role: "button" }, gNodes);
    n.g.setAttribute("aria-label", `${d.name}: ${plural(d.memories, "memory", "memories")}, ${plural(d.links, "connection", "connections")}`);
    svgEl("circle", { r: radius(d) }, n.g);
    svgEl("text", { dy: radius(d) + 14 }, n.g).textContent = shorten(d.name);
    svgEl("title", null, n.g).textContent = d.name;
    n.g.addEventListener("pointerdown", (e) => startDrag(e, n));
    n.g.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); select(n); }
    });
    nodes.set(d.id, n);
    return n;
  }

  function addEdge(d) {
    const key = `${d.s}|${d.p}|${d.t}`;
    const s = nodes.get(d.s), t = nodes.get(d.t);
    if (!s || !t || edgeKeys.has(key)) return;
    edgeKeys.add(key);
    const pair = d.s < d.t ? `${d.s}-${d.t}` : `${d.t}-${d.s}`;
    const k = pairCount.get(pair) || 0; // labels of parallel connections stack
    pairCount.set(pair, k + 1);
    const e = { s, t, p: d.p, k };
    e.line = svgEl("line", { class: "g-edge" }, gEdges);
    e.arrow = svgEl("path", { class: "g-arrow", d: "M0 0 L-10 -4.5 L-10 4.5 Z" }, gEdges); // points at the thing the connection leads to
    e.label = svgEl("text", { class: "g-elabel" }, gLabels);
    e.label.textContent = d.p;
    edges.push(e);
  }

  // One step of the layout: circles push each other away, connections pull
  // their two ends together, and everything drifts gently to the middle.
  function tick() {
    const list = [...nodes.values()];
    const rest = Math.max(110, Math.min(190, Math.min(W, H) * 0.42)); // how long a connection likes to be
    for (let i = 0; i < list.length; i++) {
      const a = list[i];
      for (let j = i + 1; j < list.length; j++) {
        const b = list[j];
        let dx = a.x - b.x, dy = a.y - b.y, d2 = dx * dx + dy * dy;
        if (d2 < 1) { dx = Math.random() - 0.5; dy = Math.random() - 0.5; d2 = 1; }
        const d = Math.sqrt(d2), f = Math.min(14, 9000 / d2);
        a.vx += (dx / d) * f; a.vy += (dy / d) * f;
        b.vx -= (dx / d) * f; b.vy -= (dy / d) * f;
      }
      a.vx += (W / 2 - a.x) * 0.0025;
      a.vy += (H / 2 - a.y) * 0.006;
    }
    for (const e of edges) {
      const dx = e.t.x - e.s.x, dy = e.t.y - e.s.y, d = Math.hypot(dx, dy) || 1;
      const f = (d - rest) * 0.02;
      e.s.vx += (dx / d) * f; e.s.vy += (dy / d) * f;
      e.t.vx -= (dx / d) * f; e.t.vy -= (dy / d) * f;
    }
    for (const n of list) {
      if (n.held) { n.vx = n.vy = 0; continue; }
      n.vx *= 0.8; n.vy *= 0.8;
      n.x = clampX(n.x + n.vx * heat);
      n.y = clampY(n.y + n.vy * heat);
    }
  }

  function draw() {
    for (const n of nodes.values()) n.g.setAttribute("transform", `translate(${n.x.toFixed(1)} ${n.y.toFixed(1)})`);
    for (const e of edges) {
      // The line runs from the edge of one circle to an arrow head touching the other.
      const dx = e.t.x - e.s.x, dy = e.t.y - e.s.y, d = Math.hypot(dx, dy) || 1;
      const ux = dx / d, uy = dy / d;
      const from = Math.min(radius(e.s), d / 2), to = Math.min(radius(e.t) + 2, d / 2);
      const tipX = e.t.x - ux * to, tipY = e.t.y - uy * to;
      e.line.setAttribute("x1", (e.s.x + ux * from).toFixed(1)); e.line.setAttribute("y1", (e.s.y + uy * from).toFixed(1));
      e.line.setAttribute("x2", (tipX - ux * 8).toFixed(1)); e.line.setAttribute("y2", (tipY - uy * 8).toFixed(1));
      e.arrow.setAttribute("transform", `translate(${tipX.toFixed(1)} ${tipY.toFixed(1)}) rotate(${(Math.atan2(dy, dx) * 180 / Math.PI).toFixed(1)})`);
      e.label.setAttribute("x", ((e.s.x + e.t.x) / 2).toFixed(1));
      e.label.setAttribute("y", ((e.s.y + e.t.y) / 2 - 5 + e.k * 13).toFixed(1));
    }
  }

  function step() {
    tick(); draw();
    heat *= 0.985;
    raf = heat > 0.03 ? requestAnimationFrame(step) : 0;
  }
  // (Re)start the layout with some energy. People who asked for reduced
  // motion get the finished picture at once.
  function settle(energy) {
    heat = Math.max(heat, energy);
    if (still) {
      for (let i = 0; i < 280; i++) { tick(); heat *= 0.985; }
      heat = 0; draw();
    } else if (!raf) raf = requestAnimationFrame(step);
  }

  function startDrag(ev, n) {
    if (ev.button) return;
    ev.preventDefault();
    const sx = ev.clientX, sy = ev.clientY;
    let moved = false;
    n.g.setPointerCapture(ev.pointerId);
    const move = (e) => {
      if (!moved && Math.hypot(e.clientX - sx, e.clientY - sy) < 5) return;
      moved = true; n.held = true; // a circle you placed stays where you put it
      const r = svg.getBoundingClientRect();
      n.x = clampX(((e.clientX - r.left) * W) / r.width);
      n.y = clampY(((e.clientY - r.top) * H) / r.height);
      if (still) draw(); else settle(0.35);
    };
    const up = () => {
      n.g.removeEventListener("pointermove", move);
      n.g.removeEventListener("pointerup", up);
      n.g.removeEventListener("pointercancel", up);
      if (!moved) select(n);
    };
    n.g.addEventListener("pointermove", move);
    n.g.addEventListener("pointerup", up);
    n.g.addEventListener("pointercancel", up);
  }

  function paint() {
    const near = new Set();
    if (selected) {
      near.add(selected);
      for (const e of edges) if (e.s === selected || e.t === selected) { near.add(e.s); near.add(e.t); }
    }
    for (const n of nodes.values()) {
      n.g.classList.toggle("is-sel", n === selected);
      n.g.classList.toggle("is-dim", !!selected && !near.has(n));
    }
    for (const e of edges) {
      const on = !!selected && (e.s === selected || e.t === selected);
      e.line.classList.toggle("is-on", on);
      e.arrow.classList.toggle("is-on", on);
      e.label.classList.toggle("is-on", on);
    }
  }

  function say(text) {
    info.textContent = "";
    htmlEl("p", { className: "hint", textContent: text }, info);
  }

  // The box under the map: what the chosen thing is connected to, in words.
  function showInfo(n) {
    info.textContent = "";
    const head = htmlEl("div", { className: "graph-info-head" }, info);
    htmlEl("b", { textContent: n.name }, head);
    htmlEl("span", { className: "hint", textContent: `${plural(n.memories, "memory", "memories")} · ${plural(n.links, "connection", "connections")}` }, head);
    htmlEl("a", { className: "btn sm", href: `/memory/things/${n.id}`, textContent: "Open its page" }, head);
    const mine = edges.filter((e) => e.s === n || e.t === n);
    if (!mine.length) { htmlEl("p", { className: "hint", textContent: "It is not connected to anything yet." }, info); return; }
    const list = htmlEl("ul", { className: "rel-list" }, info);
    const end = (parent, node) => {
      if (node === n) { htmlEl("b", { textContent: node.name }, parent); return; }
      const a = htmlEl("a", { href: `/memory/things/${node.id}`, textContent: node.name, title: "Show this on the map" }, parent);
      a.addEventListener("click", (e) => { e.preventDefault(); select(node); });
    };
    for (const e of mine.slice(0, 14)) {
      const li = htmlEl("li", { className: "rel" }, list);
      end(li, e.s);
      htmlEl("i", { textContent: e.p }, li);
      end(li, e.t);
    }
    if (mine.length > 14) htmlEl("p", { className: "hint", textContent: `And ${mine.length - 14} more. Open its page to see them all.` }, info);
  }

  async function load(focus, near) {
    let data;
    try {
      const r = await fetch(`${src}?focus=${focus}`, { headers: { "X-Fetch": "1" } });
      if (!r.ok) throw new Error(String(r.status));
      data = await r.json();
    } catch {
      say("The map could not be loaded. Check your connection, then press “Start over”.");
      return null;
    }
    for (const d of data.nodes) addNode(d, near);
    for (const d of data.edges) addEdge(d);
    settle(1);
    return data;
  }

  async function select(n) {
    selected = n;
    n.g.focus({ preventScroll: true });
    paint();
    const data = await load(n.id, n); // bring in its neighbours
    if (!data || selected !== n) return;
    paint();
    showInfo(n);
    if (nodes.size >= MAX_NODES) htmlEl("p", { className: "hint", textContent: "The map is full. Press “Start over” to clear it." }, info);
  }

  async function reset() {
    gEdges.textContent = gLabels.textContent = gNodes.textContent = "";
    nodes = new Map(); edges = []; edgeKeys = new Set(); pairCount = new Map(); selected = null;
    size();
    const data = await load(home, null);
    if (!data) return;
    $(".graph-empty", box)?.remove();
    if (!svg.isConnected) box.append(svg);
    if (!nodes.size) { say("Nothing to draw yet. Connections appear here once a memory links two things."); return; }
    const start = nodes.get(home);
    if (start) { selected = start; paint(); say("Click another circle to see what it is connected to."); } // its own links are listed below the map
    else say(data.more ? "These are your best-connected things. Click one to bring in everything it is linked to." : "Click a circle to see what it is connected to.");
  }

  document.addEventListener("click", (e) => { if (e.target.closest("[data-graph-reset]")) reset(); });
  new ResizeObserver(() => { const w = W; size(); if (svg.isConnected && w !== W) settle(0.4); }).observe(box);
  reset();
})();
