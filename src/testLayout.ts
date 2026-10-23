import contract from "./testingContract.json";

// The test layers of an Arandu project, each a directory under tests/. The
// order is the order the Testing view shows them in.
export interface TestLayer {
  readonly id: string;
  readonly label: string;
  readonly directory: string;
}

export const testLayers: readonly TestLayer[] = contract.layers;

// A test function found in a file: its name and the 1-based line of its func
// keyword, the numbering aru test --list uses.
export interface DiscoveredTest {
  readonly name: string;
  readonly line: number;
}

// editorLine is the zero-based line the editor's positions count in, for a
// 1-based line as aru and the scanner report it.
export function editorLine(line: number): number {
  return Math.max(0, line - 1);
}

// What a test file declares: its package, the build constraint it opens with,
// the tags that constraint requires, and its test functions in source order.
export interface TestFile {
  readonly packageName: string;
  readonly constraint: string;
  readonly tags: readonly string[];
  readonly tests: readonly DiscoveredTest[];
}

// layerOf answers the layer a slash-separated path inside the project belongs
// to, or undefined for a path outside the five layer directories -- a helper
// such as tests/TestCase.go, or a directory the layout does not name.
export function layerOf(relative: string): TestLayer | undefined {
  if (!relative.endsWith(contract.testFileSuffix)) {
    return undefined;
  }
  return testLayers.find((layer) => relative.startsWith(`${layer.directory}/`));
}

// isE2E reports whether the layer is the end-to-end one, which runs with its
// own build tag, a driver and a browser.
export function isE2E(layer: TestLayer | string): boolean {
  return (typeof layer === "string" ? layer : layer.id) === contract.e2eLayer;
}

// Tags a constraint can name that select a platform or toolchain rather than a
// part of the suite: operating systems, architectures, compilers and cgo. They
// are satisfied by the machine, never passed to -tags. ignore is not one of
// them: a file that opens with //go:build ignore compiles only under
// -tags ignore, so it is a tag like any other.
const platformTags = new Set([
  "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux", "nacl",
  "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos", "unix",
  "386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "loong64", "mips", "mipsle", "mips64",
  "mips64le", "mips64p32", "mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390", "s390x",
  "sparc", "sparc64", "wasm", "cgo", "gc", "gccgo",
]);

