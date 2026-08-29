package extension_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTheProjectMapPinsSchemaV1AndTheNineCanonicalGroups(t *testing.T) {
	var contract struct {
		Request       string `json:"request"`
		SchemaVersion int    `json:"schemaVersion"`
		Groups        []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"groups"`
	}
	readJSON(t, "src/projectGraphContract.json", &contract)

	if contract.Request != "arandu/projectGraph" || contract.SchemaVersion != 1 {
		t.Fatalf("project graph seam = %q schema %d", contract.Request, contract.SchemaVersion)
	}
	want := [][2]string{
		{"application-features", "Application Features"},
		{"http", "HTTP"},
		{"database", "Database"},
		{"views", "Views"},
		{"async", "Async"},
		{"console", "Console"},
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
	for _, path := range []string{"arandu.toml", "go.mod", "main.go", "app/", "database/", "resources/views/", "routes/", "cmd/", "modules/", "framework/modules/"} {
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
