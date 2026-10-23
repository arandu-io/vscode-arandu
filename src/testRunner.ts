import { spawn } from "node:child_process";
import { parentKey, parseTestEvent, resultKey, type TestEvent } from "./testLayout";

// What one run of aru reports while it runs: each go test event, and every
// other line it prints -- aru building the views, a compile error.
export interface RunListener {
  readonly onEvent: (event: TestEvent) => void;
  readonly onOutput: (text: string) => void;
}

export interface RunOptions {
  readonly cwd: string;
  readonly env: Readonly<Record<string, string>>;
  readonly signal?: AbortSignal;
}

// runAru starts aru with the arguments as they are -- never through a shell, so
// no test name, tag or path is ever read as shell syntax -- and streams its
// output to the listener line by line. It answers the exit code, or null when
// the run was cancelled or aru could not be started.
export function runAru(executable: string, args: readonly string[], options: RunOptions, listener: RunListener): Promise<number | null> {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (code: number | null): void => {
      if (!settled) {
        settled = true;
        resolve(code);
      }
    };
    const child = spawn(executable, [...args], {
      cwd: options.cwd,
      env: { ...process.env, ...options.env },
      shell: false,
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"],
    });
    const abort = (): void => {
      child.kill();
      finish(null);
    };
    if (options.signal !== undefined) {
      if (options.signal.aborted) {
        abort();
      } else {
        options.signal.addEventListener("abort", abort, { once: true });
      }
    }
    let pending = "";
    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk: string) => {
      pending += chunk;
      let newline = pending.indexOf("\n");
      while (newline >= 0) {
        emit(pending.slice(0, newline), listener);
        pending = pending.slice(newline + 1);
        newline = pending.indexOf("\n");
      }
    });
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => listener.onOutput(chunk));
    child.on("error", (error) => {
      listener.onOutput(`${error.message}\n`);
      finish(null);
    });
    child.on("close", (code) => {
      if (pending !== "") {
        emit(pending, listener);
        pending = "";
      }
      options.signal?.removeEventListener("abort", abort);
      finish(code);
    });
  });
}

// What one short run of aru printed and how it ended.
export interface CapturedRun {
  // code is the exit code, or null when aru could not start, was stopped at
  // the time limit, or printed more than the limit allows.
  readonly code: number | null;
  readonly stdout: string;
  readonly stderr: string;
  // problem says why code is null, or is empty.
  readonly problem: string;
}

export interface CaptureOptions {
  readonly cwd: string;
  readonly timeoutMs: number;
  readonly maxBytes: number;
}

// captureAru runs aru to completion with the arguments as they are -- never
// through a shell -- and answers everything it printed. A run that outlives
// the time limit, or prints more than maxBytes on either stream, is stopped.
export function captureAru(executable: string, args: readonly string[], options: CaptureOptions): Promise<CapturedRun> {
  return new Promise((resolve) => {
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    let size = 0;
    let problem = "";
    let settled = false;
    const finish = (code: number | null): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      resolve({
        code: problem === "" ? code : null,
        stdout: Buffer.concat(stdout).toString("utf8"),
        stderr: Buffer.concat(stderr).toString("utf8"),
        problem,
      });
    };
    const child = spawn(executable, [...args], {
      cwd: options.cwd,
      env: process.env,
      shell: false,
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"],
    });
    // A stopped run is answered at once: a process aru started may still hold
    // the pipes open, and waiting for them to close would wait for it.
    const stop = (why: string): void => {
      if (problem === "") {
        problem = why;
      }
      child.kill();
      child.stdout.destroy();
      child.stderr.destroy();
      finish(null);
    };
    const timer = setTimeout(() => stop(`it did not finish within ${options.timeoutMs} ms`), options.timeoutMs);
    const collect = (into: Buffer[]) => (chunk: Buffer): void => {
      size += chunk.length;
      if (size > options.maxBytes) {
        stop(`it printed more than ${options.maxBytes} bytes`);
        return;
      }
      into.push(chunk);
    };
    child.stdout.on("data", collect(stdout));
    child.stderr.on("data", collect(stderr));
    child.on("error", (error) => {
      if (problem === "") {
        problem = error.message;
      }
      finish(null);
    });
    child.on("close", (code) => finish(code));
  });
}

function emit(line: string, listener: RunListener): void {
  const event = parseTestEvent(line);
  if (event === undefined) {
    listener.onOutput(`${line}\n`);
    return;
  }
  listener.onEvent(event);
}

// A change in a test's state that the Testing view shows. The key of a test's
// update is its resultKey: the package and the full name, so a subtest's
// update names the subtest.
export type ResultUpdate =
  | { readonly kind: "started"; readonly key: string }
  | { readonly kind: "output"; readonly key: string | undefined; readonly text: string }
  | { readonly kind: "passed" | "skipped"; readonly key: string; readonly elapsedMs: number | undefined; readonly output: string }
  | { readonly kind: "failed"; readonly key: string; readonly elapsedMs: number | undefined; readonly output: string }
  | { readonly kind: "package-failed"; readonly packagePath: string; readonly output: string };

// ResultCollector turns go test events into what happened to each test and
// subtest: when it started, what it printed -- the output of the subtests
// under it included -- and how it ended. A package that failed without a test
// failing, which is a build that broke, is reported with everything the
// package printed.
export class ResultCollector {
  private readonly outputs = new Map<string, string>();
  private readonly packageOutputs = new Map<string, string>();
  private readonly failedTests = new Set<string>();

  public feed(event: TestEvent): ResultUpdate | undefined {
    if (event.test === undefined) {
      if (event.output !== undefined) {
        this.packageOutputs.set(event.packagePath, (this.packageOutputs.get(event.packagePath) ?? "") + event.output);
        return { kind: "output", key: undefined, text: event.output };
      }
      if (event.action === "fail" || event.action === "build-fail") {
        const anyTestFailed = [...this.failedTests].some((key) => key.startsWith(`${event.packagePath}#`));
        if (!anyTestFailed) {
          return { kind: "package-failed", packagePath: event.packagePath, output: this.packageOutputs.get(event.packagePath) ?? "" };
        }
      }
      return undefined;
    }
    const key = resultKey(event.packagePath, event.test);
    switch (event.action) {
      case "run":
        return { kind: "started", key };
      case "output": {
        const text = event.output ?? "";
        for (let owner: string | undefined = key; owner !== undefined; owner = parentKey(owner)) {
          this.outputs.set(owner, (this.outputs.get(owner) ?? "") + text);
        }
        return { kind: "output", key, text };
      }
      case "pass":
      case "skip":
        return { kind: event.action === "pass" ? "passed" : "skipped", key, elapsedMs: event.elapsedMs, output: this.outputs.get(key) ?? "" };
      case "fail":
        if (!event.test.includes("/")) {
          this.failedTests.add(key);
        }
        return { kind: "failed", key, elapsedMs: event.elapsedMs, output: this.outputs.get(key) ?? "" };
      default:
        return undefined;
    }
  }
}
