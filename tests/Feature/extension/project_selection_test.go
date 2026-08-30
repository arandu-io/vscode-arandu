package extension_test

import (
	"os"
	"strings"
	"testing"
)

func TestTheProjectMapDiscoversNestedProjectsAcrossFilesystemWorkspaceFolders(t *testing.T) {
	var contract struct {
		ProjectMarker           string `json:"projectMarker"`
		ProjectDiscoveryGlob    string `json:"projectDiscoveryGlob"`
		ProjectDiscoveryExclude string `json:"projectDiscoveryExclude"`
	}
	readJSON(t, "src/adapterContract.json", &contract)
	if contract.ProjectMarker != "arandu.toml" {
		t.Fatalf("project marker = %q, want arandu.toml", contract.ProjectMarker)
	}
	if contract.ProjectDiscoveryGlob != "**/arandu.toml" {
		t.Fatalf("project discovery glob = %q, want nested markers", contract.ProjectDiscoveryGlob)
	}
	if contract.ProjectDiscoveryExclude == "" {
		t.Fatal("project discovery must exclude generated and dependency trees")
	}

	raw, err := os.ReadFile(rootPath(t, "src/projects.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		"vscode.workspace.workspaceFolders ?? []",
		"new vscode.RelativePattern(folder, adapterContract.projectDiscoveryGlob)",
		"vscode.workspace.findFiles(pattern, adapterContract.projectDiscoveryExclude)",
		"path.dirname(marker.fsPath)",
		`folder.uri.scheme !== "file"`,
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("nested project discovery does not contain %q", seam)
		}
	}
}

func TestTheProjectMapRequiresExplicitSelectionWhenSeveralProjectsAreAvailable(t *testing.T) {
	var contract struct {
		ActiveProjectStateKey                string `json:"activeProjectStateKey"`
		ExplicitProjectSelectionWhenMultiple bool   `json:"explicitProjectSelectionWhenMultiple"`
	}
	readJSON(t, "src/adapterContract.json", &contract)
	if contract.ActiveProjectStateKey != "arandu.activeProjectUri" {
		t.Fatalf("active project state key = %q", contract.ActiveProjectStateKey)
	}
	if !contract.ExplicitProjectSelectionWhenMultiple {
		t.Fatal("workspaces with several Arandu projects must require an explicit selection")
	}

	raw, err := os.ReadFile(rootPath(t, "src/projects.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		"context.workspaceState",
		"projects.length === 1",
		"this.state.get<string>(adapterContract.activeProjectStateKey)",
		"projects.find((project) => project.key === rememberedKey)",
		"vscode.window.showQuickPick",
		"this.state.update(adapterContract.activeProjectStateKey, project.key)",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("explicit project selection does not contain %q", seam)
		}
	}
	if strings.Contains(source, "return projects[0]") {
		t.Fatal("multiple Arandu projects must never silently select the first discovery result")
	}
	resolveStart := strings.Index(source, "public async resolve()")
	chooseStart := strings.Index(source, "public async choose()")
	if resolveStart < 0 || chooseStart < 0 || resolveStart >= chooseStart {
		t.Fatal("project selection authority does not expose resolve followed by choose")
	}
	if strings.Contains(source[resolveStart:chooseStart], "this.pick(") {
		t.Fatal("project discovery must not open a picker automatically")
	}
	if !strings.Contains(source[chooseStart:], "return this.pick(projects)") {
		t.Fatal("only the explicit project selection command may open the picker")
	}
}

