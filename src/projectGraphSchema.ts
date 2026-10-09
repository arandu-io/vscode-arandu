import contract from "./projectGraphContract.json";

export type DiagnosticLevel = "warning" | "error";

export type ProjectGraphSchemaVersion = 1 | 2;

export interface ProjectGraphGroup {
  readonly id: string;
  readonly label: string;
  readonly nodeIds: readonly string[];
}

// ProjectGraphLocation is a place in a file, in zero-based UTF-16 positions.
export interface ProjectGraphLocation {
  readonly file: string;
  readonly line: number;
  readonly column: number;
  readonly endLine: number;
  readonly endColumn: number;
}

// ProjectGraphNode is one node of either schema. The first schema carries a
// start position and a diagnostic level; the second always carries the whole
// range, calls the level a severity, and adds what is read from the code:
// the feature a node belongs to, whether a tool generated it, the shape of a
// controller, and the method, pattern and name of a route. Both levels are
// kept in level, so the rest of the extension reads one field.
export interface ProjectGraphNode {
  readonly id: string;
  readonly kind: string;
  readonly label: string;
  readonly detail?: string;
  readonly file?: string;
  readonly line?: number;
  readonly column?: number;
  readonly endLine?: number;
  readonly endColumn?: number;
  readonly level?: DiagnosticLevel;
  readonly feature?: string;
  readonly generated?: boolean;
  readonly variant?: string;
  readonly nestedUnder?: string;
  readonly parent?: string;
  readonly method?: string;
  readonly pattern?: string;
  readonly name?: string;
  readonly rule?: string;
  readonly ruleDoc?: string;
}

// ProjectGraphEdge is one directed relationship. The first schema only has
// containment; the second has typed edges, each with the place in the code it
// was read from, except containment, which is written nowhere.
export interface ProjectGraphEdge {
  readonly from: string;
  readonly to: string;
  readonly kind: string;
  readonly at?: ProjectGraphLocation;
}

// ProjectGraphEdgeKind is the server's own description of an edge kind.
export interface ProjectGraphEdgeKind {
  readonly kind: string;
  readonly meaning: string;
  readonly follows?: string;
  readonly doesNotFollow?: string;
}

export interface ProjectGraph {
  readonly schemaVersion: ProjectGraphSchemaVersion;
  readonly profile?: string;
  readonly groups: readonly ProjectGraphGroup[];
  readonly nodes: readonly ProjectGraphNode[];
  readonly edges: readonly ProjectGraphEdge[];
  readonly edgeKinds: readonly ProjectGraphEdgeKind[];
}

export class ProjectGraphContractError extends Error {
  public constructor(message: string) {
    super(message);
    this.name = "ProjectGraphContractError";
  }
}

// parseProjectGraph reads the answer to arandu/projectGraph for the schema
// that was asked for. A server answering another schema than the one asked
// is refused rather than guessed at.
export function parseProjectGraph(value: unknown, expected: ProjectGraphSchemaVersion = 1): ProjectGraph {
  const graph = expectRecord(value, "project graph");
  if (graph.schemaVersion !== expected) {
    throw new ProjectGraphContractError(
      `Unsupported Arandu project graph schema ${String(graph.schemaVersion)}; expected ${expected}.`,
    );
  }
  return expected === contract.typedSchemaVersion ? parseTypedGraph(graph) : parseFirstSchema(graph);
}

// parseFirstSchema is the reading of schema 1 the extension has always made:
// the ten groups in their fixed order and containment as the only edge.
function parseFirstSchema(graph: Record<string, unknown>): ProjectGraph {
  if (graph.schemaVersion !== contract.schemaVersion) {
    throw new ProjectGraphContractError(
      `Unsupported Arandu project graph schema ${String(graph.schemaVersion)}; expected ${contract.schemaVersion}.`,
    );
  }

  const groups = expectArray(graph.groups, "project graph groups").map((raw, index) => {
    const group = expectRecord(raw, `project graph group ${index}`);
    const expected = contract.groups[index];
    const id = expectString(group.id, `project graph group ${index} id`);
    const label = expectString(group.label, `project graph group ${index} label`);
    if (expected === undefined || id !== expected.id || label !== expected.label) {
      throw new ProjectGraphContractError(
        `Unexpected project graph group ${index}: ${id} (${label}).`,
      );
    }
    return {
      id,
      label,
      nodeIds: expectArray(group.nodeIds, `project graph group ${id} nodeIds`).map((nodeID) =>
        expectString(nodeID, `project graph group ${id} node id`),
      ),
    };
  });
  if (groups.length !== contract.groups.length) {
    throw new ProjectGraphContractError(
      `Project graph has ${groups.length} groups; expected ${contract.groups.length}.`,
    );
  }

  const nodes = expectArray(graph.nodes, "project graph nodes").map((raw, index) => parseNode(raw, index, 1));
  const nodeIDs = uniqueNodeIDs(nodes);
  for (const group of groups) {
    for (const nodeID of group.nodeIds) {
      expectNodeReference(nodeIDs, nodeID, `group ${group.id}`);
    }
  }

  const edges = expectArray(graph.edges, "project graph edges").map((raw, index) => {
    const edge = expectRecord(raw, `project graph edge ${index}`);
    const from = expectString(edge.from, `project graph edge ${index} from`);
    const to = expectString(edge.to, `project graph edge ${index} to`);
    if (edge.kind !== "contains") {
      throw new ProjectGraphContractError(
        `Unsupported project graph edge kind ${String(edge.kind)}; expected contains.`,
      );
    }
    expectNodeReference(nodeIDs, from, `edge ${index} source`);
    expectNodeReference(nodeIDs, to, `edge ${index} target`);
    return { from, to, kind: "contains" };
  });

  return { schemaVersion: 1, groups, nodes, edges, edgeKinds: [] };
}

