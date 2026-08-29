package extension_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTheExtensionClaimsKyseWithoutARuntime(t *testing.T) {
	var manifest struct {
		Name         string         `json:"name"`
		DisplayName  string         `json:"displayName"`
		Publisher    string         `json:"publisher"`
		Version      string         `json:"version"`
		Preview      bool           `json:"preview"`
		Icon         string         `json:"icon"`
		Main         string         `json:"main"`
		Browser      string         `json:"browser"`
		Activation   []string       `json:"activationEvents"`
		Scripts      map[string]any `json:"scripts"`
		Dependencies map[string]any `json:"dependencies"`
		DevDeps      map[string]any `json:"devDependencies"`
		Contributes  struct {
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
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	if got, want := manifest.Publisher+"."+manifest.Name, "arandu-io.arandu"; got != want {
		t.Fatalf("extension identifier = %q, want %q", got, want)
	}
	if manifest.DisplayName != "Arandu" || manifest.Version != "0.1.0" || !manifest.Preview {
		t.Fatalf("identity = %q %q preview=%t, want Arandu 0.1.0 preview=true", manifest.DisplayName, manifest.Version, manifest.Preview)
	}
	if manifest.Main != "" || manifest.Browser != "" || len(manifest.Activation) != 0 || len(manifest.Scripts) != 0 || len(manifest.Dependencies) != 0 || len(manifest.DevDeps) != 0 {
		t.Fatal("the declarative extension acquired a Node runtime or dependency graph")
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
