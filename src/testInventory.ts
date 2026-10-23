import contract from "./testingContract.json";
import { layerOf, packagePath, parseTestFile, testLayers, type TestLayer } from "./testLayout";
import { captureAru } from "./testRunner";

// A subtest the Testing view shows under its test: the full name go test
// reports, TestX/sub/inner, and the 1-based line of its t.Run.
export interface ListedSubtest {
  readonly name: string;
  readonly line: number;
}

// A top-level test: its name, the 1-based line of its func keyword, and the
// subtests whose names are known before it runs.
export interface ListedTest {
  readonly name: string;
  readonly line: number;
  readonly subtests: readonly ListedSubtest[];
}

// A test file and what running its tests needs: the layer it is in, its path
// relative to the project with forward slashes, the import path go test
// reports for its package, and the tags its build constraint requires.
export interface ListedFile {
  readonly layer: TestLayer;
  readonly relative: string;
  readonly packagePath: string;
  readonly tags: readonly string[];
  readonly tests: readonly ListedTest[];
}

// A file aru could not parse, and what the parser said.
export interface UnreadableFile {
  readonly file: string;
  readonly error: string;
}

// Every test of a project, from aru test --list --json or, for an aru that
// predates it, from the editor's own reading of the sources.
export interface TestListing {
  readonly source: "aru" | "scanner";
  readonly layers: readonly TestLayer[];
  readonly files: readonly ListedFile[];
  readonly unreadable: readonly UnreadableFile[];
}

// listingFromInventory reads the output of aru test --list --json, version 1,
// into a listing. It refuses anything else -- another version, a missing or
// mistyped field, a file outside the project -- with the reason, so the caller
// can fall back to reading the sources itself. A field it does not know is
// ignored, as the format allows.
export function listingFromInventory(text: string): TestListing {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch (error: unknown) {
    throw new Error(`the output is not JSON: ${error instanceof Error ? error.message : String(error)}`);
  }
  const inventory = object(raw, "the inventory");
  if (inventory.version !== contract.inventoryVersion) {
    throw new Error(`the inventory is format version ${JSON.stringify(inventory.version)}, and this editor reads ${contract.inventoryVersion}`);
  }
  const layers = new Map<string, TestLayer>();
  for (const [index, entry] of array(inventory.layers, "layers").entries()) {
    const layer = object(entry, `layers[${index}]`);
    const name = stringField(layer.name, `layers[${index}].name`);
    const dir = stringField(layer.dir, `layers[${index}].dir`);
    if (name === "" || (dir !== "" && !safeRelative(dir))) {
      throw new Error(`layers[${index}] is not a layer of the project: ${JSON.stringify(entry)}`);
    }
    layers.set(name, { id: name.toLowerCase(), label: name, directory: dir });
  }
  const files = new Map<string, { file: ListedFile; tests: ListedTest[] }>();
  for (const [index, entry] of array(inventory.tests, "tests").entries()) {
    const where = `tests[${index}]`;
    const test = object(entry, where);
    const layer = layers.get(stringField(test.layer, `${where}.layer`));
    if (layer === undefined) {
      throw new Error(`${where} is in a layer the inventory does not declare: ${JSON.stringify(test.layer)}`);
    }
    const relative = stringField(test.file, `${where}.file`);
    if (!safeRelative(relative) || !relative.endsWith(contract.testFileSuffix)) {
      throw new Error(`${where}.file is not a test file inside the project: ${JSON.stringify(relative)}`);
    }
    const name = stringField(test.name, `${where}.name`);
    if (!/^Test/.test(name) || name.includes("/")) {
      throw new Error(`${where}.name is not a test function: ${JSON.stringify(name)}`);
    }
    const subtests = array(test.subtests, `${where}.subtests`).map((value, at) => {
      const subtest = object(value, `${where}.subtests[${at}]`);
      const full = stringField(subtest.name, `${where}.subtests[${at}].name`);
      if (!full.startsWith(`${name}/`)) {
        throw new Error(`${where}.subtests[${at}] is not a subtest of ${name}: ${JSON.stringify(full)}`);
      }
      return { name: full, line: line(subtest.line, `${where}.subtests[${at}].line`) };
    });
    const tags = array(test.tags, `${where}.tags`).map((tag, at) => stringField(tag, `${where}.tags[${at}]`));
    const listed = files.get(relative) ?? {
      file: { layer, relative, packagePath: stringField(test.package, `${where}.package`), tags, tests: [] },
      tests: [],
    };
    listed.tests.push({ name, line: line(test.line, `${where}.line`), subtests });
    files.set(relative, listed);
  }
  const unreadable = array(inventory.unreadable, "unreadable").map((value, at) => {
    const entry = object(value, `unreadable[${at}]`);
    return { file: stringField(entry.file, `unreadable[${at}].file`), error: stringField(entry.error, `unreadable[${at}].error`) };
  });
  return {
    source: "aru",
    layers: [...layers.values()],
    files: [...files.values()].map(({ file, tests }) => ({ ...file, tests })),
    unreadable,
  };
}