// parseTypedGraph reads schema 2. The groups are whatever the server lists,
// in its order: their number is the server's to change. The structure is
// checked as strictly as schema 1 -- every reference resolves, every range is
// a position -- while the vocabulary of node and edge kinds is left open, so a
// kind this extension has not heard of is shown rather than refused.
function parseTypedGraph(graph: Record<string, unknown>): ProjectGraph {
  const profile = expectString(graph.profile, "project graph profile");

  const groupIDs = new Set<string>();
  const groups = expectArray(graph.groups, "project graph groups").map((raw, index) => {
    const group = expectRecord(raw, `project graph group ${index}`);
    const id = expectString(group.id, `project graph group ${index} id`);
    const label = expectString(group.label, `project graph group ${index} label`);
    if (groupIDs.has(id)) {
      throw new ProjectGraphContractError(`Project graph repeats group ${id}.`);
    }
    groupIDs.add(id);
    return {
      id,
      label,
      nodeIds: expectArray(group.nodeIds, `project graph group ${id} nodeIds`).map((nodeID) =>
        expectString(nodeID, `project graph group ${id} node id`),
      ),
    };
  });

  const nodes = expectArray(graph.nodes, "project graph nodes").map((raw, index) => parseNode(raw, index, 2));
  const nodeIDs = uniqueNodeIDs(nodes);
  for (const group of groups) {
    for (const nodeID of group.nodeIds) {
      expectNodeReference(nodeIDs, nodeID, `group ${group.id}`);
    }
  }

  const edges = expectArray(graph.edges, "project graph edges").map((raw, index) => {
    const edge = expectRecord(raw, `project graph edge ${index}`);
    const from = expectString(edge.from, `project graph edge ${index} from`);
    const to = expectString(edge.to, `project graph edge ${index} to`);
    const kind = expectString(edge.kind, `project graph edge ${index} kind`);
    expectNodeReference(nodeIDs, from, `edge ${index} source`);
    expectNodeReference(nodeIDs, to, `edge ${index} target`);
    if (edge.at === undefined) {
      return { from, to, kind };
    }
    return { from, to, kind, at: parseLocation(edge.at, `project graph edge ${index} at`) };
  });

  const kinds = new Set<string>();
  const edgeKinds = expectArray(graph.edgeKinds, "project graph edgeKinds").map((raw, index) => {
    const entry = expectRecord(raw, `project graph edge kind ${index}`);
    const kind = expectString(entry.kind, `project graph edge kind ${index} kind`);
    if (kinds.has(kind)) {
      throw new ProjectGraphContractError(`Project graph describes edge kind ${kind} twice.`);
    }
    kinds.add(kind);
    const parsed: { kind: string; meaning: string; follows?: string; doesNotFollow?: string } = {
      kind,
      meaning: expectString(entry.meaning, `project graph edge kind ${kind} meaning`),
    };
    if (entry.follows !== undefined && entry.follows !== "") {
      parsed.follows = expectString(entry.follows, `project graph edge kind ${kind} follows`);
    }
    if (entry.doesNotFollow !== undefined && entry.doesNotFollow !== "") {
      parsed.doesNotFollow = expectString(entry.doesNotFollow, `project graph edge kind ${kind} doesNotFollow`);
    }
    return parsed;
  });

  return { schemaVersion: 2, profile, groups, nodes, edges, edgeKinds };
}

type MutableNode = { -readonly [K in keyof ProjectGraphNode]: ProjectGraphNode[K] };

