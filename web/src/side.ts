// The side panel: what was analyzed, how to drive the diagram, and the
// unclassified list.

import type { Graph, Unclassified, UnclassifiedKind } from "./model";

const KIND_LABEL: Record<UnclassifiedKind, string> = {
  interface: "Interfaces",
  data: "Data types",
  service: "Services",
  value: "Values",
  other: "Other",
};

const KIND_NOTE: Record<UnclassifiedKind, string> = {
  interface:
    "Ports and repositories. Implemented elsewhere, so no aggregate holds them.",
  data: "Every field exported — the shape of something filled in from outside.",
  service: "No fields at all, so these can only be behaviour.",
  value:
    "Named types over something other than a struct, reached by no aggregate.",
  other: "Structs that fit none of the above. A forgotten marker would be here.",
};

export function escapeHtml(s: string): string {
  return s.replace(
    /[&<>"]/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c] ?? c
  );
}

export function plural(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}

/**
 * The aggregates a type takes or returns.
 *
 * A type living outside every aggregate is still part of the domain when it
 * works on one, and saying which one places it far better than calling it
 * "unclassified" does. Hovering lights up that aggregate in the diagram.
 */
function touches(names: string[] | undefined): string {
  if (!names?.length) return "";
  return (
    ` <span class="touches">` +
    names
      .map((n) => `<span class="touch" data-agg="${escapeHtml(n)}">${escapeHtml(n)}</span>`)
      .join("") +
    `</span>`
  );
}

function entry(u: Unclassified): string {
  const name = `<b>${escapeHtml(u.name)}</b>`;
  const pos = `<span class="pos">${escapeHtml(u.pos)}</span>`;

  if (!u.members?.length) {
    return `<div class="unc">${name}${touches(u.touches)}<br>${pos}</div>`;
  }

  const rows = u.members
    .map(
      (m) =>
        `<div class="unc-child" style="padding-left:${m.depth * 10}px">` +
        `${escapeHtml(m.name)} <span class="tag">${escapeHtml(m.kind)}</span>` +
        `${touches(m.touches)}<br>` +
        `<span class="pos">${escapeHtml(m.pos)}</span></div>`
    )
    .join("");

  return (
    `<details class="unc"><summary>${name}${touches(u.touches)} ` +
    `<span class="count">+${u.members.length}</span><br>${pos}</summary>` +
    `${rows}</details>`
  );
}

function unclassifiedSection(graph: Graph): string {
  const list = graph.unclassified;
  if (!list.length) return "";

  const total = graph.unclassifiedTotal ?? list.length;
  let out = `<h2>Unclassified ${list.length}</h2>`;
  if (total > list.length) {
    out +=
      `<div class="hint" style="margin:-4px 0 10px">` +
      `${plural(total, "type")}, folded into ${list.length} entries</div>`;
  }

  // Entries arrive already grouped by kind.
  let current: UnclassifiedKind | null = null;
  for (const u of list) {
    if (u.kind !== current) {
      current = u.kind;
      out +=
        `<h3 class="group">${escapeHtml(KIND_LABEL[current])}</h3>` +
        `<div class="group-note">${escapeHtml(KIND_NOTE[current])}</div>`;
    }
    out += entry(u);
  }

  out +=
    `<div class="hint" style="margin-top:14px">Types no aggregate root can ` +
    `reach. Most are meant to be here; a missing <code>//ddd:aggregate</code> ` +
    `would be too.</div>`;
  return out;
}

export interface SideHandlers {
  onExpandAll: () => void;
  onCollapseAll: () => void;
  onToggleServices: () => void;
  servicesShown: boolean;
  onHighlight: (aggregate: string | null) => void;
}

export function buildSide(
  side: HTMLElement,
  graph: Graph,
  handlers: SideHandlers
): void {
  const aggCount = graph.aggregates.length;

  let html =
    `<h1>${escapeHtml(graph.meta.title)}</h1>` +
    `<div class="sub">${plural(aggCount, "aggregate")} / ` +
    `${plural(graph.references.length, "reference")}</div>`;

  // With no diagram on screen the controls would only be noise.
  if (aggCount) {
    html +=
      `<h2>Controls</h2><div class="hint">` +
      "Click an aggregate to expand or collapse<br>" +
      "Hover to highlight what it relates to<br>" +
      "Drag to pan, <kbd>wheel</kbd> to zoom<br>" +
      "<kbd>f</kbd> to fit on screen<br><br>" +
      `<button class="link" id="all-open">Expand all</button> / ` +
      `<button class="link" id="all-close">Collapse all</button>` +
      "</div>";
  }

  const services = graph.services ?? [];
  if (services.length) {
    html +=
      `<h2>Services ${services.length}</h2>` +
      `<div class="group-note">Types outside every aggregate whose methods ` +
      `take or return one — repositories, domain services, policies.</div>` +
      `<div class="hint" style="margin-bottom:8px">` +
      `<button class="link" id="toggle-services">` +
      (handlers.servicesShown ? "Hide in diagram" : "Show in diagram") +
      `</button></div>`;

    let lastAgg: string | null = null;
    for (const s of services) {
      const head = s.touches[0] ?? "";
      if (head !== lastAgg) {
        lastAgg = head;
        html +=
          `<div class="svc-group"><span class="touch" data-agg="${escapeHtml(head)}">` +
          `${escapeHtml(head)}</span></div>`;
      }
      html +=
        `<div class="unc"><b>${escapeHtml(s.name)}</b> ` +
        `<span class="tag">${escapeHtml(s.kind)}</span><br>` +
        `<span class="pos">${escapeHtml(s.pos)}</span></div>`;
    }
  }

  html += unclassifiedSection(graph);
  side.innerHTML = html;

  // Hovering an aggregate name in the list lights it up in the diagram.
  side.querySelectorAll<HTMLElement>(".touch").forEach((el) => {
    el.addEventListener("mouseenter", () =>
      handlers.onHighlight(el.dataset["agg"] ?? null)
    );
    el.addEventListener("mouseleave", () => handlers.onHighlight(null));
  });

  document
    .getElementById("toggle-services")
    ?.addEventListener("click", handlers.onToggleServices);

  if (!aggCount) return;
  document.getElementById("all-open")?.addEventListener("click", handlers.onExpandAll);
  document.getElementById("all-close")?.addEventListener("click", handlers.onCollapseAll);
}

/**
 * Shown when nothing carries a marker. An empty canvas reads as a broken
 * tool, so the page says what is missing and where to start.
 */
export function gettingStarted(graph: Graph): string {
  const cands = graph.candidates ?? [];
  const rows = cands
    .slice(0, 12)
    .map(
      (c) =>
        `<tr><td><b>${escapeHtml(c.name)}</b></td>` +
        `<td>${escapeHtml(c.pos)}</td>` +
        `<td>${escapeHtml(c.idType)}</td></tr>`
    )
    .join("");

  let html =
    `<div class="empty"><h2>Nothing is marked yet</h2>` +
    `<p>dddviz found no type marked with <code>//ddd:aggregate</code>, ` +
    `so there is no aggregate to draw.</p>` +
    `<p>Mark an aggregate root and run again:</p>` +
    `<pre>//ddd:aggregate\ntype Order struct {\n\tid       OrderID\n\tcustomer CustomerID\n}</pre>`;

  if (rows) {
    html +=
      `<h3>Types that own an ID type</h3><table>${rows}</table>` +
      `<p class="caveat">Owning an ID does not make a type an aggregate root — ` +
      `roots without their own ID exist, which is why the marker is needed at ` +
      `all. Treat this as a place to start reading, not an answer.</p>`;
  } else {
    html +=
      `<p class="caveat">No types owning an ID type were found either, so ` +
      `there is no list to start from here.</p>`;
  }

  return html + "</div>";
}
