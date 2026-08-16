// Drawing. Takes placed boxes from the layout and produces SVG.

import type { BoxLike, Member } from "./model";
import type { Layout, PlacedAggregate } from "./layout";
import {
  CHAR,
  DOC_ROW,
  HEAD,
  PAD,
  ROW,
  boxChars,
  docLine,
  wrapValues,
} from "./measure";

const NS = "http://www.w3.org/2000/svg";

export function el<K extends keyof SVGElementTagNameMap>(
  name: K,
  attrs: Record<string, string | number | undefined> = {},
  parent?: Element
): SVGElementTagNameMap[K] {
  const n = document.createElementNS(NS, name);
  for (const [k, v] of Object.entries(attrs)) {
    if (v !== undefined) n.setAttribute(k, String(v));
  }
  parent?.appendChild(n);
  return n;
}

function text(
  parent: Element,
  x: number,
  y: number,
  cls: string,
  content: string
): SVGTextElement {
  const t = el("text", { x, y, class: cls }, parent);
  t.textContent = content;
  return t;
}

/**
 * A row whose runs are separated by literal spaces.
 *
 * Browsers keep those spaces, but a standalone SVG renderer collapses them
 * by default and the columns run together, so the rows say so explicitly.
 */
function row(parent: Element, x: number, y: number, cls: string): SVGTextElement {
  const t = el("text", { x, y, class: cls }, parent);
  t.setAttribute("xml:space", "preserve");
  return t;
}

/**
 * Draw the doc line, fields and methods, returning the y it finished at.
 *
 * Full doc text and method docs go into <title> elements, so hovering
 * recovers what truncation dropped without the box having to grow.
 */
function drawBody(
  g: Element,
  t: BoxLike,
  x: number,
  startY: number,
  boxWidth: number
): number {
  let y = startY;
  const chars = boxChars(boxWidth);

  if (t.doc) {
    const d = text(g, x, y + 10, "doc", docLine(t.doc, chars));
    el("title", {}, d).textContent = t.doc;
    y += DOC_ROW + 4;
  }

  if (t.fields.length) {
    t.fields.forEach((f, i) => {
      const line = row(g, x, y + 11 + i * ROW, "field");
      el("tspan", { class: "field-name" }, line).textContent = f.name;
      el("tspan", {}, line).textContent = "  " + f.type;
    });
    y += t.fields.length * ROW + 4;
  }

  // The constants come before the methods: what a type can be is read
  // before what it can do.
  const values = t.values ?? [];
  if (values.length) {
    el(
      "line",
      { x1: x, y1: y + 2, x2: x + boxWidth - PAD * 2, y2: y + 2, class: "rule dim" },
      g
    );
    y += 4;

    const lines = wrapValues(values, chars);
    lines.forEach((content, i) => {
      const line = row(g, x, y + 11 + i * ROW, "enum");
      el("tspan", {}, line).textContent = content;
    });

    const withDocs = values.filter((v) => v.doc);
    if (withDocs.length) {
      el("title", {}, g).textContent = withDocs
        .map((v) => `${v.name} — ${v.doc}`)
        .join("\n");
    }
    y += lines.length * ROW + 4;
  }

  const methods = t.methods ?? [];
  if (methods.length) {
    el(
      "line",
      { x1: x, y1: y + 2, x2: x + boxWidth - PAD * 2, y2: y + 2, class: "rule dim" },
      g
    );
    y += 4;

    // The receiver mark only earns its place when a type mixes the two
    // forms. Marking every method of a uniformly pointer-receiver type says
    // nothing, and a pointer receiver does not by itself mean mutation.
    const mixed =
      methods.some((m) => m.pointer) && methods.some((m) => !m.pointer);

    methods.forEach((m, j) => {
      const line = row(g, x, y + 11 + j * ROW, "method");
      if (mixed) {
        el("tspan", { class: "recv" }, line).textContent = m.pointer ? "•" : " ";
      }
      el("tspan", { class: "method-name" }, line).textContent =
        (mixed ? " " : "") + m.name;
      el("tspan", {}, line).textContent = m.signature;
      if (m.doc) {
        el("title", {}, line).textContent = `${m.name}${m.signature}\n\n${m.doc}`;
      }
    });
    y += methods.length * ROW + 4;
  }

  return y;
}

