package extension_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestTheProjectMapPinsSchemaV1AndTheTenCanonicalGroups pins the fallback: an
// aru older than the typed map answers schema 1, whose ten groups are fixed.
// The typed schema's groups are the server's and are not counted here.
func TestTheProjectMapPinsSchemaV1AndTheTenCanonicalGroups(t *testing.T) {
	var contract struct {
		Request                     string `json:"request"`
		SchemaVersion               int    `json:"schemaVersion"`
		TypedSchemaVersion          int    `json:"typedSchemaVersion"`
		SchemasCapability           string `json:"schemasCapability"`
		DoctorDiagnosticsCapability string `json:"doctorDiagnosticsCapability"`
		DoctorDiagnosticsOption     string `json:"doctorDiagnosticsOption"`
		TypedMapAru                 string `json:"typedMapAru"`
		Groups                      []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"groups"`
	}
	readJSON(t, "src/projectGraphContract.json", &contract)

	if contract.Request != "arandu/projectGraph" || contract.SchemaVersion != 1 {
		t.Fatalf("project graph seam = %q schema %d", contract.Request, contract.SchemaVersion)
	}
	if contract.TypedSchemaVersion != 2 || contract.SchemasCapability != "aranduProjectGraphSchemas" {
		t.Fatalf("typed schema = %d advertised by %q", contract.TypedSchemaVersion, contract.SchemasCapability)
	}
	if contract.DoctorDiagnosticsCapability != "aranduDoctorDiagnostics" || contract.DoctorDiagnosticsOption != "doctorDiagnostics" {
		t.Fatalf("doctor diagnostics seam = capability %q option %q", contract.DoctorDiagnosticsCapability, contract.DoctorDiagnosticsOption)
	}
	if contract.TypedMapAru != "v0.65.0" {
		t.Fatalf("typed map needs aru %q, want v0.65.0", contract.TypedMapAru)
	}
	want := [][2]string{
		{"application-features", "Application Features"},
		{"http", "HTTP"},
		{"database", "Database"},
		{"views", "Views"},
		{"async", "Async"},
		{"console", "Console"},
		{"native-screens", "Native Screens"},
		{"native-capabilities", "Native Capabilities"},
		{"community-modules", "Community Modules"},
		{"diagnostics", "Diagnostics"},
	}
	got := make([][2]string, len(contract.Groups))
	for index, group := range contract.Groups {
		got[index] = [2]string{group.ID, group.Label}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("project graph groups = %v, want %v", got, want)
	}
}

func TestTheEditorAdapterHasAReadOnlyTrustedWorkspaceContract(t *testing.T) {
	var contract struct {
		ServerArgs            []string `json:"serverArgs"`
		AruPathOrder          []string `json:"aruPathOrder"`
		TrustedWorkspaces     bool     `json:"trustedWorkspacesOnly"`
		FilesystemWorkspaces  bool     `json:"filesystemWorkspacesOnly"`
		DebounceMilliseconds  int      `json:"debounceMilliseconds"`
		RelevantPaths         []string `json:"relevantPaths"`
		DiagnosticsCollection string   `json:"diagnosticsCollection"`
		DevArgs               []string `json:"devArgs"`
		ManualDevOnly         bool     `json:"manualDevOnly"`
	}
	readJSON(t, "src/adapterContract.json", &contract)

	if !reflect.DeepEqual(contract.ServerArgs, []string{"lsp"}) {
		t.Fatalf("server args = %v, want only aru lsp", contract.ServerArgs)
	}
	wantDiscovery := []string{"configuration", "PATH", "/opt/homebrew/bin/aru", "/usr/local/bin/aru"}
	if !reflect.DeepEqual(contract.AruPathOrder, wantDiscovery) {
		t.Fatalf("aru discovery = %v, want %v", contract.AruPathOrder, wantDiscovery)
	}
	if !contract.TrustedWorkspaces || !contract.FilesystemWorkspaces {
		t.Fatalf("workspace boundary = trusted:%t filesystem:%t", contract.TrustedWorkspaces, contract.FilesystemWorkspaces)
	}
	if contract.DebounceMilliseconds < 100 || contract.DebounceMilliseconds > 1_000 {
		t.Fatalf("watch debounce = %dms", contract.DebounceMilliseconds)
	}
	if contract.DiagnosticsCollection != "Arandu Doctor" {
		t.Fatalf("Doctor diagnostic collection = %q", contract.DiagnosticsCollection)
	}
	if !reflect.DeepEqual(contract.DevArgs, []string{"dev"}) || !contract.ManualDevOnly {
		t.Fatalf("dev contract = args:%v manual:%t", contract.DevArgs, contract.ManualDevOnly)
	}
	// Doctor reads tests/ for the test layout rules and .env.example for the
	// engines a project names, so a save in either has to refresh it too.
	for _, path := range []string{"arandu.toml", "go.mod", "main.go", ".env.example", "app/", "database/", "resources/views/", "routes/", "tests/", "cmd/", "modules/", "framework/modules/"} {
		if !contains(contract.RelevantPaths, path) {
			t.Errorf("relevant project paths do not contain %q", path)
		}
	}
	for _, argument := range contract.ServerArgs {
		lower := strings.ToLower(argument)
		for _, forbidden := range []string{"migrate", "seed", "generate"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("editor adapter may not run %q automatically", argument)
			}
		}
	}
}

func TestDoctorAndDevStayInsideExplicitEditorActions(t *testing.T) {
	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		"createDiagnosticCollection(adapterContract.diagnosticsCollection)",
		"this.doctorDiagnostics.clear()",
		"shellPath: aru.executable",
		"shellArgs: adapterContract.devArgs",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("editor adapter does not contain %q", seam)
		}
	}
	if strings.Contains(source, "sendText(") {
		t.Fatal("dev execution must not concatenate shell text")
	}
}

func TestTheDevelopmentViewUsesTheSameDoctorRefreshWithoutStartingProcesses(t *testing.T) {
	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		`createTreeView("arandu.development"`,
		`registerCommand("arandu.projectMap.refresh", () => this.refresh())`,
		`registerCommand("arandu.doctor.run", () => this.refresh())`,
		`void this.refreshGraph();`,
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("Development view adapter does not contain %q", seam)
		}
	}
	for _, forbidden := range []string{"sendText(", `shellArgs: ["migrate"`, `shellArgs: ["seed"`, `shellArgs: ["generate"`} {
		if strings.Contains(source, forbidden) {
			t.Errorf("Development view introduced forbidden process execution %q", forbidden)
		}
	}
}

// TestDoctorRefreshesForEveryGoFileDoctorReads runs the editor's own path
// matcher. Doctor parses every Go file in the project and skips the same five
// directories at any depth, so a save anywhere else in Go source can change a
// finding; the views aru writes under storage are its output, not its input.
func TestDoctorRefreshesForEveryGoFileDoctorReads(t *testing.T) {
	var contract struct {
		RelevantSuffix     string   `json:"relevantSuffix"`
		SkippedDirectories []string `json:"skippedDirectories"`
		SkippedPaths       []string `json:"skippedPaths"`
	}
	readJSON(t, "src/adapterContract.json", &contract)
	if contract.RelevantSuffix != ".go" {
		t.Fatalf("relevant suffix = %q, want .go", contract.RelevantSuffix)
	}
	if want := []string{".git", "bin", "node_modules", "testdata", "vendor"}; !reflect.DeepEqual(contract.SkippedDirectories, want) {
		t.Fatalf("skipped directories = %v, want the ones Doctor's walk skips: %v", contract.SkippedDirectories, want)
	}
	if want := []string{"storage/framework/views/"}; !reflect.DeepEqual(contract.SkippedPaths, want) {
		t.Fatalf("skipped paths = %v, want %v", contract.SkippedPaths, want)
	}

	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		`import { isRelevantProjectPath } from "./projectPaths";`,
		"if (!isRelevantProjectURI(folder, uri)) {",
		`return isRelevantProjectPath(relative.split(path.sep).join("/"));`,
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("watcher does not contain %q", seam)
		}
	}
	if strings.Contains(source, "adapterContract.relevantPaths") {
		t.Error("extension.ts matches project paths itself instead of calling projectPaths")
	}

	cases := map[string]bool{
		"main.go":                                 true,
		"internal/billing/charge.go":              true,
		"pkg/money/money_test.go":                 true,
		"app/Http/Controllers/home_controller.go": true,
		"resources/views/home.kyse.go":            true,
		"tests/Feature/home_test.go":              true,
		"arandu.toml":                             true,
		"go.mod":                                  true,
		".env.example":                            true,
		"config/app.toml":                         true,
		"modules/blog/arandu.mod.toml":            true,
		"storage/framework/views/home.go":         false,
		"storage/framework/views/layouts/app.go":  false,
		"vendor/github.com/acme/lib/lib.go":       false,
		"node_modules/acme/main.go":               false,
		"app/testdata/fixture.go":                 false,
		"tests/testdata/golden.json":              false,
		"tools/bin/generate.go":                   false,
		".git/hooks/pre-commit.go":                false,
		"README.md":                               false,
		"public/app.css":                          false,
		"../other/main.go":                        false,
		"":                                        false,
	}
	table, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "project-paths.cjs")
	build := exec.Command(
		filepath.Join(rootPath(t), "node_modules", ".bin", "esbuild"),
		"src/projectPaths.ts",
		"--bundle",
		"--platform=node",
		"--format=cjs",
		"--outfile="+bundle,
	)
	build.Dir = rootPath(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("bundle project path seam: %v\n%s", err, output)
	}
	check := exec.Command("node", "-e", `
const paths = require(process.env.ARANDU_PROJECT_PATHS_MODULE);
const cases = JSON.parse(process.env.ARANDU_PROJECT_PATHS_CASES);
const wrong = [];
for (const [relative, want] of Object.entries(cases)) {
  const got = paths.isRelevantProjectPath(relative);
  if (got !== want) {
    wrong.push(JSON.stringify(relative) + " relevant = " + got + ", want " + want);
  }
}
if (wrong.length > 0) {
  throw new Error(wrong.join("\n"));
}
`)
	check.Env = append(os.Environ(), "ARANDU_PROJECT_PATHS_MODULE="+bundle, "ARANDU_PROJECT_PATHS_CASES="+string(table))
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("project path contract: %v\n%s", err, output)
	}
}
