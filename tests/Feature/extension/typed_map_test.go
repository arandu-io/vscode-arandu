package extension_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// runExtensionModules bundles the editor-free modules of the extension and
// runs a script against them in Node, with the bundle's exports as `m` and
// the test's fixtures directory as `fixtures`. A thrown error fails the test
// with the script's output.
func runExtensionModules(t *testing.T, script string) {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "modules.cjs")
	build := exec.Command(
		filepath.Join(rootPath(t), "node_modules", ".bin", "esbuild"),
		"--bundle",
		"--platform=node",
		"--format=cjs",
		"--log-level=warning",
		"--sourcefile=modules.ts",
		"--outfile="+bundle,
	)
	build.Dir = rootPath(t)
	build.Stdin = strings.NewReader(strings.Join([]string{
		`export * from "./src/projectGraphSchema";`,
		`export * from "./src/projectGraphProtocol";`,
		`export * from "./src/projectMapModel";`,
		`export * from "./src/catalog";`,
	}, "\n"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("bundle extension modules: %v\n%s", err, output)
	}
	check := exec.Command("node", "-e", `
const m = require(process.env.ARANDU_MODULES);
const fs = require("node:fs");
const path = require("node:path");
const fixture = (name) => JSON.parse(fs.readFileSync(path.join(process.env.ARANDU_FIXTURES, name), "utf8"));
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
`+script)
	check.Env = append(os.Environ(),
		"ARANDU_MODULES="+bundle,
		"ARANDU_FIXTURES="+rootPath(t, "tests", "Feature", "extension", "testdata"),
	)
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

// TestTheProjectMapReadsTheTypedSchemaFromTheHandoffSamples parses the
// samples aru's side of the typed map published, one node and one edge of
// each kind, and reads them the way the tree does.
func TestTheProjectMapReadsTheTypedSchemaFromTheHandoffSamples(t *testing.T) {
	runExtensionModules(t, `
const raw = fixture("projectGraph.v2.json");
const graph = m.parseProjectGraph(raw, 2);
same(graph.schemaVersion, 2, "schema");
same(graph.profile, "conventional", "profile");
same(graph.groups.map((group) => group.id), raw.groups.map((group) => group.id), "groups are the response's, in its order");
same(graph.groups.length, 12, "the samples carry twelve groups");

// The number of groups is the server's: two fewer and one unheard of still parse,
// and are read as they came.
const reshaped = JSON.parse(JSON.stringify(raw));
reshaped.groups = reshaped.groups.filter((group) => group.id !== "console" && group.id !== "community-modules");
reshaped.groups.push({ id: "schedules", label: "Schedules", nodeIds: [] });
same(m.parseProjectGraph(reshaped, 2).groups.map((group) => group.id), reshaped.groups.map((group) => group.id), "eleven reshaped groups");

const byLabel = new Map(graph.nodes.map((node) => [node.label, node]));
const route = byLabel.get("GET /tasks/{task}");
same([route.method, route.pattern, route.name, route.nestedUnder], ["GET", "/tasks/{task}", "projects.tasks.show", "projects"], "route fields");
const routeRow = m.presentNode(route);
same(routeRow.label, "GET /tasks/{task}", "route label");
assert(routeRow.description.startsWith("projects.tasks.show"), "route row shows its name: " + routeRow.description);

const controller = byLabel.get("ExportController");
same(controller.variant, "invokable", "controller variant");
same(m.presentNode(controller).description, "invokable", "controller row shows its variant");
same(m.nodeLocation(controller), { file: "file:///ROOT/app/Http/Controllers/ExportController.go", line: 28, column: 0, endLine: 55, endColumn: 1 }, "a node reveals its whole range");

const generated = byLabel.get("InvoiceQuery");
same(generated.generated, true, "generated flag");
assert(m.presentNode(generated).description.includes("generated"), "generated row is marked");
assert(m.presentNode(byLabel.get("Invoice")).description === undefined || !m.presentNode(byLabel.get("Invoice")).description.includes("generated"), "a hand-written model is not marked");

const diagnostic = graph.nodes.find((node) => node.kind === "diagnostic");
same([diagnostic.level, diagnostic.rule, diagnostic.endColumn], ["warning", "generated-not-wired", 46], "diagnostic fields");

const model = m.buildProjectMapModel(graph);
same(model.children.get(controller.id), [byLabel.get("ExportController.Invoke").id], "containment stays the tree");
const routeRelations = model.relations.get(route.id);
same(routeRelations.map((group) => group.kind), ["routes-to"], "route edges grouped by kind");
const meaning = raw.edgeKinds.find((entry) => entry.kind === "routes-to").meaning;
same(routeRelations[0].meaning, meaning, "the kind's meaning comes from edgeKinds");
same(m.relationLocation(routeRelations[0].relations[0]), raw.edges[0].at, "an edge reveals where it was read");
const showRelations = model.relations.get(byLabel.get("TaskController.Show").id);
same(showRelations[0].relations.map((relation) => [relation.direction, relation.other.label]), [["incoming", "GET /tasks/{task}"]], "the other end sees the edge too");
const kinds = new Set();
for (const groups of model.relations.values()) { for (const group of groups) { kinds.add(group.kind); } }
same([...kinds].sort(), ["authorizes", "dispatches", "listens-to", "persists", "renders", "routes-to", "tested-by", "validates-with"], "every typed edge is shown");
assert(!kinds.has("contains"), "containment is the tree, not a relation group");

const unranged = JSON.parse(JSON.stringify(raw));
delete unranged.nodes[0].endLine;
throws(() => m.parseProjectGraph(unranged, 2), /endLine must be a zero-based integer/, "a typed node without its range");
const dangling = JSON.parse(JSON.stringify(raw));
dangling.edges[0].to = "action:missing";
throws(() => m.parseProjectGraph(dangling, 2), /references unknown node action:missing/, "an edge to nowhere");
`)
}

// TestTheProjectMapFallsBackToSchemaOneForAnOlderAru holds an aru that
// predates the typed map to exactly the request and the reading of before,
// on the map aru v0.64.0 answered for the skeleton.
func TestTheProjectMapFallsBackToSchemaOneForAnOlderAru(t *testing.T) {
	runExtensionModules(t, `
for (const capabilities of [undefined, null, {}, { experimental: {} }, { experimental: { aranduProjectGraphSchemas: [1] } }]) {
  const features = m.readServerFeatures(capabilities);
  same(features, { projectGraphSchema: 1, catalog: false, doctorDiagnostics: false }, "features of " + JSON.stringify(capabilities));
  same(m.projectGraphParams(features), undefined, "an older aru is asked with no parameters");
}
const typed = m.readServerFeatures({ experimental: { aranduProjectGraphSchemas: [1, 2], aranduCatalog: true, aranduDoctorDiagnostics: true } });
same(typed, { projectGraphSchema: 2, catalog: true, doctorDiagnostics: true }, "features aru v0.65.0 reports");
same(m.projectGraphParams(typed), { schemaVersion: 2 }, "the typed schema is asked for by number");

const raw = fixture("projectGraph.v1.aru-0.64.json");
const graph = m.parseProjectGraph(raw, 1);
same(graph.schemaVersion, 1, "schema");
same(graph.groups.map((group) => group.id), ["application-features", "http", "database", "views", "async", "console", "native-screens", "native-capabilities", "community-modules", "diagnostics"], "the ten groups of schema 1");
same(graph.edgeKinds, [], "schema 1 describes no edge kinds");
const model = m.buildProjectMapModel(graph);
same(model.relations.size, 0, "schema 1 has containment only");
const located = graph.nodes.find((node) => node.file !== undefined && node.line !== undefined);
const at = m.nodeLocation(located);
same([at.endLine, at.endColumn], [at.line, at.column], "a schema 1 node opens at its start, as before");

throws(() => m.parseProjectGraph(raw, 2), /schema 1; expected 2/, "a schema 1 answer to the typed request");
throws(() => m.parseProjectGraph(fixture("projectGraph.v2.json"), 1), /schema 2; expected 1/, "a typed answer to the first request");
const reordered = JSON.parse(JSON.stringify(raw));
reordered.groups.reverse();
throws(() => m.parseProjectGraph(reordered, 1), /Unexpected project graph group 0/, "schema 1 keeps its fixed groups");
`)

	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		"this.serverFeatures = readServerFeatures(client.initializeResult?.capabilities);",
		"const params = projectGraphParams(features);",
		"? await client.sendRequest<unknown>(graphContract.request)\n",
		": await client.sendRequest<unknown>(graphContract.request, params);",
		"const graph = parseProjectGraph(response, features.projectGraphSchema);",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("project map request does not contain %q", seam)
		}
	}
}

