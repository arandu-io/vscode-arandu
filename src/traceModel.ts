import * as path from "node:path";
import contract from "./testingContract.json";

// The trace an end-to-end test leaves behind, as trace.json writes it. Every
// driver writes the same document, so one reader serves all of them.

export interface TraceStep {
  readonly index: number;
  readonly action: string;
  readonly target: string;
  readonly url: string;
  readonly started: number | undefined;
  readonly durationMs: number;
  readonly error: string;
  readonly screenshot: string;
  readonly dom: string;
}

export interface TraceRequest {
  readonly id: string;
  readonly method: string;
  readonly url: string;
  readonly type: string;
  readonly status: number;
  readonly serverRequestId: string;
  readonly started: number | undefined;
  readonly finished: number | undefined;
  readonly error: string;
}

export interface TraceMessage {
  readonly time: number | undefined;
  readonly level: string;
  readonly source: string;
  readonly text: string;
  readonly url: string;
}

export interface TraceFailure {
  readonly time: number | undefined;
  readonly kind: string;
  readonly text: string;
}

export interface TraceLogLine {
  readonly time: number | undefined;
  readonly level: string;
  readonly message: string;
  readonly attrs: Readonly<Record<string, unknown>>;
}

export interface TraceQuery {
  readonly sql: string;
  readonly durationMs: number;
  readonly error: string;
}

export interface TraceDecision {
  readonly policy: string;
  readonly action: string;
  readonly allowed: boolean;
  readonly grant: string;
  readonly reason: string;
}

export interface TraceServer {
  readonly logs: readonly TraceLogLine[];
  readonly queries: readonly TraceQuery[];
  readonly decisions: readonly TraceDecision[];
}

export interface Trace {
  readonly version: number;
  readonly test: string;
  readonly driver: string;
  readonly browser: string;
  readonly started: number | undefined;
  readonly finished: number | undefined;
  readonly failed: boolean;
  readonly steps: readonly TraceStep[];
  readonly network: readonly TraceRequest[];
  readonly console: readonly TraceMessage[];
  readonly failures: readonly TraceFailure[];
  readonly server: ReadonlyMap<string, TraceServer>;
}

// parseTrace reads a decoded trace.json. A version it does not know is refused
// rather than read by guessing which fields moved.
export function parseTrace(raw: unknown): Trace {
  const doc = record(raw, "trace");
  if (doc.version !== contract.traceVersion) {
    throw new Error(`The trace has format version ${String(doc.version)}; this extension reads version ${contract.traceVersion}.`);
  }
  const server = new Map<string, TraceServer>();
  if (typeof doc.server === "object" && doc.server !== null) {
    for (const [id, value] of Object.entries(doc.server as Record<string, unknown>)) {
      if (typeof value !== "object" || value === null) {
        continue;
      }
      const section = value as Record<string, unknown>;
      server.set(id, {
        logs: list(section.logs).map((line) => ({
          time: time(line.time),
          level: text(line.level),
          message: text(line.message),
          attrs: typeof line.attrs === "object" && line.attrs !== null ? line.attrs as Record<string, unknown> : {},
        })),
        queries: list(section.queries).map((query) => ({
          sql: text(query.sql),
          durationMs: nanoseconds(query.duration_ns),
          error: text(query.error),
        })),
        decisions: list(section.decisions).map((decision) => ({
          policy: text(decision.policy),
          action: text(decision.action),
          allowed: decision.allowed === true,
          grant: text(decision.grant),
          reason: text(decision.reason),
        })),
      });
    }
  }
  return {
    version: contract.traceVersion,
    test: text(doc.test),
    driver: text(doc.driver),
    browser: text(doc.browser),
    started: time(doc.started),
    finished: time(doc.finished),
    failed: doc.failed === true,
    steps: list(doc.steps).map((step, position) => ({
      index: typeof step.index === "number" ? step.index : position + 1,
      action: text(step.action),
      target: text(step.target),
      url: text(step.url),
      started: time(step.started),
      durationMs: nanoseconds(step.duration_ns),
      error: text(step.error),
      screenshot: text(step.screenshot),
      dom: text(step.dom),
    })),
    network: list(doc.network).map((request) => ({
      id: text(request.id),
      method: text(request.method),
      url: text(request.url),
      type: text(request.type),
      status: typeof request.status === "number" ? request.status : 0,
      serverRequestId: text(request.server_request_id),
      started: time(request.started),
      finished: time(request.finished),
      error: text(request.error),
    })),
    console: list(doc.console).map((message) => ({
      time: time(message.time),
      level: text(message.level),
      source: text(message.source),
      text: text(message.text),
      url: text(message.url),
    })),
    failures: list(doc.failures).map((failure) => ({
      time: time(failure.time),
      kind: text(failure.kind),
      text: text(failure.text),
    })),
    server,
  };
}

