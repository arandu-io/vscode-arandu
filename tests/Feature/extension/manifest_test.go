package extension_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTheExtensionStartsTheAranduLanguageClientAndProjectMap(t *testing.T) {
	var manifest struct {
		Name         string            `json:"name"`
		DisplayName  string            `json:"displayName"`
		Publisher    string            `json:"publisher"`
		Version      string            `json:"version"`
		Preview      bool              `json:"preview"`
		Icon         string            `json:"icon"`
		Main         string            `json:"main"`
		Browser      string            `json:"browser"`
		Activation   []string          `json:"activationEvents"`
		Scripts      map[string]string `json:"scripts"`
		Dependencies map[string]string `json:"dependencies"`
		DevDeps      map[string]string `json:"devDependencies"`
		Files        []string          `json:"files"`
		Capabilities struct {
			Untrusted struct {
				Supported any `json:"supported"`
			} `json:"untrustedWorkspaces"`
			Virtual struct {
				Supported bool `json:"supported"`
			} `json:"virtualWorkspaces"`
		} `json:"capabilities"`
		Contributes struct {
			Languages []struct {
				ID            string   `json:"id"`
				Extensions    []string `json:"extensions"`
				Configuration string   `json:"configuration"`
			} `json:"languages"`
			Grammars []struct {
				Language  string `json:"language"`
				ScopeName string `json:"scopeName"`
				Path      string `json:"path"`
			} `json:"grammars"`
			Snippets []struct {
				Language string `json:"language"`
				Path     string `json:"path"`
			} `json:"snippets"`
			ConfigurationDefaults map[string]map[string]any `json:"configurationDefaults"`
			Configuration         struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			} `json:"configuration"`
			ViewContainers struct {
				ActivityBar []struct {
					ID string `json:"id"`
				} `json:"activitybar"`
			} `json:"viewsContainers"`
			Views map[string][]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"views"`
			ViewsWelcome []struct {
				View     string `json:"view"`
				Contents string `json:"contents"`
				When     string `json:"when"`
			} `json:"viewsWelcome"`
			Commands []struct {
				Command string `json:"command"`
			} `json:"commands"`
			Menus map[string][]struct {
				Command string `json:"command"`
				When    string `json:"when"`
			} `json:"menus"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	if got, want := manifest.Publisher+"."+manifest.Name, "arandu-io.arandu"; got != want {
		t.Fatalf("extension identifier = %q, want %q", got, want)
	}
	if manifest.DisplayName != "Arandu" || manifest.Version != "0.1.1" || !manifest.Preview {
		t.Fatalf("identity = %q %q preview=%t, want Arandu 0.1.1 preview=true", manifest.DisplayName, manifest.Version, manifest.Preview)
	}
	if manifest.Main != "./dist/extension.js" || manifest.Browser != "" {
		t.Fatalf("extension host = main %q browser %q", manifest.Main, manifest.Browser)
	}
	for _, event := range []string{"onLanguage:kyse", "onView:arandu.projectMap", "onView:arandu.development", "workspaceContains:arandu.toml"} {
		if !contains(manifest.Activation, event) {
			t.Errorf("activation events do not contain %q", event)
		}
	}
	if got := manifest.Dependencies["vscode-languageclient"]; got == "" || len(manifest.Dependencies) != 1 {
		t.Fatalf("runtime dependencies = %v, want only vscode-languageclient", manifest.Dependencies)
	}
	for _, tool := range []string{"@types/node", "@types/vscode", "@vscode/vsce", "esbuild", "typescript"} {
		if manifest.DevDeps[tool] == "" {
			t.Errorf("devDependencies do not pin %s", tool)
		}
	}
	if manifest.Scripts["typecheck"] != "tsc --noEmit" || manifest.Scripts["bundle"] == "" {
		t.Fatalf("build scripts = %v", manifest.Scripts)
	}
	if !contains(manifest.Files, "dist/extension.js") || !contains(manifest.Files, "images/activity.svg") {
		t.Fatalf("published files = %v", manifest.Files)
	}
	if manifest.Capabilities.Untrusted.Supported != "limited" || manifest.Capabilities.Virtual.Supported {
		t.Fatalf("workspace capabilities = untrusted:%v virtual:%t", manifest.Capabilities.Untrusted.Supported, manifest.Capabilities.Virtual.Supported)
	}
	if property := manifest.Contributes.Configuration.Properties["arandu.aru.path"]; property.Type != "string" {
		t.Fatal("arandu.aru.path is not a string workspace configuration")
	}
	if len(manifest.Contributes.ViewContainers.ActivityBar) != 1 || manifest.Contributes.ViewContainers.ActivityBar[0].ID != "arandu" {
		t.Fatalf("activity bar containers = %#v", manifest.Contributes.ViewContainers.ActivityBar)
	}
	views := manifest.Contributes.Views["arandu"]
	if len(views) != 2 || views[0].ID != "arandu.projectMap" || views[1].ID != "arandu.development" || views[1].Name != "Development" {
		t.Fatalf("Arandu views = %#v", manifest.Contributes.Views["arandu"])
	}
	for _, command := range []string{
		"arandu.projectMap.refresh",
		"arandu.languageServer.restart",
		"arandu.aru.configure",
		"arandu.dev.start",
		"arandu.dev.stop",
		"arandu.dev.restart",
		"arandu.doctor.run",
		"arandu.aru.updateWithHomebrew",
	} {
		found := false
		for _, candidate := range manifest.Contributes.Commands {
			found = found || candidate.Command == command
		}
		if !found {
			t.Errorf("commands do not contain %q", command)
		}
	}
	viewMenu := manifest.Contributes.Menus["view/title"]
	for _, command := range []string{"arandu.dev.start", "arandu.dev.stop", "arandu.dev.restart"} {
		found := false
		for _, item := range viewMenu {
			found = found || item.Command == command && strings.Contains(item.When, "view == arandu.projectMap")
		}
		if !found {
			t.Errorf("Project Map title menu does not contain %q", command)
		}
	}
	if manifest.Icon != "images/icon.png" {
		t.Fatalf("icon = %q, want the first-party Arandu icon", manifest.Icon)
	}
	if len(manifest.Contributes.Languages) != 1 {
		t.Fatalf("languages = %d, want exactly Kyse", len(manifest.Contributes.Languages))
	}
	language := manifest.Contributes.Languages[0]
	if language.ID != "kyse" || !contains(language.Extensions, ".kyse.go") || language.Configuration != "language-configuration.json" {
		t.Fatalf("Kyse language declaration = %#v", language)
	}
	if len(manifest.Contributes.Grammars) != 1 {
		t.Fatalf("grammars = %d, want exactly one", len(manifest.Contributes.Grammars))
	}
	grammar := manifest.Contributes.Grammars[0]
	if grammar.Language != "kyse" || grammar.ScopeName != "source.kyse" || grammar.Path != "syntaxes/kyse.tmLanguage.json" {
		t.Fatalf("Kyse grammar declaration = %#v", grammar)
	}
	if len(manifest.Contributes.Snippets) != 1 || manifest.Contributes.Snippets[0].Language != "kyse" || manifest.Contributes.Snippets[0].Path != "snippets/kyse.json" {
		t.Fatalf("Kyse snippets declaration = %#v", manifest.Contributes.Snippets)
	}
	defaults := manifest.Contributes.ConfigurationDefaults["[kyse]"]
	if format, ok := defaults["editor.formatOnSave"].(bool); !ok || format {
		t.Fatal("Kyse must disable format-on-save because gofmt cannot parse view directives")
	}
	if _, err := os.Stat(rootPath(t, manifest.Icon)); err != nil {
		t.Fatalf("first-party icon: %v", err)
	}
	activity, err := os.ReadFile(rootPath(t, "images/activity.svg"))
	if err != nil {
		t.Fatalf("activity icon: %v", err)
	}
	if !strings.Contains(string(activity), `fill="currentColor"`) || strings.Contains(string(activity), `fill="white"`) {
		t.Fatal("Activity Bar icon must follow the current VS Code theme")
	}

	welcomeByState := make(map[string]string, len(manifest.Contributes.ViewsWelcome))
	for _, welcome := range manifest.Contributes.ViewsWelcome {
		if welcome.View == "arandu.development" {
			welcomeByState[welcome.When] = welcome.Contents
		}
	}
	stopped := welcomeByState["!arandu.dev.running"]
	for _, action := range []string{
		"[Start aru dev](command:arandu.dev.start)",
		"[Run Doctor](command:arandu.doctor.run)",
		"[Configure Aru](command:arandu.aru.configure)",
	} {
		if !strings.Contains(stopped, action) {
			t.Errorf("stopped Development view does not contain %q", action)
		}
	}
	running := welcomeByState["arandu.dev.running"]
	for _, action := range []string{
		"[Stop](command:arandu.dev.stop)",
		"[Restart](command:arandu.dev.restart)",
		"[Run Doctor](command:arandu.doctor.run)",
	} {
		if !strings.Contains(running, action) {
			t.Errorf("running Development view does not contain %q", action)
		}
	}
}

func readJSON(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(rootPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func rootPath(t *testing.T, elements ...string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elements...)...)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