func TestTheProjectMapUsesTheSelectedRootForEveryEditorCapability(t *testing.T) {
	extensionRaw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	extension := string(extensionRaw)
	for _, seam := range []string{
		`import { AranduProjects } from "./projects"`,
		"new AranduProjects(context)",
		"const project = await this.projects.resolve()",
		"const folder = project.folder",
		"resolveAruExecutable(folder)",
		"options: { cwd: folder.uri.fsPath, shell: false }",
		"workspaceFolder: folder",
		`pattern: { baseUri: folder.uri.toString(), pattern: "**/*.kyse.go" }`,
		"this.createProjectWatcher(folder)",
		"this.publishDoctorDiagnostics(graph)",
		"cwd: folder.uri",
		"async () => this.projects.active?.folder",
	} {
		if !strings.Contains(extension, seam) {
			t.Errorf("selected project root does not reach the editor seam %q", seam)
		}
	}
	if strings.Contains(extension, "findAranduWorkspace") {
		t.Fatal("extension.ts must not keep a second project-root authority")
	}

	projectsRaw, err := os.ReadFile(rootPath(t, "src/projects.ts"))
	if err != nil {
		t.Fatal(err)
	}
	projects := string(projectsRaw)
	if !strings.Contains(projects, "folder: { uri: root, name: label, index: folder.index }") {
		t.Fatal("selected nested roots must become synthetic WorkspaceFolder values for the LSP root URI")
	}

	updateRaw, err := os.ReadFile(rootPath(t, "src/aruUpdate.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updateRaw), "{ cwd: folder.uri.fsPath }") {
		t.Fatal("the visible Homebrew task must run from the selected project root")
	}

	mapRaw, err := os.ReadFile(rootPath(t, "src/projectMap.ts"))
	if err != nil {
		t.Fatal(err)
	}
	projectMap := string(mapRaw)
	for _, seam := range []string{`readonly type: "project"`, `command: "arandu.project.select"`} {
		if !strings.Contains(projectMap, seam) {
			t.Errorf("Project Map selector row does not contain %q", seam)
		}
	}

	var manifest struct {
		Activation  []string `json:"activationEvents"`
		Contributes struct {
			Commands []struct {
				Command string `json:"command"`
			} `json:"commands"`
			Menus map[string][]struct {
				Command string `json:"command"`
				When    string `json:"when"`
			} `json:"menus"`
			ViewsWelcome []struct {
				View string `json:"view"`
				When string `json:"when"`
			} `json:"viewsWelcome"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)
	if !contains(manifest.Activation, "workspaceContains:**/arandu.toml") {
		t.Fatal("nested Arandu projects do not activate the extension")
	}
	commandDeclared := false
	for _, command := range manifest.Contributes.Commands {
		commandDeclared = commandDeclared || command.Command == "arandu.project.select"
	}
	if !commandDeclared {
		t.Fatal("project selection command is not declared")
	}
	for _, view := range []string{"arandu.projectMap", "arandu.development"} {
		found := false
		for _, item := range manifest.Contributes.Menus["view/title"] {
			found = found || item.Command == "arandu.project.select" && strings.Contains(item.When, "view == "+view)
		}
		if !found {
			t.Errorf("%s toolbar does not expose project selection", view)
		}
	}
	for _, command := range []string{"arandu.dev.start", "arandu.dev.stop", "arandu.dev.restart"} {
		found := false
		for _, item := range manifest.Contributes.Menus["view/title"] {
			found = found || item.Command == command && strings.Contains(item.When, "arandu.project.selected")
		}
		if !found {
			t.Errorf("%s toolbar action is visible without a selected project", command)
		}
	}
	selectionState := false
	for _, welcome := range manifest.Contributes.ViewsWelcome {
		selectionState = selectionState || welcome.View == "arandu.development" && welcome.When == "!arandu.project.selected"
	}
	if !selectionState {
		t.Fatal("Development view does not explain that a project selection is required")
	}
}

func TestTheProjectMapSwitchStopsTheOldServerWithoutStartingAnother(t *testing.T) {
	var contract struct {
		ProjectSwitchStartsDev *bool `json:"projectSwitchStartsDev"`
	}
	readJSON(t, "src/adapterContract.json", &contract)
	if contract.ProjectSwitchStartsDev == nil {
		t.Fatal("project switching must explicitly declare whether it starts aru dev")
	}
	if *contract.ProjectSwitchStartsDev {
		t.Fatal("selecting a project must never start aru dev automatically")
	}

	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "private async selectProject()")
	end := strings.Index(source, "private async configureAruPath()")
	if start < 0 || end < 0 || start >= end {
		t.Fatal("cannot locate the explicit project switch handler")
	}
	handler := source[start:end]
	if !strings.Contains(handler, "this.stopDev()") {
		t.Fatal("project switching does not stop the old development terminal")
	}
	if !strings.Contains(handler, "await this.restart()") {
		t.Fatal("project switching does not restart the selected project's language runtime")
	}
	if strings.Contains(handler, "startDev(") || strings.Contains(handler, "restartDev(") {
		t.Fatal("project switching starts a development server without an explicit Start command")
	}
}
