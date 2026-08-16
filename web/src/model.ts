// Mirrors internal/model/model.go. When a field changes on the Go side,
// change it here too and `npm run check` will point at everything that
// needs updating.

export type Kind = "entity" | "vo";

export type UnclassifiedKind =
  | "interface"
  | "service"
  | "data"
  | "value"
  | "other";

export interface Field {
  name: string;
  type: string;
}

export interface EnumValue {
  name: string;
  value?: string;
  doc?: string;
}

export interface Invariant {
  text: string;
  pos: string;
  from?: string;
}

export interface Method {
  name: string;
  signature: string;
  doc?: string;
  pointer?: boolean;
}

export interface Member {
  name: string;
  pkg: string;
  pos: string;
  kind: Kind;
  fields: Field[];
  methods?: Method[];
  values?: EnumValue[];
  invariants?: Invariant[];
  doc?: string;
  depth: number;
}

export interface Aggregate {
  name: string;
  pkg: string;
  pos: string;
  idType?: string;
  members: Member[];
  fields: Field[];
  methods?: Method[];
  values?: EnumValue[];
  invariants?: Invariant[];
  doc?: string;
}

export interface Reference {
  from: string;
  to: string;
  via: string;
}

export interface UnclassifiedRef {
  name: string;
  pkg: string;
  pos: string;
  kind: UnclassifiedKind;
  touches?: string[];
  depth: number;
}

export interface Unclassified {
  name: string;
  pkg: string;
  pos: string;
  kind: UnclassifiedKind;
  touches?: string[];
  members?: UnclassifiedRef[];
}

export interface Candidate {
  name: string;
  pkg: string;
  pos: string;
  idType: string;
}

export interface Meta {
  title: string;
  packages: string[];
}

export interface Service {
  name: string;
  pkg: string;
  pos: string;
  kind: UnclassifiedKind;
  doc?: string;
  touches: string[];
  methods?: Method[];
}

export interface Graph {
  meta: Meta;
  aggregates: Aggregate[];
  references: Reference[];
  services?: Service[];
  unclassified: Unclassified[];
  unclassifiedTotal?: number;
  candidates?: Candidate[];
}

/** Anything drawn as a box: an aggregate root or one of its members. */
export type BoxLike = Pick<
  Aggregate,
  "fields" | "methods" | "values" | "invariants" | "doc"
>;

declare global {
  interface Window {
    __DDDVIZ__: Graph;
    __DDDVIZ_LIVE__: boolean;
  }
}