// requestsByStep answers, for each step index, the positions in the network of
// the requests that started while it was the current step: from its start to
// the next step's, and to the end for the last one. A step's requests are the
// ones its action caused, including the htmx swaps that finish after it.
export function requestsByStep(trace: Trace): Map<number, number[]> {
  const out = new Map<number, number[]>();
  const steps = [...trace.steps].filter((step) => step.started !== undefined)
    .sort((left, right) => (left.started ?? 0) - (right.started ?? 0));
  for (const step of trace.steps) {
    out.set(step.index, []);
  }
  trace.network.forEach((request, position) => {
    if (request.started === undefined) {
      return;
    }
    let owner: TraceStep | undefined;
    for (const step of steps) {
      if ((step.started ?? 0) <= request.started) {
        owner = step;
      } else {
        break;
      }
    }
    if (owner !== undefined) {
      out.get(owner.index)?.push(position);
    }
  });
  return out;
}

// safeTracePath answers the absolute path of a file the trace names, or
// undefined when the name would leave the trace directory: a trace is a file
// on disk that anybody can write, and it never gets to point the viewer at
// another file of the machine.
export function safeTracePath(traceDir: string, relative: string): string | undefined {
  if (relative === "" || relative.includes("\\") || relative.includes("\0") || path.posix.isAbsolute(relative)) {
    return undefined;
  }
  const normalized = path.posix.normalize(relative);
  if (normalized === ".." || normalized.startsWith("../") || normalized === ".") {
    return undefined;
  }
  const root = path.resolve(traceDir);
  const resolved = path.resolve(root, ...normalized.split("/"));
  if (resolved !== root && resolved.startsWith(root + path.sep)) {
    return resolved;
  }
  return undefined;
}

// traceSlug keeps letters, digits, dot, dash and underscore, and turns every
// other run into a single dash: the same name the test kit gives a test's trace
// directory, so the trace of a test is found where the test wrote it.
export function traceSlug(value: string): string {
  let out = "";
  let dash = false;
  for (const c of value) {
    if (/^[A-Za-z0-9._-]$/.test(c)) {
      out += c;
      dash = false;
    } else if (!dash && out.length > 0) {
      out += "-";
      dash = true;
    }
  }
  out = out.replace(/^[-.]+|[-.]+$/g, "");
  if (out === "") {
    return "step";
  }
  if (out.length > 60) {
    out = out.slice(0, 60).replace(/[-.]+$/, "");
  }
  return out;
}

// traceDirectory is where the test kit writes the trace of the named test under
// base: one directory per name segment, each subtest a directory of its own.
export function traceDirectory(base: string, test: string): string {
  return path.join(base, ...test.split("/").map(traceSlug));
}

// traceBases answers the directories a trace may have been written under, most
// specific first: the one the editor's runs use, the one the environment sets,
// and the test kit's default under the temporary directory.
export function traceBases(editorDir: string | undefined, env: Readonly<Record<string, string | undefined>>, tmpdir: string): string[] {
  const bases: string[] = [];
  for (const base of [editorDir, env[contract.env.traceDir], path.join(tmpdir, contract.traceDefaultBase)]) {
    if (base !== undefined && base !== "" && !bases.includes(base)) {
      bases.push(base);
    }
  }
  return bases;
}

function record(value: unknown, name: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`The ${name} is not a JSON object.`);
  }
  return value as Record<string, unknown>;
}

function list(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((item): item is Record<string, unknown> => typeof item === "object" && item !== null);
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function nanoseconds(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value / 1e6 : 0;
}

// time reads an RFC 3339 instant, and answers undefined for Go's zero time,
// which is what an instant never recorded is written as.
function time(value: unknown): number | undefined {
  if (typeof value !== "string" || value === "" || value.startsWith("0001-01-01")) {
    return undefined;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : parsed;
}