function parseNode(raw: unknown, index: number, schema: ProjectGraphSchemaVersion): ProjectGraphNode {
  const node = expectRecord(raw, `project graph node ${index}`);
  const parsed: MutableNode = {
    id: expectString(node.id, `project graph node ${index} id`),
    kind: expectString(node.kind, `project graph node ${index} kind`),
    label: expectString(node.label, `project graph node ${index} label`),
  };

  if (node.detail !== undefined && node.detail !== "") {
    parsed.detail = expectString(node.detail, `project graph node ${parsed.id} detail`);
  }
  if (node.file !== undefined && node.file !== "") {
    parsed.file = expectString(node.file, `project graph node ${parsed.id} file`);
    if (!isURI(parsed.file)) {
      throw new ProjectGraphContractError(`Project graph node ${parsed.id} has a non-URI file.`);
    }
  }
  if (schema === 1) {
    if (node.line !== undefined) {
      parsed.line = expectPosition(node.line, `project graph node ${parsed.id} line`);
    }
    if (node.column !== undefined) {
      parsed.column = expectPosition(node.column, `project graph node ${parsed.id} column`);
    }
    if (node.level !== undefined && node.level !== "") {
      parsed.level = expectLevel(node.level, `Project graph node ${parsed.id} has unsupported diagnostic level`);
    }
    return parsed;
  }

  parsed.line = expectPosition(node.line, `project graph node ${parsed.id} line`);
  parsed.column = expectPosition(node.column, `project graph node ${parsed.id} column`);
  parsed.endLine = expectPosition(node.endLine, `project graph node ${parsed.id} endLine`);
  parsed.endColumn = expectPosition(node.endColumn, `project graph node ${parsed.id} endColumn`);
  if (node.severity !== undefined && node.severity !== "") {
    parsed.level = expectLevel(node.severity, `Project graph node ${parsed.id} has unsupported severity`);
  }
  if (node.generated !== undefined) {
    if (typeof node.generated !== "boolean") {
      throw new ProjectGraphContractError(`project graph node ${parsed.id} generated must be a boolean.`);
    }
    if (node.generated) {
      parsed.generated = true;
    }
  }
  for (const key of ["feature", "variant", "nestedUnder", "parent", "method", "pattern", "name", "rule"] as const) {
    const field = node[key];
    if (field !== undefined && field !== "") {
      parsed[key] = expectString(field, `project graph node ${parsed.id} ${key}`);
    }
  }
  if (node.ruleDoc !== undefined && node.ruleDoc !== "") {
    parsed.ruleDoc = expectString(node.ruleDoc, `project graph node ${parsed.id} ruleDoc`);
  }
  return parsed;
}

function parseLocation(value: unknown, label: string): ProjectGraphLocation {
  const location = expectRecord(value, label);
  return {
    file: expectURI(location.file, `${label} file`),
    line: expectPosition(location.line, `${label} line`),
    column: expectPosition(location.column, `${label} column`),
    endLine: expectPosition(location.endLine, `${label} endLine`),
    endColumn: expectPosition(location.endColumn, `${label} endColumn`),
  };
}

function uniqueNodeIDs(nodes: readonly ProjectGraphNode[]): ReadonlySet<string> {
  const nodeIDs = new Set<string>();
  for (const node of nodes) {
    if (nodeIDs.has(node.id)) {
      throw new ProjectGraphContractError(`Project graph repeats node ${node.id}.`);
    }
    nodeIDs.add(node.id);
  }
  return nodeIDs;
}

function expectLevel(value: unknown, message: string): DiagnosticLevel {
  if (value !== "warning" && value !== "error") {
    throw new ProjectGraphContractError(`${message} ${String(value)}.`);
  }
  return value;
}

function expectURI(value: unknown, label: string): string {
  const uri = expectString(value, label);
  if (!isURI(uri)) {
    throw new ProjectGraphContractError(`${label} is not a URI.`);
  }
  return uri;
}

function isURI(value: string): boolean {
  return /^[A-Za-z][A-Za-z\d+.-]*:/.test(value);
}

function expectRecord(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new ProjectGraphContractError(`${label} must be an object.`);
  }
  return value as Record<string, unknown>;
}

function expectArray(value: unknown, label: string): readonly unknown[] {
  if (!Array.isArray(value)) {
    throw new ProjectGraphContractError(`${label} must be an array.`);
  }
  return value;
}

function expectString(value: unknown, label: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new ProjectGraphContractError(`${label} must be a non-empty string.`);
  }
  return value;
}

function expectPosition(value: unknown, label: string): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) {
    throw new ProjectGraphContractError(`${label} must be a zero-based integer.`);
  }
  return value;
}

function expectNodeReference(nodeIDs: ReadonlySet<string>, nodeID: string, label: string): void {
  if (!nodeIDs.has(nodeID)) {
    throw new ProjectGraphContractError(`${label} references unknown node ${nodeID}.`);
  }
}
