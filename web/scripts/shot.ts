// Render a graph JSON to a standalone SVG, without a browser.
//
// The layout and drawing code are pure enough to run under jsdom, so the
// figures in the README come from the same code that draws the real page
// rather than from a screenshot. Vector output also means they stay sharp.
//
//   npm run shot -- graph.json out.svg [--expand]

import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { JSDOM } from "jsdom";

const [, , inPath, outPath, ...flags] = process.argv;

if (!inPath || !outPath) {
  console.error("usage: shot <graph.json> <out.svg> [--expand]");
  process.exit(1);
}

const dom = new JSDOM("<!doctype html><html><body><div id='stage'></div></body></html>");
const g = globalThis as unknown as {
  document: Document;
  window: unknown;
  SVGElement: unknown;
};
g.document = dom.window.document;
g.window = dom.window;
g.SVGElement = dom.window.SVGElement;

// Imported after the DOM exists, since the drawing module touches document
// at call time only, but keeping the order explicit avoids surprises.
const { layoutGraph } = await import("../src/layout.js");
const { render } = await import("../src/svg.js");
const model = await import("../src/model.js");
void model;

const graph = JSON.parse(readFileSync(inPath, "utf8"));
const expanded = new Set<string>(
  flags.includes("--expand")
    ? graph.aggregates.map((a: { name: string }) => a.name)
    : []
);

const layout = layoutGraph(graph, expanded);
const stage = dom.window.document.getElementById("stage") as unknown as HTMLElement;
const svg = render(stage, layout);

// jsdom has no layout engine, so getBBox is unavailable; the layout already
// knows the extents.
const M = 24;
const width = Math.round(layout.width + M * 2);
const height = Math.round(layout.height + M * 2);
svg.setAttribute("viewBox", `${-M} ${-M} ${width} ${height}`);
svg.setAttribute("width", String(width));
svg.setAttribute("height", String(height));
svg.setAttribute("xmlns", "http://www.w3.org/2000/svg");

// The page's stylesheet is inlined so the file stands alone. Custom
// properties are flattened to literal colours first: librsvg and other
// standalone SVG renderers do not implement them, and a var() they cannot
// resolve renders as nothing at all.
// Resolved against the working directory, which npm sets to web/.
const css = readFileSync(
  resolve(process.cwd(), "../internal/render/assets/style.css"),
  "utf8"
);

const vars = new Map<string, string>();
for (const block of css.matchAll(/:root[^{]*\{([^}]*)\}/g)) {
  for (const decl of (block[1] ?? "").matchAll(/(--[\w-]+):\s*([^;]+);/g)) {
    // Later blocks win, which puts the dark palette on top.
    vars.set(decl[1]!, decl[2]!.trim());
  }
}

const flattened = css
  // Media queries never match in a standalone renderer, and their contents
  // have already been folded into the variable map above.
  .replace(/@media[^{]*\{(?:[^{}]*\{[^}]*\})*[^}]*\}/g, "")
  .replace(/:root[^{]*\{[^}]*\}/g, "")
  .replace(/var\((--[\w-]+)\)/g, (_, name: string) => vars.get(name) ?? "none");

const style = dom.window.document.createElementNS(
  "http://www.w3.org/2000/svg",
  "style"
);
style.textContent = flattened;
svg.insertBefore(style, svg.firstChild);

// SVG has no background property, so the page colour is painted as a rect.
const bg = dom.window.document.createElementNS(
  "http://www.w3.org/2000/svg",
  "rect"
);
bg.setAttribute("x", String(-M));
bg.setAttribute("y", String(-M));
bg.setAttribute("width", String(width));
bg.setAttribute("height", String(height));
bg.setAttribute("fill", vars.get("--bg") ?? "#ffffff");
svg.insertBefore(bg, style.nextSibling);

writeFileSync(outPath, svg.outerHTML);
console.log(`wrote ${outPath} (${width}x${height})`);