// parseTestFile reads a Go test file the way the go tool lists its tests: the
// top-level functions named Test followed by anything but a lower-case letter,
// with a single *testing.T parameter, no result and a body. A declaration
// inside a block comment or a raw string is not one, and neither is a method,
// TestMain(m *testing.M), or Testify; a TestMain taking *testing.T is an
// ordinary test to the go tool, and so it is here. Lines are 1-based, as aru
// test --list reports them.
export function parseTestFile(source: string): TestFile {
  const lines = topLevelLines(source);
  let constraint = "";
  let packageName = "";
  const tests: DiscoveredTest[] = [];
  const seen = new Set<string>();
  for (const { text, line } of lines) {
    if (packageName === "") {
      const build = /^\/\/go:build\s+(.+?)\s*$/.exec(text);
      if (build !== null && constraint === "") {
        constraint = build[1];
        continue;
      }
      const pkg = /^package\s+([A-Za-z_][A-Za-z0-9_]*)/.exec(text);
      if (pkg !== null) {
        packageName = pkg[1];
      }
      continue;
    }
    const declaration = /^func\s+(Test(?:(?!\p{Ll})[\p{L}\p{Nd}_][\p{L}\p{Nd}_]*)?)\s*\(\s*(?:[\p{L}_][\p{L}\p{Nd}_]*\s*)?\*\s*(?:[\p{L}_][\p{L}\p{Nd}_]*\s*\.\s*)?T\s*,?\s*\)\s*\{/u.exec(text);
    if (declaration === null) {
      continue;
    }
    const name = declaration[1];
    if (seen.has(name)) {
      continue;
    }
    seen.add(name);
    tests.push({ name, line });
  }
  return { packageName, constraint, tags: constraintTags(constraint), tests };
}

// A build constraint expression, as the go tool parses it.
type ConstraintExpr =
  | { readonly kind: "tag"; readonly tag: string }
  | { readonly kind: "not"; readonly x: ConstraintExpr }
  | { readonly kind: "and" | "or"; readonly x: ConstraintExpr; readonly y: ConstraintExpr };

// constraintTags answers the tags a build constraint requires that the suite
// has to be asked for: every tag it names, left to right, that is not under an
// odd number of negations and is not a platform or a Go version. A tag inside
// !(a && b) is negated like one written !a. A constraint that does not parse
// asks for nothing, as aru test --list reads it.
export function constraintTags(constraint: string): readonly string[] {
  const expr = parseConstraint(constraint);
  const tags: string[] = [];
  const collect = (e: ConstraintExpr, negated: boolean): void => {
    switch (e.kind) {
      case "tag":
        if (!negated && !platformTags.has(e.tag) && !/^go1\.[0-9]+$/.test(e.tag) && !tags.includes(e.tag)) {
          tags.push(e.tag);
        }
        return;
      case "not":
        collect(e.x, !negated);
        return;
      default:
        collect(e.x, negated);
        collect(e.y, negated);
    }
  };
  if (expr !== undefined) {
    collect(expr, false);
  }
  return tags;
}

// parseConstraint reads a //go:build expression -- tags, !, &&, || and
// parentheses, with && binding tighter than || -- or answers undefined when it
// is not one.
function parseConstraint(text: string): ConstraintExpr | undefined {
  const tokens = text.match(/\s+|&&|\|\||[!()]|[A-Za-z0-9_.]+|./g)?.filter((token) => token.trim() !== "") ?? [];
  let at = 0;
  const or = (): ConstraintExpr | undefined => {
    let left = and();
    while (left !== undefined && tokens[at] === "||") {
      at++;
      const right = and();
      left = right === undefined ? undefined : { kind: "or", x: left, y: right };
    }
    return left;
  };
  const and = (): ConstraintExpr | undefined => {
    let left = not();
    while (left !== undefined && tokens[at] === "&&") {
      at++;
      const right = not();
      left = right === undefined ? undefined : { kind: "and", x: left, y: right };
    }
    return left;
  };
  const not = (): ConstraintExpr | undefined => {
    const token = tokens[at];
    if (token === "!") {
      at++;
      const x = not();
      return x === undefined ? undefined : { kind: "not", x };
    }
    if (token === "(") {
      at++;
      const x = or();
      if (x === undefined || tokens[at] !== ")") {
        return undefined;
      }
      at++;
      return x;
    }
    if (token !== undefined && /^[A-Za-z0-9_.]+$/.test(token)) {
      at++;
      return { kind: "tag", tag: token };
    }
    return undefined;
  };
  const expr = or();
  return expr !== undefined && at === tokens.length ? expr : undefined;
}

// topLevelLines answers the lines of the source that start outside a block
// comment and outside a raw string, with their 1-based numbers. Only those
// can open a declaration the go tool would list.
function topLevelLines(source: string): Array<{ text: string; line: number }> {
  const out: Array<{ text: string; line: number }> = [];
  let state: "code" | "block" | "raw" | "string" | "rune" = "code";
  let lineStart = 0;
  let line = 1;
  let startsInCode = true;
  for (let i = 0; i <= source.length; i++) {
    const c = source[i];
    if (c === "\n" || i === source.length) {
      if (startsInCode) {
        out.push({ text: source.slice(lineStart, i).replace(/\r$/, ""), line });
      }
      if (state === "string" || state === "rune") {
        state = "code";
      }
      line++;
      lineStart = i + 1;
      startsInCode = state === "code";
      continue;
    }
    switch (state) {
      case "code":
        if (c === "/" && source[i + 1] === "/") {
          const end = source.indexOf("\n", i);
          i = (end < 0 ? source.length : end) - 1;
        } else if (c === "/" && source[i + 1] === "*") {
          state = "block";
          i++;
        } else if (c === "`") {
          state = "raw";
        } else if (c === "\"") {
          state = "string";
        } else if (c === "'") {
          state = "rune";
        }
        break;
      case "block":
        if (c === "*" && source[i + 1] === "/") {
          state = "code";
          i++;
        }
        break;
      case "raw":
        if (c === "`") {
          state = "code";
        }
        break;
      case "string":
      case "rune":
        if (c === "\\") {
          i++;
        } else if ((state === "string" && c === "\"") || (state === "rune" && c === "'")) {
          state = "code";
        }
        break;
    }
  }
  return out;
}

// moduleOf answers the module path a go.mod declares.
export function moduleOf(goMod: string): string | undefined {
  for (const raw of goMod.split(/\r?\n/)) {
    const match = /^\s*module\s+("?)([^\s"]+)\1\s*(?:\/\/.*)?$/.exec(raw);
    if (match !== null) {
      return match[2];
    }
  }
  return undefined;
}

// packagePath answers the import path go test reports for the package in the
// slash-separated directory `directory` of the module.
export function packagePath(modulePath: string, directory: string): string {
  const trimmed = directory.replace(/^\.\/?/, "").replace(/\/+$/, "");
  return trimmed === "" ? modulePath : `${modulePath}/${trimmed}`;
}

// A test to run: its layer, the package it is in, its name and the tags its
// file asks for.
export interface TestSelection {
  readonly layer: string;
  readonly packagePath: string;
  readonly name: string;
  readonly tags: readonly string[];
}

// The driver and browser an end-to-end run uses.
export interface DriverChoice {
  readonly driver: string;
  readonly browser: string;
}

// A batch is one invocation of aru test: the tests it runs and the arguments it
// runs them with.
export interface TestBatch {
  readonly e2e: boolean;
  readonly args: readonly string[];
  readonly tests: readonly TestSelection[];
}

