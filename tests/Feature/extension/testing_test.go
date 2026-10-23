package extension_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// runTestingModules bundles the editor-free modules behind the Testing view,
// the journey commands and the trace viewer, and runs a script against them in
// Node with the bundle's exports as `m`. The helpers are the ones
// runExtensionModules gives its scripts, plus `env(name)` for a value the Go
// side passes in.
func runTestingModules(t *testing.T, script string, env ...string) {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "testing.cjs")
	build := exec.Command(
		filepath.Join(rootPath(t), "node_modules", ".bin", "esbuild"),
		"--bundle",
		"--platform=node",
		"--format=cjs",
		"--log-level=warning",
		"--sourcefile=testing-modules.ts",
		"--outfile="+bundle,
	)
	build.Dir = rootPath(t)
	build.Stdin = strings.NewReader(strings.Join([]string{
		`export * from "./src/testLayout";`,
		`export * from "./src/e2eDrivers";`,
		`export * from "./src/traceModel";`,
		`export * from "./src/traceView";`,
		`export * from "./src/devServer";`,
		`export * from "./src/testRunner";`,
	}, "\n"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("bundle testing modules: %v\n%s", err, output)
	}
	check := exec.Command("node", "-e", `
const m = require(process.env.ARANDU_MODULES);
const fs = require("node:fs");
const path = require("node:path");
const fixtures = process.env.ARANDU_FIXTURES;
const env = (name) => process.env[name];
const assert = (condition, message) => { if (!condition) { throw new Error(message); } };
const same = (got, want, message) => {
  const left = JSON.stringify(got);
  const right = JSON.stringify(want);
  if (left !== right) { throw new Error(message + ": got " + left + ", want " + right); }
};
const throws = (fn, pattern, message) => {
  try { fn(); } catch (error) {
    if (!pattern.test(String(error.message))) { throw new Error(message + ": wrong error " + error.message); }
    return;
  }
  throw new Error(message + ": did not throw");
};
(async () => {
`+script+`
})().catch((error) => { console.error(error && error.stack || error); process.exit(1); });
`)
	check.Env = append(os.Environ(),
		"ARANDU_MODULES="+bundle,
		"ARANDU_FIXTURES="+rootPath(t, "tests", "Feature", "extension", "testdata"),
	)
	check.Env = append(check.Env, env...)
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

// TestTheTestingViewFindsEveryLayersTestsPerFunction fixes what the Testing
// view lists: the five layers by directory, and in each file the functions go
// test would run -- not a method, not TestMain, not Testify, not a declaration
// inside a comment or a raw string.
func TestTheTestingViewFindsEveryLayersTestsPerFunction(t *testing.T) {
	runTestingModules(t, `
same(m.testLayers.map((layer) => layer.label), ["Unit", "Feature", "Contract", "E2E", "Conformance"], "the layers, in order");
const layer = (relative) => m.layerOf(relative)?.id;
same(layer("tests/Unit/money_test.go"), "unit", "unit");
same(layer("tests/Feature/billing/invoice_test.go"), "feature", "feature, nested");
same(layer("tests/Contract/openapi_test.go"), "contract", "contract");
same(layer("tests/E2E/checkout_test.go"), "e2e", "e2e");
same(layer("tests/Conformance/auth_test.go"), "conformance", "conformance");
same(layer("tests/TestCase.go"), undefined, "a helper is no layer");
same(layer("tests/E2E/helpers.go"), undefined, "only test files");
same(layer("tests/Browser/login_test.go"), undefined, "the absorbed browser directory is not a layer");
same(layer("tests/Unitary/x_test.go"), undefined, "a prefix of a layer directory is not the layer");
same(layer("app/models/user_test.go"), undefined, "application packages are not a layer");
assert(m.isE2E("e2e") && !m.isE2E("feature"), "isE2E");

const source = [
  "//go:build e2e && !windows",
  "",
  "package e2e_test",
  "",
  "import \"testing\"",
  "",
  "func TestCheckout(t *testing.T) {",
  "\tt.Run(\"func TestInsideString(t *testing.T)\", func(t *testing.T) {})",
  "}",
  "",
  "/*",
  "func TestCommentedOut(t *testing.T) {}",
  "*/",
  "var doc = `+"`"+`",
  "func TestInRawString(t *testing.T) {}",
  "func TestStillInRawString(t *testing.T) {}",
  "`+"`"+`",
  "func TestMain(m *testing.M) {}",
  "func TestMain(t *testing.T) {}",
  "func Testify(t *testing.T) {}",
  "func (s *suite) TestMethod(t *testing.T) {}",
  "func TestBench(b *testing.B) {}",
  "// func TestLineComment(t *testing.T) {}",
  "func Test_underscore(t *testing.T) {}",
  "func Test(t *testing.T) {}",
  "func TestAlias(tt * tst.T) {}",
  "func TestLogin(t *testing.T) { t.Log(\"/* not a comment\") }",
  "func TestAfterString(t *testing.T) {}",
].join("\n");
const parsed = m.parseTestFile(source);
same(parsed.packageName, "e2e_test", "package");
same(parsed.constraint, "e2e && !windows", "constraint");
same(parsed.tags, ["e2e"], "the tags the constraint requires");
same(parsed.tests, [
  { name: "TestCheckout", line: 7 },
  { name: "TestMain", line: 19 },
  { name: "Test_underscore", line: 24 },
  { name: "Test", line: 25 },
  { name: "TestAlias", line: 26 },
  { name: "TestLogin", line: 27 },
  { name: "TestAfterString", line: 28 },
], "the functions go test runs, with their 1-based lines; a TestMain taking *testing.T is a test to the go tool");
same(m.editorLine(7), 6, "the editor counts from zero");
same(m.editorLine(0), 0, "a line before the first is the first");
same(m.parseTestFile("package unit\nfunc TestResult(t *testing.T) error { return nil }\nfunc TestNoBody(t *testing.T)\nfunc TestNoName(*testing.T) {}\nfunc Testé(t *testing.T) {}\nfunc TestÜber(t *testing.T) {}\n").tests, [
  { name: "TestNoName", line: 4 },
  { name: "TestÜber", line: 6 },
], "a result or no body is no test; an unnamed parameter is one; a lower-case letter after Test is not one in any script");

same(m.constraintTags("integration || (linux && amd64)"), ["integration"], "platforms are never tags to ask for");
same(m.constraintTags("go1.22 && sqlite_ok && !cgo"), ["sqlite_ok"], "release tags and negations are left out");
same(m.constraintTags("ignore"), ["ignore"], "only -tags ignore compiles a //go:build ignore file, so it is asked for");
same(m.constraintTags("!(linux && slowauth) || conformance"), ["conformance"], "a tag under a negated group is negated");
same(m.constraintTags("!!twice && sparc64 || riscv || armbe"), ["twice"], "a double negation asks; every architecture the toolchain knows is the machine's");
same(m.constraintTags("broken &&"), [], "a constraint that does not parse asks for nothing");
same(m.parseTestFile("package unit\nfunc TestX(t *testing.T) {}\n").tags, [], "no constraint, no tags");
same(m.moduleOf("// comment\nmodule github.com/acme/shop // the shop\n\ngo 1.26\n"), "github.com/acme/shop", "module path");
same(m.packagePath("github.com/acme/shop", "tests/Feature/billing"), "github.com/acme/shop/tests/Feature/billing", "package import path");
`)
}

// TestTheTestingViewRunsEachLayerThroughAruTest fixes the command lines: aru
// test with JSON and an anchored -run, the end-to-end tests apart with --e2e
// and the chosen driver and browser, and a file's own tags passed to the rest.
func TestTheTestingViewRunsEachLayerThroughAruTest(t *testing.T) {
	runTestingModules(t, `
const pkg = "github.com/acme/shop/tests/";
const tests = [
  { layer: "unit", packagePath: pkg + "Unit", name: "TestMoney", tags: [] },
  { layer: "feature", packagePath: pkg + "Feature", name: "TestInvoice", tags: [] },
  { layer: "contract", packagePath: pkg + "Contract", name: "TestOpenAPI", tags: [] },
  { layer: "conformance", packagePath: pkg + "Conformance", name: "TestAuthKit", tags: ["integration"] },
  { layer: "e2e", packagePath: pkg + "E2E", name: "TestCheckout", tags: ["e2e"] },
  { layer: "e2e", packagePath: pkg + "E2E", name: "TestLogin", tags: ["e2e"] },
];
const batches = m.testBatches(tests, { driver: "playwright", browser: "firefox" });
same(batches.map((batch) => batch.args), [
  ["test", "-json", "-run", "^(TestInvoice|TestMoney|TestOpenAPI)$"],
  ["test", "-tags", "integration", "-json", "-run", "^(TestAuthKit)$"],
  ["test", "--e2e", "--driver=playwright", "--browser=firefox", "-json", "-run", "^(TestCheckout|TestLogin)$"],
], "one aru test per kind of run");
same(batches.map((batch) => batch.e2e), [false, false, true], "the journeys run last and apart");
same(m.testBatches([tests[4]], undefined)[0].args, ["test", "--e2e", "-json", "-run", "^(TestCheckout)$"], "no choice leaves the driver to the test case");
same(m.runPattern(["TestA", "TestA", "TestB"]), "^(TestA|TestB)$", "names are anchored and unique");
assert(new RegExp(m.runPattern(["TestA"])).test("TestA") && !new RegExp(m.runPattern(["TestA"])).test("TestAB"), "a prefix does not run its longer neighbour");
`)
}

// TestTheTestingViewReadsGoTestJSONPerPackage fixes how results reach their
// test: by package and top-level name, a subtest's output under its parent, a
// broken build reported on the tests it stopped, and the failure's line found
// in its own file.
func TestTheTestingViewReadsGoTestJSONPerPackage(t *testing.T) {
	runTestingModules(t, `
const pkgA = "github.com/acme/shop/tests/Feature/billing";
const pkgB = "github.com/acme/shop/tests/Feature/orders";
const lines = [
  "aru: building the views",
  JSON.stringify({ Action: "start", Package: pkgA }),
  JSON.stringify({ Action: "run", Package: pkgA, Test: "TestIndex" }),
  JSON.stringify({ Action: "output", Package: pkgA, Test: "TestIndex", Output: "=== RUN   TestIndex\n" }),
  JSON.stringify({ Action: "run", Package: pkgA, Test: "TestIndex/empty" }),
  JSON.stringify({ Action: "output", Package: pkgA, Test: "TestIndex/empty", Output: "    invoice_test.go:42: want 2 rows, got 0\n" }),
  JSON.stringify({ Action: "fail", Package: pkgA, Test: "TestIndex/empty", Elapsed: 0.01 }),
  JSON.stringify({ Action: "fail", Package: pkgA, Test: "TestIndex", Elapsed: 0.25 }),
  JSON.stringify({ Action: "run", Package: pkgB, Test: "TestIndex" }),
  JSON.stringify({ Action: "pass", Package: pkgB, Test: "TestIndex", Elapsed: 0.5 }),
  JSON.stringify({ Action: "fail", Package: pkgA, Elapsed: 0.3 }),
  JSON.stringify({ ImportPath: "github.com/acme/shop/tests/Unit [github.com/acme/shop/tests/Unit.test]", Action: "build-output", Output: "tests/Unit/money_test.go:3:1: syntax error\n" }),
  JSON.stringify({ ImportPath: "github.com/acme/shop/tests/Unit [github.com/acme/shop/tests/Unit.test]", Action: "build-fail" }),
];
same(m.parseTestEvent(lines[0]), undefined, "a line that is not an event is output");
same(m.parseTestEvent("{not json"), undefined, "broken JSON is output");
same(m.parseTestEvent(lines[11]).packagePath, "github.com/acme/shop/tests/Unit", "a build event names its package");

const collector = new m.ResultCollector();
const updates = lines.map(m.parseTestEvent).filter(Boolean).map((event) => collector.feed(event)).filter(Boolean);
const finals = updates.filter((update) => update.kind !== "output");
same(finals.map((update) => [update.kind, update.key ?? update.packagePath]), [
  ["started", pkgA + "#TestIndex"],
  ["failed", pkgA + "#TestIndex"],
  ["started", pkgB + "#TestIndex"],
  ["passed", pkgB + "#TestIndex"],
  ["package-failed", "github.com/acme/shop/tests/Unit"],
], "results keyed by package and top-level test; a package whose test failed is not a broken build");
const failed = finals.find((update) => update.kind === "failed");
same(failed.elapsedMs, 250, "elapsed in milliseconds");
assert(failed.output.includes("want 2 rows, got 0"), "a subtest's output belongs to its parent: " + failed.output);
same(finals.at(-1).output, "tests/Unit/money_test.go:3:1: syntax error\n", "a broken build carries what the compiler said");
same(m.failureLine(failed.output, "invoice_test.go"), 41, "the failure points at its line, zero-based");
same(m.failureLine(failed.output, "other_test.go"), undefined, "a line in another file is not this test's");
same(m.selectionKey(pkgA, "TestIndex/empty/deeper"), pkgA + "#TestIndex", "subtests key to the top-level test");
`)
}

// TestTestsRunThroughAruWithoutAShell runs the runner against a stand-in aru
// that records its arguments. Every argument arrives as it was given -- a
// shell would have run the command substitution and split the spaces -- with
// the environment and the directory the run asked for, and a cancelled run
// stops the process.
func TestTestsRunThroughAruWithoutAShell(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fake := filepath.Join(dir, "aru")
	if err := os.WriteFile(fake, []byte(`#!/bin/sh
for arg in "$@"; do printf '%s\n' "$arg" >> "$ARGS_FILE"; done
printf 'cwd=%s headed=%s\n' "$(pwd -P)" "$ARANDU_E2E_HEADED" >> "$ARGS_FILE"
echo "aru: views are up to date" >&2
echo '{"Action":"run","Package":"example.com/app/tests/E2E","Test":"TestCheckout"}'
printf '{"Action":"pass","Package":"example.com/app/tests/E2E","Test":"TestCheckout","Elapsed":1.5}'
if [ "$1" = "hang" ]; then exec sleep 30; fi
exit 3
`), 0o755); err != nil {
		t.Fatal(err)
	}
	workdir := filepath.Join(dir, "project")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	pwned := filepath.Join(dir, "pwned")
	runTestingModules(t, `
const events = [];
let output = "";
const args = ["test", "-run", "^(TestA)$", "$(touch `+pwned+`)", "two words; echo injected"];
const code = await m.runAru(env("FAKE_ARU"), args, { cwd: env("WORKDIR"), env: { ARGS_FILE: env("ARGS_FILE"), ARANDU_E2E_HEADED: "1" } }, {
  onEvent: (event) => events.push(event),
  onOutput: (text) => { output += text; },
});
same(code, 3, "the exit code is aru's");
same(events.map((event) => event.action), ["run", "pass"], "events stream, the last one without a newline included");
assert(output.includes("views are up to date"), "stderr is shown: " + output);
const recorded = fs.readFileSync(env("ARGS_FILE"), "utf8").split("\n");
same(recorded.slice(0, args.length), args, "every argument arrives as given");
same(recorded[args.length], "cwd=" + fs.realpathSync(env("WORKDIR")) + " headed=1", "directory and environment");
assert(!fs.existsSync(env("PWNED")), "a command substitution in an argument was run by a shell");

const abort = new AbortController();
const started = Date.now();
const pending = m.runAru(env("FAKE_ARU"), ["hang"], { cwd: env("WORKDIR"), env: { ARGS_FILE: env("ARGS_FILE") }, signal: abort.signal }, { onEvent: () => {}, onOutput: () => {} });
setTimeout(() => abort.abort(), 200);
same(await pending, null, "a cancelled run answers null");
assert(Date.now() - started < 10000, "cancelling stops the process");
same(await m.runAru(path.join(env("WORKDIR"), "missing-aru"), [], { cwd: env("WORKDIR"), env: {} }, { onEvent: () => {}, onOutput: () => {} }), null, "an aru that cannot start answers null");
`, "FAKE_ARU="+fake, "ARGS_FILE="+argsFile, "WORKDIR="+workdir, "PWNED="+pwned)
}

// TestThePlaywrightBrowsersAppearOnlyWhenTheModuleIsRequired fixes the driver
// picker: the native driver always, the Playwright driver's three engines only
// while go.mod requires github.com/hyz-is/arandu-playwright, and a remembered
// choice dropped when its module leaves.
func TestThePlaywrightBrowsersAppearOnlyWhenTheModuleIsRequired(t *testing.T) {
	runTestingModules(t, `
const native = [{ driver: "cdp", browser: "chromium" }];
const all = [...native, { driver: "playwright", browser: "chromium" }, { driver: "playwright", browser: "firefox" }, { driver: "playwright", browser: "webkit" }];
const block = "module example.com/app\n\ngo 1.26\n\nrequire (\n\tgithub.com/arandu-io/hesape v0.82.0\n\tgithub.com/hyz-is/arandu-playwright v0.1.0 // indirect\n)\n";
same(m.driverChoices(""), native, "no go.mod, native only");
same(m.driverChoices("module example.com/app\nrequire github.com/arandu-io/hesape v0.82.0\n"), native, "without the module, native only");
same(m.driverChoices(block), all, "required in a block");
same(m.driverChoices("module x\nrequire github.com/hyz-is/arandu-playwright v0.1.0\n"), all, "required on its own line");
same(m.driverChoices("module x\n// require github.com/hyz-is/arandu-playwright v0.1.0\n"), native, "a commented-out requirement is none");
same(m.driverChoices("module x\nreplace github.com/hyz-is/arandu-playwright => ../pw\n"), native, "a replace is not a requirement");
same(m.driverChoices("module x\nrequire github.com/hyz-is/arandu-playwright-extra v0.1.0\n"), native, "a longer module path is another module");

same(m.resolveChoice({ driver: "playwright", browser: "webkit" }, block), { driver: "playwright", browser: "webkit" }, "a valid choice is kept");
same(m.resolveChoice({ driver: "playwright", browser: "webkit" }, "module x\n"), undefined, "the module left go.mod");
same(m.resolveChoice({ driver: "cdp", browser: "firefox" }, block), undefined, "an engine the driver does not have");
same(m.resolveChoice("cdp", block), undefined, "a malformed memory");
same(m.describeChoice(undefined), "project default", "no choice");
same(m.describeChoice({ driver: "cdp", browser: "chromium" }), "cdp · chromium", "a choice");

same(m.e2eEnvironment({ driver: "playwright", browser: "firefox" }, { headed: true, traceDir: "/traces" }), {
  ARANDU_E2E_TRACE: "1",
  ARANDU_E2E_DRIVER: "playwright",
  ARANDU_E2E_BROWSER: "firefox",
  ARANDU_E2E_HEADED: "1",
  ARANDU_E2E_TRACE_DIR: "/traces",
}, "a headed debug run");
same(m.e2eEnvironment(undefined, { headed: false, traceDir: undefined }), { ARANDU_E2E_TRACE: "1" }, "a run leaves a trace even when it passes");
`)
}

// TestJourneysAreDebuggedThroughTheGoExtensionHeaded fixes the launch
// configuration: the Go extension's test mode on the package, -test.run
// anchored, -tags e2e for a journey and the window shown.
func TestJourneysAreDebuggedThroughTheGoExtensionHeaded(t *testing.T) {
	runTestingModules(t, `
const e2e = m.debugConfiguration({
  name: "Arandu: debug TestCheckout",
  packageDir: "/project/tests/E2E",
  names: ["TestCheckout"],
  tags: ["e2e", "slow"],
  e2e: true,
  env: m.e2eEnvironment({ driver: "cdp", browser: "chromium" }, { headed: true, traceDir: "/traces" }),
  marker: "run-1",
});
same(e2e, {
  type: "go",
  request: "launch",
  mode: "test",
  name: "Arandu: debug TestCheckout",
  program: "/project/tests/E2E",
  args: ["-test.run", "^(TestCheckout)$", "-test.v"],
  buildFlags: "-tags=e2e,slow",
  env: { ARANDU_E2E_TRACE: "1", ARANDU_E2E_DRIVER: "cdp", ARANDU_E2E_BROWSER: "chromium", ARANDU_E2E_HEADED: "1", ARANDU_E2E_TRACE_DIR: "/traces" },
  aranduTestRun: "run-1",
}, "a headed journey under Delve");
same(m.debugConfiguration({ name: "x", packageDir: "/p/tests/E2E", names: ["TestA"], tags: [], e2e: true, env: {}, marker: "m" }).buildFlags, "-tags=e2e", "a journey always builds with e2e");
const unit = m.debugConfiguration({ name: "x", packageDir: "/p/tests/Unit", names: ["TestB", "TestA"], tags: [], e2e: false, env: {}, marker: "m" });
same([unit.buildFlags, unit.args], [undefined, ["-test.run", "^(TestA|TestB)$", "-test.v"]], "a unit test has no tag to ask for");
`)
}

// TestTheTraceViewerReadsWhatTheTestKitWrites reads a trace the test kit's
// own Recorder wrote, and finds a test's trace where the kit puts it. The
// directory names were taken from the kit's trace.Dir.
func TestTheTraceViewerReadsWhatTheTestKitWrites(t *testing.T) {
	runTestingModules(t, `
const dir = path.join(fixtures, "trace");
const trace = m.parseTrace(JSON.parse(fs.readFileSync(path.join(dir, "trace.json"), "utf8")));
same([trace.test, trace.driver, trace.browser, trace.failed], ["TestCheckout/pays_with_card", "cdp", "chromium", true], "header");
same(trace.steps.map((step) => [step.index, step.action, step.screenshot, step.dom]), [
  [1, "visit", "steps/001-visit.png", "steps/001-visit.html"],
  [2, "click", "steps/002-click.png", "steps/002-click.html"],
], "steps with their files");
same(trace.steps[0].durationMs, 42, "durations from nanoseconds");
same(trace.steps[0].started, Date.parse("2026-10-23T18:00:00.123Z"), "nanosecond instants");
assert(trace.steps[1].error.includes("answered 500"), "the step's error");
same(trace.network.map((request) => [request.method, request.status, request.serverRequestId]), [["GET", 200, "req-aaa"], ["POST", 500, "req-bbb"]], "network");
same(trace.failures.map((failure) => failure.kind), ["5xx"], "failures");
same([...trace.server.keys()].sort(), ["req-aaa", "req-bbb"], "server sections by request id");
const denied = trace.server.get("req-bbb");
same(denied.decisions[0], { policy: "PaymentPolicy", action: "create", allowed: false, grant: "", reason: "the cart belongs to another member" }, "a Policy decision");
assert(denied.queries[0].error.includes("duplicate key"), "a failed statement");
same(denied.logs[0].level, "ERROR", "a log line");
same([...m.requestsByStep(trace)], [[1, [0]], [2, [1]]], "each request belongs to the step that was current when it started");

throws(() => m.parseTrace({ version: 2, steps: [] }), /format version 2/, "an unknown version is refused");
throws(() => m.parseTrace([]), /not a JSON object/, "not a trace");
const empty = m.parseTrace({ version: 1, test: "TestX", started: "0001-01-01T00:00:00Z" });
same([empty.started, empty.steps.length, empty.server.size], [undefined, 0, 0], "a sparse trace reads as empty, the zero time as none");

same(m.safeTracePath(dir, "steps/001-visit.png"), path.join(dir, "steps", "001-visit.png"), "a file of the trace");
for (const hostile of ["../trace.json", "steps/../../x.png", "/etc/passwd", "steps\\..\\..\\x", "", ".", "..", "a\0b"]) {
  same(m.safeTracePath(dir, hostile), undefined, "refused: " + JSON.stringify(hostile));
}

const kit = {
  "TestCheckout": "/base/TestCheckout",
  "TestCheckout/pays_with_card": "/base/TestCheckout/pays_with_card",
  "TestJourney/a b/c:d": "/base/TestJourney/a-b/c-d",
  "TestW/-.-": "/base/TestW/step",
  ["TestX/" + "a".repeat(70) + "-tail"]: "/base/TestX/" + "a".repeat(60),
  ["TestY/" + "b".repeat(59) + "..zzz"]: "/base/TestY/" + "b".repeat(59),
  "TestZ/!!!": "/base/TestZ/step",
  "Test..hidden--": "/base/Test..hidden",
  "TestÜber/çã": "/base/Test-ber/step",
};
for (const [name, want] of Object.entries(kit)) {
  same(m.traceDirectory("/base", name), want, "trace directory of " + name);
}
same(m.traceBases("/editor", { ARANDU_E2E_TRACE_DIR: "/custom" }, "/tmp"), ["/editor", "/custom", path.join("/tmp", "arandu-e2e")], "every place a trace may be, most specific first");
same(m.traceBases(undefined, {}, "/tmp"), [path.join("/tmp", "arandu-e2e")], "the kit's default");
`)
}

// TestTheTraceViewerRunsUnderAStrictContentSecurityPolicy renders a trace full
// of hostile text and checks the page: one policy, nothing inline without the
// nonce, no handler attribute, no remote origin, and every value from the
// trace escaped -- the DOM shown as source, never rendered.
func TestTheTraceViewerRunsUnderAStrictContentSecurityPolicy(t *testing.T) {
	runTestingModules(t, `
const hostile = "<script>alert(1)</script><img src=x onerror=alert(2)>";
const raw = JSON.parse(fs.readFileSync(path.join(fixtures, "trace", "trace.json"), "utf8"));
raw.test = "TestEvil/" + hostile;
raw.steps[1].error = hostile;
raw.steps[1].target = "\"><svg onload=alert(3)>";
raw.console[0].text = hostile;
raw.console[0].level = "error\" onclick=\"alert(4)";
raw.network[1].url = "http://127.0.0.1/" + hostile;
raw.server["req-bbb"].queries[0].sql = "SELECT '</code></pre>" + hostile + "'";
raw.server["req-bbb"].logs[0].attrs = { html: hostile };
raw.server["req-evil\" data-x=\"1"] = { logs: [], queries: [], decisions: [] };
const trace = m.parseTrace(raw);
const nonce = "bm9uY2Utb2YtdGhlLXBhZ2U=";
const html = m.renderTraceHtml(trace, {
  cspSource: "https://file+.vscode-resource.vscode-cdn.net",
  nonce,
  image: (relative) => relative === "steps/001-visit.png" ? "https://file+.vscode-resource.vscode-cdn.net/trace/steps/001-visit.png" : undefined,
  dom: (relative) => relative === "steps/002-click.html" ? "<html><script>steal()</script></html>" : undefined,
});

const policy = "default-src 'none'; img-src https://file+.vscode-resource.vscode-cdn.net; style-src 'nonce-" + nonce + "'; script-src 'nonce-" + nonce + "'; base-uri 'none'; form-action 'none'";
same(m.traceContentSecurityPolicy("https://file+.vscode-resource.vscode-cdn.net", nonce), policy, "the policy");
assert(html.includes('<meta http-equiv="Content-Security-Policy" content="' + policy.replace(/'/g, "&#39;") + '">'), "the page carries the policy");
for (const loose of ["unsafe-inline", "unsafe-eval", "<iframe", "<form", "<object", "<embed", "<link", "<base", "href=", "javascript:"]) {
  assert(!html.includes(loose), "the page contains " + loose);
}
const tags = [...html.matchAll(/<([a-zA-Z][a-zA-Z0-9-]*)((?:\s+[^\s=>]+(?:="[^"]*")?)*)\s*>/g)];
assert(tags.length > 20, "the page has its structure");
for (const [whole, name, attributes] of tags) {
  for (const attribute of attributes.matchAll(/\s+([^\s=>]+)/g)) {
    assert(!/^on/i.test(attribute[1]), "a handler attribute reached the page: " + whole);
  }
  if (name === "script" || name === "style") {
    assert(attributes.trim() === 'nonce="' + nonce + '"', name + " without the page's nonce: " + whole);
  }
  if (name === "img") {
    assert(/src="https:\/\/file\+\.vscode-resource\.vscode-cdn\.net\//.test(attributes), "an image from outside the trace: " + whole);
  }
}
same((html.match(/<script/g) || []).length, 1, "one script");
same((html.match(/<style/g) || []).length, 1, "one style");
assert(!html.includes("<script>alert") && !html.includes("<img src=x") && !html.includes("<svg"), "hostile markup reached the page");
assert(html.includes("&lt;script&gt;alert(1)&lt;/script&gt;&lt;img src=x onerror=alert(2)&gt;"), "hostile text is shown escaped");
assert(html.includes("&lt;html&gt;&lt;script&gt;steal()&lt;/script&gt;&lt;/html&gt;"), "the DOM is shown as source");
assert(html.includes('data-server="req-evil&quot; data-x=&quot;1"'), "a request id cannot open an attribute");
assert(!/class="level-[^"]*"[^>]*onclick/.test(html), "a log level cannot add an attribute");

assert(html.includes('<body data-selected="2">'), "the failing step is selected first");
assert(html.includes('<button type="button" data-command="explain">'), "a failed trace offers Explain failure");
assert(html.includes("No screenshot was taken after this step."), "a missing screenshot is said, not drawn");
assert(/<tr data-steps="2" class="status-5xx current">/.test(html), "the 5xx of the selected step is marked");
for (const server of ["Policy decisions", "PaymentPolicy", "denied", "the cart belongs to another member", "SQL", "SELECT id, total FROM carts", "SELECT '&lt;/code&gt;&lt;/pre&gt;", "duplicate key", "Logs", "charge failed", "grant:cart:view:tenant=7"]) {
  assert(html.includes(server), "the server panel is missing " + server);
}
assert(/<article data-server="req-bbb" data-steps="2">/.test(html), "the denied request belongs to the failing step and is shown");
assert(/<article data-server="req-aaa" data-steps="1" hidden>/.test(html), "another step's request starts hidden");
for (const tab of ["screenshot", "dom", "network", "console", "server"]) {
  assert(html.includes('data-tab="' + tab + '"') && html.includes('data-panel="' + tab + '"'), "the " + tab + " tab");
}

const passed = m.renderTraceHtml(m.parseTrace({ version: 1, test: "TestOK", failed: false, steps: [] }), { cspSource: "src", nonce, image: () => undefined, dom: () => undefined });
assert(!passed.includes('<button type="button" data-command="explain">'), "a passing trace has no failure to explain");
assert(passed.includes("The test recorded no step.") && passed.includes("The trace has no server section"), "empty states are said");
same([m.formatDuration(0.5), m.formatDuration(42), m.formatDuration(1500)], ["0.50 ms", "42 ms", "1.50 s"], "durations");
`)
}

// TestRecordingReusesTheRunningDevelopmentServer fixes where aru e2e:record
// starts: the address the project's .env gives the development server when
// something answers there, a typed path joined to it, and a route name or an
// address handed to aru as typed.
func TestRecordingReusesTheRunningDevelopmentServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	open := listener.Addr().(*net.TCPAddr).Port

	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := closedListener.Addr().(*net.TCPAddr).Port
	if err := closedListener.Close(); err != nil {
		t.Fatal(err)
	}

	runTestingModules(t, `
same(m.devServerURL("APP_URL=http://localhost:8080\nHTTP_ADDR=:9000\n"), "http://localhost:8080", "APP_URL first");
same(m.devServerURL("APP_URL=\"https://shop.test/\" # local\n"), "https://shop.test", "quoted, trailing slash trimmed");
same(m.devServerURL("APP_URL=\nHTTP_ADDR=:3000 # dev\n"), "http://localhost:3000", "the bound port");
same(m.devServerURL("export HTTP_ADDR=127.0.0.1:4000\n"), "http://127.0.0.1:4000", "a bound host");
same(m.devServerURL("HTTP_ADDR=0.0.0.0:5000\n"), "http://localhost:5000", "every interface is reached on localhost");
same(m.devServerURL("# nothing\n"), "http://localhost:8080", "the skeleton's default");

same(m.recordTarget("", "http://localhost:8080"), "http://localhost:8080", "empty starts from the running server");
same(m.recordTarget("  ", undefined), undefined, "empty with none running lets aru serve the app");
same(m.recordTarget("/login", "http://localhost:8080/"), "http://localhost:8080/login", "a path joins the running server");
same(m.recordTarget("/login", undefined), "/login", "a path without a server is aru's to resolve");
same(m.recordTarget("checkout.show", "http://localhost:8080"), "checkout.show", "a route name is aru's to resolve");
same(m.recordTarget("https://staging.shop.test/x", "http://localhost:8080"), "https://staging.shop.test/x", "an address is used as typed");

same(await m.isListening("http://127.0.0.1:" + env("OPEN_PORT")), true, "a server that answers is found");
same(await m.isListening("http://127.0.0.1:" + env("CLOSED_PORT"), 300), false, "a closed port is not a server");
same(await m.isListening("not a url"), false, "an address that does not parse is not a server");
`, "OPEN_PORT="+strconv.Itoa(open), "CLOSED_PORT="+strconv.Itoa(closed))
}