// TestDoctorFindingsAreNeverDrawnTwice holds the extension's own Doctor
// collection to an aru that does not publish the findings itself.
func TestDoctorFindingsAreNeverDrawnTwice(t *testing.T) {
	runExtensionModules(t, `
same(m.initializationOptions, { doctorDiagnostics: true }, "the server is asked to publish the findings");
const graph = m.parseProjectGraph(fixture("projectGraph.v2.json"), 2);
const publishing = m.readServerFeatures({ experimental: { aranduProjectGraphSchemas: [1, 2], aranduDoctorDiagnostics: true } });
same(m.doctorFindings(graph, publishing), [], "nothing is drawn when the server publishes");

const silent = m.readServerFeatures({ experimental: { aranduProjectGraphSchemas: [1, 2], aranduDoctorDiagnostics: false } });
const findings = m.doctorFindings(graph, silent);
same(findings.map((finding) => [finding.code, finding.level, finding.line, finding.column, finding.endLine, finding.endColumn]), [["generated-not-wired", "warning", 40, 0, 40, 46]], "a typed finding keeps its rule and range");
assert(findings[0].codeHref.startsWith("https://"), "the rule's documentation travels with it");

const first = m.parseProjectGraph({
  schemaVersion: 1,
  groups: fixture("projectGraph.v1.aru-0.64.json").groups.map((group) => ({ ...group, nodeIds: group.id === "diagnostics" ? ["d"] : [] })),
  nodes: [{ id: "d", kind: "diagnostic", label: "finding", file: "file:///ROOT/main.go", line: 3, column: 2, level: "error" }],
  edges: [],
}, 1);
same(m.doctorFindings(first, m.readServerFeatures(undefined)).map((finding) => [finding.code, finding.level, finding.line, finding.column, finding.endLine, finding.endColumn]), [["diagnostic", "error", 3, 2, 3, 3]], "a schema 1 finding is drawn as before");
`)

	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		"      initializationOptions,\n",
		"for (const finding of doctorFindings(graph, this.serverFeatures)) {",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("Doctor diagnostics do not contain %q", seam)
		}
	}
	if count := strings.Count(source, "this.doctorDiagnostics.set("); count != 1 {
		t.Errorf("Doctor diagnostics are set in %d places, want only the one fed by doctorFindings", count)
	}
}

