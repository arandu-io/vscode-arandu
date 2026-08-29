import contract from "./projectGraphContract.json";

export type DiagnosticLevel = "warning" | "error";

export interface ProjectGraphGroup {
  readonly id: string;
  readonly label: string;
  readonly nodeIds: readonly string[];
}

export interface ProjectGraphNode {
  readonly id: string;
  readonly kind: string;
  readonly label: string;
  readonly detail?: string;
  readonly file?: string;
  readonly line?: number;
  readonly column?: number;
  readonly level?: DiagnosticLevel;
}

export interface ProjectGraphEdge {
  readonly from: string;
  readonly to: string;
  readonly kind: "contains";
}

export interface ProjectGraph {
  readonly schemaVersion: 1;
  readonly groups: readonly ProjectGraphGroup[];
  readonly nodes: readonly ProjectGraphNode[];
  readonly edges: readonly ProjectGraphEdge[];
}

export class ProjectGraphContractError extends Error {
  public constructor(message: string) {
    super(message);
    this.name = "ProjectGraphContractError";
  }
}

export function parseProjectGraph(value: unknown): ProjectGraph {
  const graph = expectRecord(value, "project graph");
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

  const nodes = expectArray(graph.nodes, "project graph nodes").map(parseNode);
  const nodeIDs = new Set<string>();
  for (const node of nodes) {
    if (nodeIDs.has(node.id)) {
      throw new ProjectGraphContractError(`Project graph repeats node ${node.id}.`);
    }
    nodeIDs.add(node.id);
  }
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
    return { from, to, kind: "contains" as const };
  });

  return { schemaVersion: 1, groups, nodes, edges };
}

function parseNode(raw: unknown, index: number): ProjectGraphNode {
  const node = expectRecord(raw, `project graph node ${index}`);
  const parsed: {
    id: string;
    kind: string;
    label: string;
    detail?: string;
    file?: string;
    line?: number;
    column?: number;
    level?: DiagnosticLevel;
  } = {
    id: expectString(node.id, `project graph node ${index} id`),
    kind: expectString(node.kind, `project graph node ${index} kind`),
    label: expectString(node.label, `project graph node ${index} label`),
  };

  if (node.detail !== undefined && node.detail !== "") {
    parsed.detail = expectString(node.detail, `project graph node ${parsed.id} detail`);
  }
  if (node.file !== undefined && node.file !== "") {
    parsed.file = expectString(node.file, `project graph node ${parsed.id} file`);
    if (!/^[A-Za-z][A-Za-z\d+.-]*:/.test(parsed.file)) {
      throw new ProjectGraphContractError(`Project graph node ${parsed.id} has a non-URI file.`);
    }
  }
  if (node.line !== undefined) {
    parsed.line = expectPosition(node.line, `project graph node ${parsed.id} line`);
  }
  if (node.column !== undefined) {
    parsed.column = expectPosition(node.column, `project graph node ${parsed.id} column`);
  }
  if (node.level !== undefined && node.level !== "") {
    if (node.level !== "warning" && node.level !== "error") {
      throw new ProjectGraphContractError(
        `Project graph node ${parsed.id} has unsupported diagnostic level ${String(node.level)}.`,
      );
    }
    parsed.level = node.level;
  }
  return parsed;
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
