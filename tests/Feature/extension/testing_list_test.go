package extension_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// listFixture is what aru test --list --json printed for testdata/testlist,
// written by aru v0.79.0 (go build -ldflags "-X main.version=v0.79.0" at the
// commit that added the flag, then aru test --list --json in the fixture
// project). The fixture project is the same set of files the editor's own
// reading is checked against, so the two can be compared.
const listFixture = "testList.v1.aru-0.79.json"

// TestTheTestingViewListsTestsFromAruTestListJSON reads what aru printed: the
// layers it names, each file with its package and tags, each test at its
// 1-based line, the subtests under it, and the tests beside the code under
// Other.
func TestTheTestingViewListsTestsFromAruTestListJSON(t *testing.T) {
	runTestingModules(t, `
const listing = m.listingFromInventory(fs.readFileSync(path.join(fixtures, env("LIST_FIXTURE")), "utf8"));
same(listing.source, "aru", "the listing says where it came from");
same(listing.layers, [
  { id: "unit", label: "Unit", directory: "tests/Unit" },
  { id: "feature", label: "Feature", directory: "tests/Feature" },
  { id: "contract", label: "Contract", directory: "tests/Contract" },
  { id: "e2e", label: "E2E", directory: "tests/E2E" },
  { id: "conformance", label: "Conformance", directory: "tests/Conformance" },
  { id: "other", label: "Other", directory: "" },
], "the layers aru names, with the ids the run and the lenses use");
same(listing.layers.slice(0, 5), m.testLayers, "aru's five layers are the editor's");
same(listing.files.map((file) => [file.layer.id, file.relative, file.packagePath, file.tags]), [
  ["unit", "tests/Unit/main_test.go", "example.com/shop/tests/Unit", []],
  ["unit", "tests/Unit/money_test.go", "example.com/shop/tests/Unit", []],
  ["feature", "tests/Feature/billing/invoice_test.go", "example.com/shop/tests/Feature/billing", ["integration"]],
  ["contract", "tests/Contract/legacy_test.go", "example.com/shop/tests/Contract", ["ignore"]],
  ["contract", "tests/Contract/openapi_test.go", "example.com/shop/tests/Contract", ["openapi_strict"]],
  ["e2e", "tests/E2E/checkout_test.go", "example.com/shop/tests/E2E", ["e2e"]],
  ["conformance", "tests/Conformance/auth_test.go", "example.com/shop/tests/Conformance", ["conformance"]],
  ["other", "app/models/user_internal_test.go", "example.com/shop/app/models", []],
], "one entry per file, in aru's order, with what running it needs");
const money = listing.files[1];
same(money.tests.map((test) => [test.name, test.line]), [
  ["TestMoney", 7], ["Test_underscore", 35], ["Test", 37], ["TestNoName", 39], ["TestLogin", 45], ["TestAfterString", 47],
], "tests at aru's 1-based lines");
same(money.tests[0].subtests, [
  { name: "TestMoney/adds_cents", line: 8 },
  { name: "TestMoney/adds_cents/carries", line: 9 },
  { name: "TestMoney/adds_cents#01", line: 11 },
], "subtests depth first, a repeat numbered, a name known only at run time left out");
same(m.editorLine(money.tests[0].line), 6, "the view puts TestMoney on the editor's line 6");
same(listing.unreadable, [], "nothing unreadable");

const base = JSON.parse(fs.readFileSync(path.join(fixtures, env("LIST_FIXTURE")), "utf8"));
const variant = (change) => { const copy = JSON.parse(JSON.stringify(base)); change(copy); return JSON.stringify(copy); };
const extra = m.listingFromInventory(variant((inv) => { inv.added = true; inv.tests[0].owner = "x"; inv.unreadable = [{ file: "tests/Unit/broken_test.go", error: "expected ';'" }]; }));
same(extra.files.length, 8, "a field this editor does not know is ignored");
same(extra.unreadable, [{ file: "tests/Unit/broken_test.go", error: "expected ';'" }], "an unreadable file is passed on");
const refusals = [
  ["not json", () => "ok  example.com/shop 0.1s", /not JSON/],
  ["another version", () => variant((inv) => { inv.version = 2; }), /format version 2.*reads 1/],
  ["no version", () => variant((inv) => { delete inv.version; }), /format version undefined/],
  ["an array", () => "[]", /not a JSON object/],
  ["tests missing", () => variant((inv) => { delete inv.tests; }), /tests is not an array/],
  ["null subtests", () => variant((inv) => { inv.tests[0].subtests = null; }), /tests\[0\]\.subtests is not an array/],
  ["null tags", () => variant((inv) => { inv.tests[0].tags = null; }), /tests\[0\]\.tags is not an array/],
  ["zero line", () => variant((inv) => { inv.tests[0].line = 0; }), /tests\[0\]\.line is not a 1-based line/],
  ["fractional line", () => variant((inv) => { inv.tests[2].subtests[0].line = 1.5; }), /subtests\[0\]\.line is not a 1-based line/],
  ["undeclared layer", () => variant((inv) => { inv.tests[0].layer = "Browser"; }), /layer the inventory does not declare/],
  ["climbing out", () => variant((inv) => { inv.tests[0].file = "../outside_test.go"; }), /not a test file inside the project/],
  ["absolute", () => variant((inv) => { inv.tests[0].file = "/etc/x_test.go"; }), /not a test file inside the project/],
  ["backslash", () => variant((inv) => { inv.tests[0].file = "tests\\Unit\\x_test.go"; }), /not a test file inside the project/],
  ["not a test file", () => variant((inv) => { inv.tests[0].file = "tests/Unit/main.go"; }), /not a test file inside the project/],
  ["not a test name", () => variant((inv) => { inv.tests[0].name = "Helper"; }), /not a test function/],
  ["foreign subtest", () => variant((inv) => { inv.tests[2].subtests[0].name = "TestOther/x"; }), /not a subtest of TestMoney/],
  ["layer outside", () => variant((inv) => { inv.layers[0].dir = "../tests"; }), /not a layer of the project/],
];
for (const [what, text, pattern] of refusals) {
  throws(() => m.listingFromInventory(text()), pattern, "refused: " + what);
}
`, "LIST_FIXTURE="+listFixture)
}

