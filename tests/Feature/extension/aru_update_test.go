package extension_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTheExtensionChecksStableAruReleasesWithoutAutomaticUpdates(t *testing.T) {
	var contract struct {
		LatestReleaseURL           string   `json:"latestReleaseURL"`
		CacheMilliseconds          int      `json:"cacheMilliseconds"`
		RequestTimeoutMilliseconds int      `json:"requestTimeoutMilliseconds"`
		VersionTimeoutMilliseconds int      `json:"versionTimeoutMilliseconds"`
		CacheKey                   string   `json:"cacheKey"`
		DismissedVersionKey        string   `json:"dismissedVersionKey"`
		BrewPathOrder              []string `json:"brewPathOrder"`
		BrewUpgradeArgs            []string `json:"brewUpgradeArgs"`
		UpdateAction               string   `json:"updateAction"`
		ManualUpdateOnly           bool     `json:"manualUpdateOnly"`
	}
	readJSON(t, "src/updateContract.json", &contract)

	if contract.LatestReleaseURL != "https://api.github.com/repos/arandu-io/aru/releases/latest" {
		t.Fatalf("latest release URL = %q", contract.LatestReleaseURL)
	}
	if contract.CacheMilliseconds != 24*60*60*1000 {
		t.Fatalf("update cache = %dms, want 24 hours", contract.CacheMilliseconds)
	}
	if contract.RequestTimeoutMilliseconds <= 0 || contract.RequestTimeoutMilliseconds > 10_000 {
		t.Fatalf("release request timeout = %dms", contract.RequestTimeoutMilliseconds)
	}
	if contract.VersionTimeoutMilliseconds <= 0 || contract.VersionTimeoutMilliseconds > 10_000 {
		t.Fatalf("aru version timeout = %dms", contract.VersionTimeoutMilliseconds)
	}
	if contract.CacheKey == "" || contract.DismissedVersionKey == "" || contract.CacheKey == contract.DismissedVersionKey {
		t.Fatalf("global state keys = %q and %q", contract.CacheKey, contract.DismissedVersionKey)
	}
	wantBrewPaths := []string{"PATH", "/opt/homebrew/bin/brew", "/usr/local/bin/brew"}
	if !reflect.DeepEqual(contract.BrewPathOrder, wantBrewPaths) {
		t.Fatalf("Homebrew discovery = %v, want %v", contract.BrewPathOrder, wantBrewPaths)
	}
	if !reflect.DeepEqual(contract.BrewUpgradeArgs, []string{"upgrade", "arandu-io/tap/aru"}) {
		t.Fatalf("Homebrew update arguments = %v", contract.BrewUpgradeArgs)
	}
	if contract.UpdateAction != "Update with Homebrew" {
		t.Fatalf("update action = %q", contract.UpdateAction)
	}
	if !contract.ManualUpdateOnly {
		t.Fatal("Homebrew updates must remain explicit user actions")
	}
}

func TestTheExtensionOffersAVisibleHomebrewUpdateOnlyAfterUserAction(t *testing.T) {
	raw, err := os.ReadFile(rootPath(t, "src/aruUpdate.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		`execFile(executable, ["version"]`,
		"context.globalState",
		"showWarningMessage",
		"updateContract.updateAction",
		"new vscode.ProcessExecution(brew.executable, updateContract.brewUpgradeArgs, { cwd: folder.uri.fsPath })",
		"vscode.TaskRevealKind.Always",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("Aru update adapter does not contain %q", seam)
		}
	}
	for _, forbidden := range []string{"sendText(", "exec(", "spawn(", "shell: true"} {
		if strings.Contains(source, forbidden) {
			t.Errorf("Aru update adapter contains unsafe execution seam %q", forbidden)
		}
	}
}

func TestTheExtensionChecksForAruUpdatesAsSoonAsAruIsResolved(t *testing.T) {
	raw, err := os.ReadFile(rootPath(t, "src/extension.ts"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, seam := range []string{
		`import { AruUpdateManager } from "./aruUpdate"`,
		"new AruUpdateManager(context, this.output, async () => this.projects.active?.folder)",
		"void this.aruUpdates.check(aru.executable)",
	} {
		if !strings.Contains(source, seam) {
			t.Errorf("extension activation does not contain %q", seam)
		}
	}
	checkAt := strings.Index(source, "void this.aruUpdates.check(aru.executable)")
	serverAt := strings.Index(source, "await client.start()")
	if checkAt < 0 || serverAt < 0 || checkAt > serverAt {
		t.Fatal("Aru update discovery must not depend on the installed CLI supporting the language server")
	}
}
