// Entry point: state, viewport, interaction, and the live-reload channel.

import type { Graph } from "./model";
import { layoutGraph, type Layout } from "./layout";
import { render } from "./svg";
import { buildSide, gettingStarted } from "./side";

let graph: Graph = window.__DDDVIZ__;

// Everything starts collapsed, so the first view is the map between
// aggregates rather than a wall of detail.
const expanded = new Set<string>();

const stage = document.getElementById("stage") as HTMLElement;
const side = document.getElementById("side") as HTMLElement;

const view = { x: 0, y: 0, k: 1 };
let lastLayout: Layout | null = null;

// Expanding one aggregate keeps the viewport where it is; only the
// whole-diagram actions refit.
let refit = true;

// Services are off by default: the diagram is about the aggregates, and
// there are usually more services than aggregates.
let showServices = false;

// ---- viewport ---------------------------------------------------------

function applyView(): void {
  const cam = document.getElementById("camera");
  cam?.setAttribute(
    "transform",
    `translate(${view.x},${view.y}) scale(${view.k})`
  );
}

function fit(layout: Layout): void {
  const w = stage.clientWidth;
  const h = stage.clientHeight;
  if (!layout.width || !layout.height || !w || !h) return;

  const k = Math.min(w / (layout.width + 40), h / (layout.height + 40), 1.4);
  view.k = k;
  view.x = (w - layout.width * k) / 2;
  view.y = (h - layout.height * k) / 2;
  applyView();
}

// ---- drawing ----------------------------------------------------------

function draw(): void {
  if (!graph.aggregates.length) {
    stage.innerHTML = gettingStarted(graph);
    return;
  }

  const layout = layoutGraph(graph, expanded, showServices);
  const svg = render(stage, layout);
  lastLayout = layout;

  applyView();
  bindNodes(svg);
  if (refit) {
    refit = false;
    fit(layout);
  }
}

function rebuild(refitNext: boolean): void {
  refit = refitNext;
  draw();
}

// ---- interaction ------------------------------------------------------

// Stage handlers are bound once. Rebinding on every redraw would pile
// listeners up and slow the page down.
function bindStage(): void {
  let drag: { x: number; y: number } | null = null;

  stage.addEventListener("mousedown", (e) => {
    if (e.button !== 0) return;
    drag = { x: e.clientX - view.x, y: e.clientY - view.y };
    stage.classList.add("dragging");
  });

  window.addEventListener("mousemove", (e) => {
    if (!drag) return;
    view.x = e.clientX - drag.x;
    view.y = e.clientY - drag.y;
    applyView();
  });

  window.addEventListener("mouseup", () => {
    drag = null;
    stage.classList.remove("dragging");
  });

  stage.addEventListener(
    "wheel",
    (e) => {
      e.preventDefault();
      const r = stage.getBoundingClientRect();
      const mx = e.clientX - r.left;
      const my = e.clientY - r.top;
      const f = Math.exp(-e.deltaY * 0.0016);
      const k = Math.min(3, Math.max(0.15, view.k * f));
      view.x = mx - (mx - view.x) * (k / view.k);
      view.y = my - (my - view.y) * (k / view.k);
      view.k = k;
      applyView();
    },
    { passive: false }
  );

  window.addEventListener("keydown", (e) => {
    if (e.key === "f" && lastLayout) fit(lastLayout);
  });
}

// Node handlers are rebound on every redraw.
function bindNodes(svg: SVGSVGElement): void {
  svg.querySelectorAll<SVGGElement>(".agg").forEach((g) => {
    g.addEventListener("click", (e) => {
      if ((e.target as Element).closest(".member")) return;
      const name = g.getAttribute("data-agg");
      if (!name) return;
      if (expanded.has(name)) expanded.delete(name);
      else expanded.add(name);
      rebuild(false);
    });
  });

  svg.querySelectorAll<SVGGElement>(".node").forEach((g) => {
    g.addEventListener("mouseenter", () => {
      const name = g.classList.contains("agg")
        ? g.getAttribute("data-agg")
        : g.closest(".agg")?.getAttribute("data-agg");
      if (name) highlight(svg, name);
    });
    g.addEventListener("mouseleave", () => clearHighlight(svg));
  });
}

function highlight(svg: SVGSVGElement, aggName: string): void {
  const keep = new Set([aggName]);

  svg.querySelectorAll<SVGGElement>(".edge").forEach((e) => {
    const from = e.getAttribute("data-from");
    const to = e.getAttribute("data-to");
    if (from === aggName || to === aggName) {
      if (from) keep.add(from);
      if (to) keep.add(to);
      e.classList.add("hl");
    } else {
      e.classList.add("fade");
    }
  });

  svg.querySelectorAll<SVGGElement>(".agg").forEach((g) => {
    const n = g.getAttribute("data-agg");
    if (n && keep.has(n)) {
      if (n === aggName) g.classList.add("hl");
    } else {
      g.classList.add("fade");
    }
  });

  svg.classList.add("has-focus");
}

function clearHighlight(svg: SVGSVGElement): void {
  svg.classList.remove("has-focus");
  svg.querySelectorAll(".fade, .hl").forEach((n) => {
    n.classList.remove("fade", "hl");
  });
}

// ---- live reload (-watch) ---------------------------------------------

// The server sends a new graph rather than telling the page to reload, so
// the expanded aggregates survive an edit. Names that no longer exist are
// simply never looked up again.
function connectLive(): void {
  const src = new EventSource("/events");

  src.addEventListener("graph", (e) => {
    graph = JSON.parse((e as MessageEvent<string>).data) as Graph;
    clearBanner();
    renderSide();
    rebuild(false);
  });

  src.addEventListener("failed", (e) => {
    // Code mid-edit does not compile. Keep the last good diagram up and say
    // why it stopped moving.
    showBanner(JSON.parse((e as MessageEvent<string>).data) as string);
  });

  src.addEventListener("error", () => {
    showBanner("Disconnected from dddviz. Is the -watch process still running?");
  });
}

function showBanner(message: string): void {
  let b = document.getElementById("banner");
  if (!b) {
    b = document.createElement("div");
    b.id = "banner";
    document.getElementById("app")?.appendChild(b);
  }
  b.textContent = firstUsefulLine(message);
  b.title = message;
}

function clearBanner(): void {
  document.getElementById("banner")?.remove();
}

// A build failure leads with a heading and the package name; the line worth
// showing is the first one naming a file and a position.
function firstUsefulLine(message: string): string {
  const lines = String(message).split("\n");
  for (const line of lines) {
    const trimmed = line.trim();
    if (/\.go:\d+(:\d+)?:/.test(trimmed)) {
      return trimmed.replace(/^.*?([^/\\]+\.go:\d+)/, "$1");
    }
  }
  return lines[0] ?? "";
}

// ---- start ------------------------------------------------------------

function renderSide(): void {
  buildSide(side, graph, {
    onExpandAll: () => {
      for (const a of graph.aggregates) expanded.add(a.name);
      rebuild(true);
    },
    onCollapseAll: () => {
      expanded.clear();
      rebuild(true);
    },
    onToggleServices: () => {
      showServices = !showServices;
      renderSide();
      rebuild(true);
    },
    servicesShown: showServices,
    onHighlight: (name) => {
      const svg = stage.querySelector("svg");
      if (!svg) return;
      if (name) highlight(svg, name);
      else clearHighlight(svg);
    },
  });
}

renderSide();
bindStage();
draw();
if (window.__DDDVIZ_LIVE__) connectLive();