// TestTheScannerAgreesWithAruTestList reads the fixture project's files the
// way an editor with an older aru does, and finds in the five layers exactly
// what aru listed: the same files, packages, tags -- ignore among them -- and
// tests at the same lines. Only the subtests and the Other layer are aru's
// alone.
func TestTheScannerAgreesWithAruTestList(t *testing.T) {
	project := rootPath(t, "tests", "Feature", "extension", "testdata", "testlist")
	var files []string
	err := filepath.WalkDir(filepath.Join(project, "tests"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		relative, err := filepath.Rel(project, path)
		files = append(files, filepath.ToSlash(relative))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	runTestingModules(t, `
const project = env("PROJECT");
const files = JSON.parse(env("FILES")).sort().map((relative) => ({ relative, source: fs.readFileSync(path.join(project, relative), "utf8") }));
const scanned = m.scannedListing(files, m.moduleOf(fs.readFileSync(path.join(project, "go.mod"), "utf8")));
const listed = m.listingFromInventory(fs.readFileSync(path.join(fixtures, env("LIST_FIXTURE")), "utf8"));
same(scanned.source, "scanner", "the listing says where it came from");
same(scanned.layers, m.testLayers, "the scanner knows the five layers");
const shape = (listing) => listing.files
  .filter((file) => file.layer.directory !== "")
  .map((file) => ({ layer: file.layer.id, relative: file.relative, packagePath: file.packagePath, tags: file.tags, tests: file.tests.map((test) => [test.name, test.line]) }));
same(shape(scanned), shape(listed), "the scanner and aru agree on every file of the five layers");
assert(scanned.files.every((file) => file.tests.every((test) => test.subtests.length === 0)), "the scanner reads no subtests");
same(listed.files.filter((file) => file.layer.directory === "").map((file) => file.relative), ["app/models/user_internal_test.go"], "the tests beside the code are aru's alone");

same(m.scannedFile("tests/Unit/x_test.go", undefined, "package unit\nfunc TestX(t *testing.T) {}\n").packagePath, "", "no module, no package");
same(m.scannedFile("app/x_test.go", "example.com/shop", "package app\nfunc TestX(t *testing.T) {}\n"), undefined, "the scanner reads only the five layers");
`, "LIST_FIXTURE="+listFixture, "PROJECT="+project, "FILES="+jsonStrings(t, files))
}

// TestOnlyAnAruThatListsTestsIsAskedTo fixes which aru version gets asked for
// --list --json. An older one hands the flags to go test, which compiles the
// project and runs its test binaries, so an aru that does not say it is
// v0.79.0 or later is never asked.
func TestOnlyAnAruThatListsTestsIsAskedTo(t *testing.T) {
	runTestingModules(t, `
for (const [reported, want] of [
  ["aru v0.79.0", true],
  ["aru 0.79.0", true],
  ["aru v0.79.0\n", true],
  ["aru v0.80.2", true],
  ["aru v1.0.0", true],
  ["aru v0.79.1-0.20261024120000-abcdef123456", true],
  ["aru v0.79.0+dirty", true],
  ["aru 0.78.0", false],
  ["aru v0.78.9", false],
  ["aru v0.78.1-0.20261023120000-abcdef123456", false],
  ["aru v0.79.0-rc.1", false],
  ["aru dev", false],
  ["aru (devel)", false],
  ["", false],
  ["v0.79", false],
  ["aru v00.79.0", false],
]) {
  same(m.listsTests(reported), want, "listsTests(" + JSON.stringify(reported) + ")");
}
`)
}

// TestDiscoveryAsksAruAndFallsBack runs the discovery against a stand-in aru
// that answers as each kind of aru does. Only an aru reporting v0.79.0 or
// later is asked to list, without a shell and from the project's root; every
// other answer is a reason to read the sources instead, and an old aru never
// sees --list.
func TestDiscoveryAsksAruAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fake := filepath.Join(dir, "aru")
	if err := os.WriteFile(fake, []byte(`#!/bin/sh
printf '%s|%s\n' "$(pwd -P)" "$*" >> "$ARGS_FILE"
case "$1" in
version)
  case "$FAKE_MODE" in
    old) echo "aru 0.78.0" ;;
    dev) echo "aru dev" ;;
    versionfails) echo "boom" >&2; exit 1 ;;
    *) echo "aru v0.79.0" ;;
  esac
  exit 0 ;;
test)
  case "$FAKE_MODE" in
    unknownflag) echo "flag provided but not defined: -list" >&2; echo "second line" >&2; exit 2 ;;
    garbage) echo "ok  	example.com/shop	0.1s"; exit 0 ;;
    future) echo '{"version":2,"tests":[]}'; exit 0 ;;
    *) cat "$FIXTURE"; exit 0 ;;
  esac ;;
esac
exit 9
`), 0o755); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	runTestingModules(t, `
const calls = () => { try { return fs.readFileSync(env("ARGS_FILE"), "utf8").trim().split("\n"); } catch { return []; } finally { fs.rmSync(env("ARGS_FILE"), { force: true }); } };
const ask = async (mode) => { process.env.FAKE_MODE = mode; const answer = await m.listWithAru(env("FAKE_ARU"), env("PROJECT")); return [answer, calls()]; };
const root = fs.realpathSync(env("PROJECT"));

let [answer, seen] = await ask("ok");
same(seen, [root + "|version", root + "|test --list --json"], "the version first, then the listing, both from the project root");
same([answer.version, answer.listing.source, answer.listing.files.length], ["aru v0.79.0", "aru", 8], "the listing is aru's");

for (const [mode, reason] of [
  ["old", /^aru 0\.78\.0 cannot list tests as JSON; aru test --list --json arrived in v0\.79\.0$/],
  ["dev", /^aru dev cannot list tests as JSON/],
  ["versionfails", /^aru version exited with status 1$/],
]) {
  [answer, seen] = await ask(mode);
  same(answer.listing, undefined, mode + ": no listing");
  assert(reason.test(answer.reason), mode + ": reason " + answer.reason);
  same(seen, [root + "|version"], mode + ": an aru that may not have --list is never asked for it");
}
for (const [mode, reason] of [
  ["unknownflag", /^aru test --list --json exited with status 2: flag provided but not defined: -list$/],
  ["garbage", /^aru test --list --json printed no inventory this editor reads: the output is not JSON/],
  ["future", /^aru test --list --json printed no inventory this editor reads: the inventory is format version 2/],
]) {
  [answer, seen] = await ask(mode);
  same(answer.listing, undefined, mode + ": no listing");
  assert(reason.test(answer.reason), mode + ": reason " + answer.reason);
  same(seen.length, 2, mode + ": asked once");
}
const missing = await m.listWithAru(path.join(env("PROJECT"), "no-aru"), env("PROJECT"));
assert(missing.listing === undefined && /^aru version stopped: spawn .*ENOENT/.test(missing.reason), "an aru that cannot start: " + missing.reason);
`, "FAKE_ARU="+fake, "ARGS_FILE="+argsFile, "PROJECT="+project, "FIXTURE="+rootPath(t, "tests", "Feature", "extension", "testdata", listFixture))
}

// TestAShortAruRunIsCappedAndNeverThroughAShell runs captureAru, which the
// discovery reads aru with, against a stand-in: arguments arrive as given, a
// run past the time limit or the byte limit is stopped, and stderr is kept
// apart from stdout.
func TestAShortAruRunIsCappedAndNeverThroughAShell(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "aru")
	if err := os.WriteFile(fake, []byte(`#!/bin/sh
case "$1" in
hang) exec sleep 30 ;;
flood) yes 0123456789 ;;
*) for arg in "$@"; do printf '%s\n' "$arg"; done; echo "said" >&2; exit 4 ;;
esac
`), 0o755); err != nil {
		t.Fatal(err)
	}
	pwned := filepath.Join(dir, "pwned")
	runTestingModules(t, `
const options = { cwd: env("DIR"), timeoutMs: 5000, maxBytes: 1 << 20 };
const args = ["test", "$(touch `+pwned+`)", "two words; echo injected"];
const plain = await m.captureAru(env("FAKE_ARU"), args, options);
same([plain.code, plain.stdout, plain.stderr, plain.problem], [4, args.join("\n") + "\n", "said\n", ""], "exit code, stdout and stderr, every argument as given");
assert(!fs.existsSync(env("PWNED")), "a command substitution in an argument was run by a shell");

const started = Date.now();
const hung = await m.captureAru(env("FAKE_ARU"), ["hang"], { ...options, timeoutMs: 200 });
same([hung.code, hung.problem], [null, "it did not finish within 200 ms"], "a run past the time limit is stopped");
assert(Date.now() - started < 10000, "stopping ends the process");
const flood = await m.captureAru(env("FAKE_ARU"), ["flood"], { ...options, maxBytes: 4096 });
same([flood.code, flood.problem], [null, "it printed more than 4096 bytes"], "a run past the byte limit is stopped");
assert(flood.stdout.length <= 4096, "nothing past the limit is kept");
`, "FAKE_ARU="+fake, "DIR="+dir, "PWNED="+pwned)
}

func jsonStrings(t *testing.T, values []string) string {
	t.Helper()
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
