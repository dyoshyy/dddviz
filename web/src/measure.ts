// Box sizes are computed from character counts rather than measured in the
// DOM, because the layout has to be decided before anything is drawn.
// The font is monospace, so counting is close enough.

import type {
  Aggregate,
  BoxLike,
  EnumValue,
  Field,
  Invariant,
  Member,
  Method,
} from "./model";

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

/** How many characters of text fit inside a box of this width. */
export function boxChars(boxWidth: number): number {
  return Math.max(8, Math.floor((boxWidth - PAD * 2) / CHAR));
}

/**
 * Pack the constant names into lines that fit the box.
 *
 * The values are read as a set, so they are run together on as few lines as
 * possible rather than given a line each.
 */
export function wrapValues(values: EnumValue[], chars: number): string[] {
  const parts = values.map((v) => (v.value ? `${v.name}=${v.value}` : v.name));
  const lines: string[] = [];
  let cur = "";

  for (const p of parts) {
    const next = cur ? `${cur} · ${p}` : p;
    if (cur && visualLen(next) > chars) {
      lines.push(cur);
      cur = p;
    } else {
      cur = next;
    }
  }
  if (cur) lines.push(cur);
  return lines;
}

/**
 * Break one rule across as many lines as it needs.
 *
 * Rules are sentences, often in a language without spaces, so they wrap on
 * character count rather than on word boundaries.
 */
export function wrapText(text: string, chars: number): string[] {
  const lines: string[] = [];
  let cur = "";
  let used = 0;

  for (const ch of text) {
    const add = ch.charCodeAt(0) > 0x1100 ? 2 : 1;
    if (used + add > chars && cur) {
      lines.push(cur);
      cur = "";
      used = 0;
    }
    cur += ch;
    used += add;
  }
  if (cur) lines.push(cur);
  return lines;
}

/** Every rule of a type, already broken into lines. */
export function wrapInvariants(inv: Invariant[], chars: number): string[][] {
  // The bullet and its indent are part of the line budget.
  return inv.map((i) => wrapText(i.text, Math.max(8, chars - 2)));
}

export function bodyHeight(t: BoxLike, boxWidth: number): number {
  let h = 0;
  if (t.doc) h += DOC_ROW + 4;
  if (t.fields.length) h += t.fields.length * ROW + 4;
  if (t.methods?.length) h += t.methods.length * ROW + 8;
  if (t.values?.length) {
    h += wrapValues(t.values, boxChars(boxWidth)).length * ROW + 8;
  }
  if (t.invariants?.length) {
    const lines = wrapInvariants(t.invariants, boxChars(boxWidth));
    h += lines.reduce((n, l) => n + l.length, 0) * ROW + 10;
  }
  return h;
}

/** Width a type with several constants wants, so the list is not a column. */
const VALUES_CHARS = 42;
const VALUES_MIN_COUNT = 4;
/** Rules are sentences; below this they wrap into an unreadable ribbon. */
const RULES_CHARS = 40;

export function bodyWidth(t: BoxLike): number {
  let w = Math.max(
    widest(t.fields, fieldWidth),
    widest(t.methods ?? [], methodWidth)
  );
  if (t.values && t.values.length >= VALUES_MIN_COUNT) {
    w = Math.max(w, VALUES_CHARS * CHAR);
  } else if (t.values?.length) {
    w = Math.max(w, widest(t.values, (v) => (v.name.length + 3) * CHAR));
  }
  if (t.invariants?.length) {
    w = Math.max(w, RULES_CHARS * CHAR);
  }
  return w;
}

export function memberSize(m: Member): { width: number; height: number } {
  const title = (m.name.length + m.kind.length + 3) * TITLE_CHAR;
  const width = Math.max(MIN_W, Math.ceil(Math.max(title, bodyWidth(m)) + PAD * 2));
  const height = Math.ceil(HEAD + bodyHeight(m, width) + 6);
  return { width, height };
}

/** The header holds the name, the doc line, and the root's own members. */
export function aggHeadHeight(agg: Aggregate, boxWidth?: number): number {
  return HEAD + bodyHeight(agg, boxWidth ?? aggCollapsedSize(agg).width) + 6;
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
  const width = Math.max(MIN_W + 30, Math.ceil(w), countChars * CHAR + PAD * 2);
  return {
    width,
    height:
      HEAD + bodyHeight(agg, width) + 6 + (agg.members.length ? ROW + 4 : 0),
  };
}