function drawMember(parent: Element, p: { member: Member; x: number; y: number; width: number; height: number }): void {
  const m = p.member;
  const g = el(
    "g",
    {
      class: `member node ${m.kind}`,
      transform: `translate(${p.x},${p.y})`,
      "data-type": m.name,
    },
    parent
  );

  el("rect", { width: p.width, height: p.height }, g);
  text(g, PAD, 16, "member-title", m.name);
  const badge = text(g, p.width - PAD, 16, "badge", m.kind.toUpperCase());
  badge.setAttribute("text-anchor", "end");

  if (m.fields.length || m.methods?.length || m.values?.length || m.doc) {
    el(
      "line",
      { x1: PAD, y1: HEAD - 6, x2: p.width - PAD, y2: HEAD - 6, class: "rule" },
      g
    );
    drawBody(g, m, PAD, HEAD - 4, p.width);
  }
}

function drawAggregate(parent: Element, p: PlacedAggregate): SVGGElement {
  const agg = p.aggregate;
  const g = el(
    "g",
    {
      class: "agg node",
      transform: `translate(${p.x},${p.y})`,
      "data-agg": agg.name,
    },
    parent
  );

  el("rect", { width: p.width, height: p.height }, g);

  const head = HEAD + bodyHeightOf(agg, p.width) + 6;
  el(
    "path",
    {
      class: "agg-head",
      d:
        `M0,6 a6,6 0 0 1 6,-6 h${p.width - 12} a6,6 0 0 1 6,6 ` +
        `v${head - 6} h${-p.width} z`,
    },
    g
  );
  el("line", { x1: 0, y1: head, x2: p.width, y2: head, class: "rule" }, g);

  text(g, PAD, 17, "agg-title", agg.name);
  if (agg.idType) {
    const idt = text(g, p.width - PAD, 17, "agg-id", agg.idType);
    idt.setAttribute("text-anchor", "end");
  }
  if (agg.doc) el("title", {}, g).textContent = agg.doc;
  drawBody(g, agg, PAD, HEAD - 4, p.width);

  if (!p.open && agg.members.length) {
    const n = agg.members.length;
    text(g, PAD, head + ROW, "badge", `${n} type${n === 1 ? "" : "s"} ▸`);
  }

  for (const m of p.members) drawMember(g, m);
  return g;
}

// Kept local so svg.ts does not have to import the whole measure module's
// aggregate helpers, which would be circular with layout.
function bodyHeightOf(t: BoxLike, boxWidth: number): number {
  let h = 0;
  if (t.doc) h += DOC_ROW + 4;
  if (t.fields.length) h += t.fields.length * ROW + 4;
  if (t.values?.length) {
    h += wrapValues(t.values, boxChars(boxWidth)).length * ROW + 8;
  }
  if (t.methods?.length) h += t.methods.length * ROW + 8;
  return h;
}

function drawEdges(parent: Element, layout: Layout): void {
  for (const e of layout.edges) {
    const g = el(
      "g",
      { class: "edge", "data-from": e.from, "data-to": e.to },
      parent
    );

    const first = e.points[0];
    if (!first) continue;
    let d = `M${first.x},${first.y}`;
    for (const p of e.points.slice(1)) d += `L${p.x},${p.y}`;
    el("path", { d, "marker-end": "url(#arrow)" }, g);

    const mid = e.points[Math.floor(e.points.length / 2)] ?? first;
    const label = text(g, mid.x, mid.y - 6, "edge-label", e.via);
    label.setAttribute("text-anchor", "middle");
  }
}

/** Render a full layout into stage, replacing whatever was there. */
export function render(stage: HTMLElement, layout: Layout): SVGSVGElement {
  stage.innerHTML = "";
  const svg = el("svg", {}, stage);

  const defs = el("defs", {}, svg);
  const marker = el(
    "marker",
    {
      id: "arrow",
      viewBox: "0 0 10 10",
      refX: "9",
      refY: "5",
      markerWidth: "7",
      markerHeight: "7",
      orient: "auto-start-reverse",
    },
    defs
  );
  el("path", { d: "M0,1 L9,5 L0,9 z", fill: "var(--edge)" }, marker);

  const camera = el("g", { id: "camera" }, svg);

  // Edges first, so they sit under the boxes.
  drawEdges(camera, layout);
  for (const p of layout.aggregates) drawAggregate(camera, p);

  return svg;
}
