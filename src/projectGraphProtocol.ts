import catalogContract from "./catalogContract.json";
import contract from "./projectGraphContract.json";
import type { DiagnosticLevel, ProjectGraph, ProjectGraphSchemaVersion } from "./projectGraphSchema";

// ServerFeatures is what the running aru said it answers, read from the
// experimental capabilities of its initialize result. An aru that says
// nothing is an aru from before the typed map, and gets exactly the requests
// the extension has always sent.
export interface ServerFeatures {
  readonly projectGraphSchema: ProjectGraphSchemaVersion;
  readonly catalog: boolean;
  readonly doctorDiagnostics: boolean;
}

export const firstSchemaServer: ServerFeatures = {
  projectGraphSchema: 1,
  catalog: false,
  doctorDiagnostics: false,
};

// initializationOptions asks the server to publish the doctor's findings
// itself. It is sent to every aru: one from before the option ignores it and
// answers without the capability, which is how the extension knows to keep
// publishing them from the map.
export const initializationOptions: Readonly<Record<string, boolean>> = {
  [contract.doctorDiagnosticsOption]: true,
};

export function readServerFeatures(capabilities: unknown): ServerFeatures {
  if (!isRecord(capabilities) || !isRecord(capabilities.experimental)) {
    return firstSchemaServer;
  }
  const experimental = capabilities.experimental;
  const schemas = experimental[contract.schemasCapability];
  const typed = Array.isArray(schemas) && schemas.includes(contract.typedSchemaVersion);
  return {
    projectGraphSchema: typed ? 2 : 1,
    catalog: experimental[catalogContract.capability] === true,
    doctorDiagnostics: experimental[contract.doctorDiagnosticsCapability] === true,
  };
}

// projectGraphParams is what arandu/projectGraph is asked with: the typed
// schema when the server lists it, and no parameters at all otherwise, which
// is the request every aru has always answered with schema 1.
export function projectGraphParams(features: ServerFeatures): { readonly schemaVersion: 2 } | undefined {
  return features.projectGraphSchema === 2 ? { schemaVersion: 2 } : undefined;
}

// DoctorFinding is one doctor diagnostic the extension draws itself.
export interface DoctorFinding {
  readonly file: string;
  readonly line: number;
  readonly column: number;
  readonly endLine: number;
  readonly endColumn: number;
  readonly message: string;
  readonly level: DiagnosticLevel;
  readonly code: string;
  readonly codeHref?: string;
}

// doctorFindings is the extension's own copy of the doctor's diagnostics,
// read from the map's diagnostics group. It is empty when the server already
// publishes them, because then the same finding would appear twice in
// Problems: once from the server and once from here.
export function doctorFindings(graph: ProjectGraph, features: ServerFeatures): DoctorFinding[] {
  if (features.doctorDiagnostics) {
    return [];
  }
  const diagnosticsGroup = graph.groups.find((group) => group.id === "diagnostics");
  if (diagnosticsGroup === undefined) {
    return [];
  }
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
  const findings: DoctorFinding[] = [];
  for (const nodeID of diagnosticsGroup.nodeIds) {
    const node = nodes.get(nodeID);
    if (node?.file === undefined) {
      continue;
    }
    const line = node.line ?? 0;
    const column = node.column ?? 0;
    const ranged = node.endLine !== undefined && node.endColumn !== undefined
      && (node.endLine > line || (node.endLine === line && node.endColumn > column));
    const finding: { -readonly [K in keyof DoctorFinding]: DoctorFinding[K] } = {
      file: node.file,
      line,
      column,
      endLine: ranged ? node.endLine ?? line : line,
      endColumn: ranged ? node.endColumn ?? column + 1 : column + 1,
      message: node.detail === undefined ? node.label : `${node.label}: ${node.detail}`,
      level: node.level === "error" ? "error" : "warning",
      code: node.rule ?? node.kind,
    };
    if (node.ruleDoc !== undefined) {
      finding.codeHref = node.ruleDoc;
    }
    findings.push(finding);
  }
  return findings;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