// scannedFile is what the editor reads from a test file itself, for an aru
// without --list: the file's layer, package and tags, and its tests with the
// same 1-based lines aru reports. It has no subtests, which only aru reads. It
// answers undefined for a file outside the five layer directories.
export function scannedFile(relative: string, modulePath: string | undefined, source: string): ListedFile | undefined {
  const layer = layerOf(relative);
  if (layer === undefined) {
    return undefined;
  }
  const parsed = parseTestFile(source);
  const slash = relative.lastIndexOf("/");
  return {
    layer,
    relative,
    packagePath: modulePath === undefined ? "" : packagePath(modulePath, slash < 0 ? "" : relative.slice(0, slash)),
    tags: parsed.tags,
    tests: parsed.tests.map((test) => ({ name: test.name, line: test.line, subtests: [] })),
  };
}

// scannedListing is the listing the editor reads itself from the files it
// found, ordered as aru orders it: by layer, then by path.
export function scannedListing(files: ReadonlyArray<{ relative: string; source: string }>, modulePath: string | undefined): TestListing {
  const listed: ListedFile[] = [];
  for (const { relative, source } of files) {
    const file = scannedFile(relative, modulePath, source);
    if (file !== undefined && file.tests.length > 0) {
      listed.push(file);
    }
  }
  const order = (file: ListedFile): number => testLayers.findIndex((layer) => layer.id === file.layer.id);
  listed.sort((left, right) => order(left) - order(right) || (left.relative < right.relative ? -1 : left.relative > right.relative ? 1 : 0));
  return { source: "scanner", layers: testLayers, files: listed, unreadable: [] };
}

// listsTests reports whether the aru that printed this to aru version can list
// tests as JSON: a release from v0.79.0 on, or a build between tags after it.
// A build that reports no version -- dev -- is not trusted to: an older aru
// hands --list to go test, which compiles the project and runs its TestMain.
export function listsTests(versionOutput: string): boolean {
  const match = /^(?:aru\s+)?v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.exec(versionOutput.trim());
  if (match === null) {
    return false;
  }
  const have = [Number(match[1]), Number(match[2]), Number(match[3])];
  const since = contract.listSince.split(".").map(Number);
  for (let index = 0; index < 3; index++) {
    if (have[index] !== since[index]) {
      return have[index] > since[index];
    }
  }
  // A prerelease of the first version that lists precedes it.
  return match[4] === undefined;
}

// What asking aru for the tests came to: the listing, or why there is none
// and the editor reads the sources itself.
export type AruListing =
  | { readonly listing: TestListing; readonly version: string }
  | { readonly listing: undefined; readonly reason: string };

// listWithAru asks the aru at executable for the tests of the project at root.
// It reads aru's version first and asks for --list --json only when that aru
// has it, because an older one passes the flags to go test. Any answer it
// cannot read -- an exit status other than zero, output that is not version 1
// of the inventory -- is a reason to fall back, never a listing.
export async function listWithAru(executable: string, root: string): Promise<AruListing> {
  const version = await captureAru(executable, contract.versionArgs, {
    cwd: root,
    timeoutMs: contract.versionTimeoutMilliseconds,
    maxBytes: 4096,
  });
  const reported = version.stdout.trim();
  if (version.code !== 0) {
    return { listing: undefined, reason: `aru version ${ended(version.code, version.problem)}` };
  }
  if (!listsTests(reported)) {
    return {
      listing: undefined,
      reason: `${reported === "" ? "this aru" : reported} cannot list tests as JSON; aru test --list --json arrived in v${contract.listSince}`,
    };
  }
  const listed = await captureAru(executable, contract.listArgs, {
    cwd: root,
    timeoutMs: contract.listTimeoutMilliseconds,
    maxBytes: contract.listMaxBytes,
  });
  if (listed.code !== 0) {
    const said = listed.stderr.trim().split("\n")[0] ?? "";
    return { listing: undefined, reason: `aru test --list --json ${ended(listed.code, listed.problem)}${said === "" ? "" : `: ${said}`}` };
  }
  try {
    return { listing: listingFromInventory(listed.stdout), version: reported };
  } catch (error: unknown) {
    return { listing: undefined, reason: `aru test --list --json printed no inventory this editor reads: ${error instanceof Error ? error.message : String(error)}` };
  }
}

function ended(code: number | null, problem: string): string {
  return code === null ? `stopped: ${problem}` : `exited with status ${code}`;
}

function object(value: unknown, what: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`${what} is not a JSON object`);
  }
  return value as Record<string, unknown>;
}

function array(value: unknown, what: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new Error(`${what} is not an array`);
  }
  return value;
}

function stringField(value: unknown, what: string): string {
  if (typeof value !== "string") {
    throw new Error(`${what} is not a string`);
  }
  return value;
}

function line(value: unknown, what: string): number {
  if (typeof value !== "number" || !Number.isInteger(value) || value < 1) {
    throw new Error(`${what} is not a 1-based line: ${JSON.stringify(value)}`);
  }
  return value;
}

// safeRelative reports whether a path from aru names something inside the
// project: relative, with forward slashes, and never climbing out of it.
function safeRelative(relative: string): boolean {
  if (relative === "" || relative.startsWith("/") || relative.includes("\\") || relative.includes("\0") || /^[A-Za-z]:/.test(relative)) {
    return false;
  }
  return relative.split("/").every((part) => part !== "" && part !== "." && part !== "..");
}
