// Layout is split in two, because the diagram is two different problems.
//
// Inside an aggregate the members have no edges between them, so there is
// nothing for a graph layout engine to solve — it is rectangle packing, and
// packing it here keeps full control of the header padding that a type's
// name, doc, fields and methods need.
//
// Between aggregates there are edges, ranks and crossings to worry about,
// which is exactly what dagre is for.

import dagre from "@dagrejs/dagre";
import type { Aggregate, Graph, Member, Service } from "./model";
import {
  aggCollapsedSize,
  aggHeadHeight,
  memberSize,
  serviceSize,
} from "./measure";

export interface PlacedMember {
  member: Member;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PlacedAggregate {
  aggregate: Aggregate;
  x: number;
  y: number;
  width: number;
  height: number;
  open: boolean;
  members: PlacedMember[];
}

export interface PlacedService {
  service: Service;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PlacedEdge {
  from: string;
  to: string;
  via: string;
  /** A service's link to what it works on, drawn differently. */
  service?: boolean;
  points: { x: number; y: number }[];
}

export interface Layout {
  width: number;
  height: number;
  aggregates: PlacedAggregate[];
  services: PlacedService[];
  edges: PlacedEdge[];
}

const PAD = 12;
const GAP = 12;
/** Rows grow until the box is about this much wider than it is tall. */
const ASPECT = 1.7;

/**
 * Pack member boxes into rows, and return the size of the box holding them.
 *
 * The width is chosen first, from the total area and the target aspect ratio,
 * then rows are filled left to right. It is not optimal packing, but the
 * boxes are similar in size and it reads as a tidy grid.
 */
function packMembers(
  members: Member[],
  headHeight: number,
  minWidth: number
): { width: number; height: number; placed: PlacedMember[] } {
  const sized = members.map((m) => ({ member: m, ...memberSize(m) }));

  let area = 0;
  let widest = 0;
  for (const s of sized) {
    area += (s.width + GAP) * (s.height + GAP);
    widest = Math.max(widest, s.width);
  }

  const target = Math.max(
    minWidth,
    widest + PAD * 2,
    Math.ceil(Math.sqrt(area * ASPECT)) + PAD * 2
  );

  const placed: PlacedMember[] = [];
  let x = PAD;
  let y = headHeight;
  let rowHeight = 0;
  let used = 0;

  for (const s of sized) {
    if (x > PAD && x + s.width + PAD > target) {
      x = PAD;
      y += rowHeight + GAP;
      rowHeight = 0;
    }
    placed.push({ member: s.member, x, y, width: s.width, height: s.height });
    x += s.width + GAP;
    rowHeight = Math.max(rowHeight, s.height);
    used = Math.max(used, x - GAP + PAD);
  }

  return {
    width: Math.max(minWidth, used),
    height: y + rowHeight + PAD,
    placed,
  };
}

/** Run the whole layout for the current expansion state. */
export function layoutGraph(
  graph: Graph,
  expanded: Set<string>,
  showServices = false
): Layout {
  const g = new dagre.graphlib.Graph({ multigraph: true });
  g.setGraph({
    rankdir: "LR",
    nodesep: 45,
    ranksep: 80,
    marginx: 30,
    marginy: 30,
  });
  g.setDefaultEdgeLabel(() => ({}));

  const packs = new Map<string, PlacedMember[]>();

  for (const agg of graph.aggregates) {
    const open = expanded.has(agg.name) && agg.members.length > 0;
    if (open) {
      const head = aggHeadHeight(agg);
      const collapsed = aggCollapsedSize(agg);
      const packed = packMembers(agg.members, head + 6, collapsed.width);
      packs.set(agg.name, packed.placed);
      g.setNode(agg.name, { width: packed.width, height: packed.height });
    } else {
      const size = aggCollapsedSize(agg);
      packs.set(agg.name, []);
      g.setNode(agg.name, { width: size.width, height: size.height });
    }
  }

  graph.references.forEach((r, i) => {
    if (r.from === r.to) return; // dagre cannot rank a self-loop usefully
    g.setEdge(r.from, r.to, {}, `ref${i}`);
  });

  // Services rank ahead of what they work on, so they gather on the side the
  // arrows leave from.
  const services = showServices ? (graph.services ?? []) : [];
  const aggNames = new Set(graph.aggregates.map((a) => a.name));
  services.forEach((s) => {
    const size = serviceSize(s);
    g.setNode(svcId(s.name), { width: size.width, height: size.height });
    s.touches.forEach((t, i) => {
      if (aggNames.has(t)) g.setEdge(svcId(s.name), t, {}, `svc${s.name}${i}`);
    });
  });

  dagre.layout(g);

  // dagre reports node centres; everything downstream wants top-left.
  const aggregates: PlacedAggregate[] = graph.aggregates.map((agg) => {
    const n = g.node(agg.name) as
      | { x: number; y: number; width: number; height: number }
      | undefined;
    const width = n?.width ?? 0;
    const height = n?.height ?? 0;
    return {
      aggregate: agg,
      x: (n?.x ?? 0) - width / 2,
      y: (n?.y ?? 0) - height / 2,
      width,
      height,
      open: expanded.has(agg.name) && agg.members.length > 0,
      members: packs.get(agg.name) ?? [],
    };
  });

  const placedServices: PlacedService[] = services.map((s) => {
    const n = g.node(svcId(s.name)) as
      | { x: number; y: number; width: number; height: number }
      | undefined;
    const width = n?.width ?? 0;
    const height = n?.height ?? 0;
    return {
      service: s,
      x: (n?.x ?? 0) - width / 2,
      y: (n?.y ?? 0) - height / 2,
      width,
      height,
    };
  });

  const edges: PlacedEdge[] = [];
  graph.references.forEach((r, i) => {
    if (r.from === r.to) return;
    const e = g.edge({ v: r.from, w: r.to, name: `ref${i}` }) as
      | { points?: { x: number; y: number }[] }
      | undefined;
    if (!e?.points?.length) return;
    edges.push({
      from: r.from,
      to: r.to,
      via: r.via,
      points: orthogonal(e.points),
    });
  });

  services.forEach((s) => {
    s.touches.forEach((t, i) => {
      if (!aggNames.has(t)) return;
      const e = g.edge({ v: svcId(s.name), w: t, name: `svc${s.name}${i}` }) as
        | { points?: { x: number; y: number }[] }
        | undefined;
      if (!e?.points?.length) return;
      edges.push({
        from: svcId(s.name),
        to: t,
        via: "",
        service: true,
        points: orthogonal(e.points),
      });
    });
  });

  const gl = g.graph() as { width?: number; height?: number };
  return {
    width: gl.width ?? 0,
    height: gl.height ?? 0,
    aggregates,
    services: placedServices,
    edges,
  };
}

/** Service node ids are prefixed so they cannot collide with a type name. */
function svcId(name: string): string {
  return `svc:${name}`;
}

/**
 * Turn dagre's spline control points into right-angled segments.
 *
 * dagre hands back a smooth path; the diagram reads better with square
 * corners, and squaring it here keeps the drawing code free of geometry.
 * The turn is made half way along, which keeps edges out of the boxes.
 */
function orthogonal(points: { x: number; y: number }[]): { x: number; y: number }[] {
  const first = points[0];
  const last = points[points.length - 1];
  if (!first || !last) return points;

  if (Math.abs(first.y - last.y) < 1) {
    return [first, last];
  }

  const midX = (first.x + last.x) / 2;
  return [
    first,
    { x: midX, y: first.y },
    { x: midX, y: last.y },
    last,
  ];
}