// testBatches groups the tests into the fewest aru test invocations: the
// end-to-end tests run apart, with --e2e and the driver chosen, and the rest run
// together per set of build tags. Each invocation asks go test for JSON and
// anchors -run to the exact names, so a name that is a prefix of another does
// not run both.
export function testBatches(tests: readonly TestSelection[], choice: DriverChoice | undefined): TestBatch[] {
  const groups = new Map<string, { e2e: boolean; tags: readonly string[]; tests: TestSelection[] }>();
  for (const test of tests) {
    const e2e = isE2E(test.layer);
    const tags = e2e ? [] : [...test.tags].sort();
    const key = `${e2e ? "e2e" : "plain"}|${tags.join(",")}`;
    const group = groups.get(key) ?? { e2e, tags, tests: [] };
    group.tests.push(test);
    groups.set(key, group);
  }
  const batches: TestBatch[] = [];
  for (const group of groups.values()) {
    const args = [...contract.testArgs];
    if (group.e2e) {
      args.push(contract.e2eFlag);
      if (choice !== undefined) {
        args.push(`${contract.driverFlag}=${choice.driver}`, `${contract.browserFlag}=${choice.browser}`);
      }
    } else if (group.tags.length > 0) {
      args.push(contract.testTagsFlag, group.tags.join(","));
    }
    args.push(contract.testJsonFlag, contract.testRunFlag, runPattern(group.tests.map((test) => test.name)));
    batches.push({ e2e: group.e2e, args, tests: group.tests });
  }
  return batches.sort((left, right) => Number(left.e2e) - Number(right.e2e));
}

// runPattern is the -run expression that matches exactly the named top-level
// tests and every subtest under them.
export function runPattern(names: readonly string[]): string {
  const unique = [...new Set(names)].sort();
  return `^(${unique.map(escapeRegExp).join("|")})$`;
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// A go test -json event, with the fields the controller reads.
export interface TestEvent {
  readonly action: string;
  readonly packagePath: string;
  readonly test: string | undefined;
  readonly output: string | undefined;
  readonly elapsedMs: number | undefined;
}

// parseTestEvent reads one line of go test -json output, and answers undefined
// for a line that is not an event -- a build error, or what aru prints before
// go test starts.
export function parseTestEvent(line: string): TestEvent | undefined {
  const trimmed = line.trim();
  if (!trimmed.startsWith("{")) {
    return undefined;
  }
  let raw: unknown;
  try {
    raw = JSON.parse(trimmed);
  } catch {
    return undefined;
  }
  if (typeof raw !== "object" || raw === null) {
    return undefined;
  }
  const event = raw as Record<string, unknown>;
  if (typeof event.Action !== "string") {
    return undefined;
  }
  // A build event names the package in ImportPath, followed by the test binary
  // it was building in brackets.
  const importPath = typeof event.ImportPath === "string" ? event.ImportPath.replace(/\s+\[.*$/, "") : "";
  return {
    action: event.Action,
    packagePath: typeof event.Package === "string" && event.Package !== "" ? event.Package : importPath,
    test: typeof event.Test === "string" && event.Test !== "" ? event.Test : undefined,
    output: typeof event.Output === "string" ? event.Output : undefined,
    elapsedMs: typeof event.Elapsed === "number" ? Math.round(event.Elapsed * 1000) : undefined,
  };
}

// topLevelTest answers the top-level test a test or subtest name belongs to.
export function topLevelTest(name: string): string {
  const slash = name.indexOf("/");
  return slash < 0 ? name : name.slice(0, slash);
}

// selectionKey is how a reported result finds the test it belongs to: the
// package and the top-level name, because two packages may hold tests with
// the same name and -run matches both.
export function selectionKey(packagePathValue: string, name: string): string {
  return `${packagePathValue}#${topLevelTest(name)}`;
}

// failureLine answers the zero-based line the first "file_test.go:N:" of a
// failure's output points at in the named file, or undefined.
export function failureLine(output: string, fileName: string): number | undefined {
  const pattern = /([A-Za-z0-9_.-]+_test\.go):(\d+)(?::\d+)?:/g;
  for (let match = pattern.exec(output); match !== null; match = pattern.exec(output)) {
    if (match[1] === fileName) {
      return Math.max(0, Number(match[2]) - 1);
    }
  }
  return undefined;
}

// A debug session of one package's tests through the Go extension and Delve.
export interface DebugRequest {
  readonly name: string;
  readonly packageDir: string;
  readonly names: readonly string[];
  readonly tags: readonly string[];
  readonly e2e: boolean;
  readonly env: Readonly<Record<string, string>>;
  readonly marker: string;
}

// debugConfiguration is the launch configuration the Go extension debugs the
// tests with: test mode on the package directory, -test.run anchored to the
// names, and the build tags the files ask for -- e2e always for a journey, which
// the ordinary build leaves out. The marker lets the editor recognise the
// session it started when it ends.
export function debugConfiguration(request: DebugRequest): Record<string, unknown> {
  const tags = request.e2e
    ? [contract.e2eTag, ...request.tags.filter((tag) => tag !== contract.e2eTag)]
    : [...request.tags];
  return {
    type: "go",
    request: "launch",
    mode: "test",
    name: request.name,
    program: request.packageDir,
    args: ["-test.run", runPattern(request.names), "-test.v"],
    ...(tags.length > 0 ? { buildFlags: `${contract.testTagsFlag}=${tags.join(",")}` } : {}),
    env: { ...request.env },
    aranduTestRun: request.marker,
  };
}