// TestTheEditorReadsDirectivesFromTheCatalogue holds the directive vocabulary
// to the running aru's catalogue, with the extension's own list read in one
// place and only for an aru that does not answer one.
func TestTheEditorReadsDirectivesFromTheCatalogue(t *testing.T) {
	var grammar struct {
		Repository map[string]struct {
			Patterns []struct {
				Match string `json:"match"`
			} `json:"patterns"`
		} `json:"repository"`
	}
	readJSON(t, "syntaxes/kyse.tmLanguage.json", &grammar)
	var grammarDirectives []string
	for _, pattern := range grammar.Repository["known-directives"].Patterns {
		grammarDirectives = append(grammarDirectives, strings.TrimSuffix(strings.TrimPrefix(pattern.Match, "@"), `\b`))
	}
	sort.Strings(grammarDirectives)

	runExtensionModules(t, `
const served = m.parseCatalog(fixture("catalog.json"));
same(served.source, "aru", "source");
same(served.directives.length, 24, "the directives aru v0.65.0 lists");
const fallback = m.fallbackCatalog();
same(fallback.source, "fallback", "fallback source");
same(fallback.commands, [], "no command list is kept by hand");
same(fallback.directives.map((directive) => directive.name).sort(), `+jsonArray(grammarDirectives)+`, "the fallback list matches the grammar");

// A directive only the server knows is found in what the server served, and
// not in the fallback: the served list is the one read.
const extended = m.parseCatalog({ directives: [...fixture("catalog.json").directives, { name: "fragment", kind: "block", closedBy: "endfragment" }], commands: [] });
const found = m.directiveAt(extended, "  @fragment('x')", 5);
assert(found !== undefined, "the served directive fragment is not found: the served list was not read");
same([found.directive.name, found.start, found.end], ["fragment", 2, 11], "the served directive is found");
same(m.directiveAt(fallback, "  @fragment('x')", 5), undefined, "the fallback does not know it");
same(m.describeDirective(found.directive), "`+"`@fragment`"+` opens a Kyse block that `+"`@endfragment`"+` closes.", "block hover");
same(m.describeDirective(served.directives.find((d) => d.name === "endif")), "`+"`@endif`"+` closes the Kyse block `+"`@if`"+` opened.", "end hover");
same(m.directiveAt(served, "mail me at someone@if.example", 21), undefined, "an address is not a directive");
throws(() => m.parseCatalog({ directives: "if", commands: [] }), /catalog directives must be an array/, "a malformed catalogue");
`)

	sources, err := filepath.Glob(rootPath(t, "src", "*.ts"))
	if err != nil {
		t.Fatal(err)
	}
	handList := regexp.MustCompile(`["'](endforeach|endforelse|endsection)["']`)
	for _, file := range sources {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		name := filepath.Base(file)
		if strings.Contains(source, "catalogFallback.json") && name != "catalog.ts" {
			t.Errorf("%s reads the fallback list; only catalog.ts may", name)
		}
		if handList.MatchString(source) {
			t.Errorf("%s spells a directive list of its own", name)
		}
	}
	raw, err := os.ReadFile(rootPath(t, "src", "catalog.ts"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := string(raw)
	body := regexp.MustCompile(`(?s)export function fallbackCatalog\(\): AruCatalog \{.*?\n\}`).FindString(catalog)
	if body == "" {
		t.Fatal("catalog.ts has no fallbackCatalog")
	}
	if strings.Count(catalog, "fallback.") != strings.Count(body, "fallback.") {
		t.Error("catalog.ts reads the fallback list outside fallbackCatalog")
	}
	extension, err := os.ReadFile(rootPath(t, "src", "extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, seam := range []string{
		"if (!this.serverFeatures.catalog) {\n      this.catalog = fallbackCatalog();",
		"this.catalog = parseCatalog(response);",
		"directiveAt(this.catalog, document.lineAt(position.line).text, position.character)",
	} {
		if !strings.Contains(string(extension), seam) {
			t.Errorf("catalogue use does not contain %q", seam)
		}
	}
}

// TestTheEditorRunsGeneratorsOnlyFromTheCatalogue holds the generator command
// to the make: commands of the catalogue, asked for and shown before they run.
func TestTheEditorRunsGeneratorsOnlyFromTheCatalogue(t *testing.T) {
	runExtensionModules(t, `
const raw = fixture("catalog.json");
const catalog = m.parseCatalog(raw);
const generators = m.generatorCommands(catalog);
assert(generators.length > 0 && generators.every((command) => command.name.startsWith("make:")), "only make: commands");
same(generators.length, raw.commands.filter((command) => command.name.startsWith("make:")).length, "every make: command is offered");
const planted = m.parseCatalog({ directives: [], commands: [
  { name: "migrate", usage: "aru migrate", description: "", flags: [] },
  { name: "db:seed", usage: "aru db:seed", description: "", flags: [] },
  { name: "vendor:publish", usage: "aru vendor:publish [--apply]", description: "", flags: ["--apply"] },
  { name: "make:../escape", usage: "aru make:../escape", description: "", flags: [] },
] });
same(m.generatorCommands(planted), [], "migrations, seeders, wiring and odd names are never generators");
throws(() => m.generatorArguments(planted.commands[0], undefined, [], false), /migrate is not a generator/, "a non-generator cannot be run");

const model = generators.find((command) => command.name === "make:model");
same(m.generatorName(model), "Name", "the positional argument");
const flags = m.generatorFlags(model);
const fields = flags.find((flag) => flag.flag === "--fields");
same([fields.takesValue, fields.required, fields.example], [true, true, "reference:string!u,total:money"], "--fields from the usage line");
same(flags.find((flag) => flag.flag === "--tenant"), { flag: "--tenant", takesValue: false, required: false }, "--tenant is a switch");
assert(!flags.some((flag) => flag.flag === "-m"), "a short alias of a long flag is not offered twice");
assert(!flags.some((flag) => flag.flag === "--dry-run"), "the preview is its own step");
assert(m.supportsPreview(model), "make:model previews");
same(m.generatorArguments(model, "Invoice", [{ flag: "--fields", value: "total:money" }, { flag: "--tenant" }], true), ["make:model", "Invoice", "--fields=total:money", "--tenant", "--dry-run"], "the argument vector");
throws(() => m.generatorArguments(model, "Invoice", [{ flag: "--apply" }], false), /make:model has no flag --apply/, "a flag the generator lacks");
const controller = generators.find((command) => command.name === "make:controller");
assert(!m.supportsPreview(controller), "make:controller has no preview");
same(m.generatorFlags(controller).find((flag) => flag.flag === "--resource").takesValue, false, "--resource is a switch");
`)

	raw, err := os.ReadFile(rootPath(t, "src", "extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		`registerCommand("arandu.make.run", () => this.runGenerator())`,
		"const generators = generatorCommands(this.catalog);",
		"if (this.catalog.source !== \"aru\" || generators.length === 0) {",
		"modal: true,",
		"const args = generatorArguments(command, request.name, request.flags, action === previewAction);",
		"shellArgs: args,",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("generator command does not contain %q", seam)
		}
	}
	var manifest struct {
		Contributes struct {
			Commands []struct {
				Command string `json:"command"`
			} `json:"commands"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)
	found := false
	for _, command := range manifest.Contributes.Commands {
		found = found || command.Command == "arandu.make.run"
	}
	if !found {
		t.Error("the manifest does not contribute arandu.make.run")
	}
}

func jsonArray(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = `"` + value + `"`
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
