// Box sizes are computed from character counts rather than measured in the
// DOM, because the layout has to be decided before anything is drawn.
// The font is monospace, so counting is close enough.

import type { Aggregate, BoxLike, Field, Member, Method } from "./model";

export const CHAR = 6.7; // width of the 11px monospace face
export const TITLE_CHAR = 7.4; // 12-13px sans-serif
export const ROW = 16;
export const DOC_ROW = 15;
export const HEAD = 24;
export const PAD = 9;
export const MIN_W = 130;

/** CJK characters occupy roughly two columns, and doc comments carry them. */
export function visualLen(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    n += c > 0x1100 && !(c >= 0x2000 && c < 0x2c00) ? 2 : 1;
  }
  return n;
}

function fieldWidth(f: Field): number {
  return (f.name.length + f.type.length + 2) * CHAR;
}

function methodWidth(m: Method): number {
  return (m.name.length + m.signature.length + 2) * CHAR;
}

function widest<T>(items: T[], measure: (t: T) => number): number {
  let w = 0;
  for (const item of items) w = Math.max(w, measure(item));
  return w;
}

/**
 * Cut a doc comment to the width the box already has.
 *
 * The doc never widens a box on its own — a long sentence would make one
 * type dwarf everything around it, and the full text is a hover away.
 */
export function docLine(doc: string, boxChars: number): string {
  const first = doc.split("\n")[0] ?? "";
  if (visualLen(first) <= boxChars) return first;

  let out = "";
  let used = 0;
  for (const ch of first) {
    const add = ch.charCodeAt(0) > 0x1100 ? 2 : 1;
    if (used + add > boxChars - 1) break;
    out += ch;
    used += add;
  }
  return out + "…";
}

export function bodyHeight(t: BoxLike): number {
  let h = 0;
  if (t.doc) h += DOC_ROW + 4;
  if (t.fields.length) h += t.fields.length * ROW + 4;
  if (t.methods?.length) h += t.methods.length * ROW + 8;
  return h;
}

export function bodyWidth(t: BoxLike): number {
  return Math.max(
    widest(t.fields, fieldWidth),
    widest(t.methods ?? [], methodWidth)
  );
}

export function memberSize(m: Member): { width: number; height: number } {
  const title = (m.name.length + m.kind.length + 3) * TITLE_CHAR;
  const w = Math.max(title, bodyWidth(m)) + PAD * 2;
  const h = HEAD + bodyHeight(m) + 6;
  return { width: Math.max(MIN_W, Math.ceil(w)), height: Math.ceil(h) };
}

/** The header holds the name, the doc line, and the root's own members. */
export function aggHeadHeight(agg: Aggregate): number {
  return HEAD + bodyHeight(agg) + 6;
}

export function aggCollapsedSize(agg: Aggregate): {
  width: number;
  height: number;
} {
  const title = (agg.name.length + (agg.idType ?? "").length + 4) * TITLE_CHAR;
  const w = Math.max(title, bodyWidth(agg)) + PAD * 2;
  const countChars = agg.members.length
    ? `${agg.members.length} types ▸`.length
    : 0;
  return {
    width: Math.max(MIN_W + 30, Math.ceil(w), countChars * CHAR + PAD * 2),
    height: aggHeadHeight(agg) + (agg.members.length ? ROW + 4 : 0),
  };
}
