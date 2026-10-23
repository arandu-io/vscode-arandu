package extension_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryDeclaredCommandIsRegisteredAndEveryRegisteredOneDeclared keeps the
// manifest and the code from drifting apart for every command at once: one
// declared and never registered fails when somebody runs it, one registered
// and never declared cannot be reached from the palette, a lens or a link.
func TestEveryDeclaredCommandIsRegisteredAndEveryRegisteredOneDeclared(t *testing.T) {
	var manifest struct {
		Contributes struct {
			Commands []struct {
				Command string `json:"command"`
				Title   string `json:"title"`
			} `json:"commands"`
			Menus map[string][]struct {
				Command string `json:"command"`
				When    string `json:"when"`
			} `json:"menus"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	declared := map[string]bool{}
	for _, command := range manifest.Contributes.Commands {
		if command.Title == "" {
			t.Errorf("%s has no title", command.Command)
		}
		declared[command.Command] = true
	}
	registration := regexp.MustCompile(`registerCommand\("([^"]+)"`)
	registered := map[string]bool{}
	for _, file := range []string{"src/extension.ts", "src/testing.ts", "src/aruUpdate.ts"} {
		for _, match := range registration.FindAllStringSubmatch(readFile(t, file), -1) {
			registered[match[1]] = true
		}
	}
	for _, command := range sortedKeys(declared) {
		if !registered[command] {
			t.Errorf("%s is declared and never registered", command)
		}
	}
	for _, command := range sortedKeys(registered) {
		// arandu.output.show is reached only from the status bar, which needs
		// no declaration.
		if !declared[command] && command != "arandu.output.show" {
			t.Errorf("%s is registered and never declared", command)
		}
	}
	for _, command := range []string{
		"arandu.testing.refresh",
		"arandu.e2e.selectDriver",
		"arandu.e2e.record",
		"arandu.e2e.explain",
		"arandu.e2e.openTrace",
		"arandu.e2e.runJourney",
		"arandu.e2e.debugJourney",
	} {
		if !declared[command] {
			t.Errorf("the testing command %s is not declared", command)
		}
	}
	// The lens commands take the file and the test; from the palette they
	// would run with neither.
	hidden := map[string]bool{}
	for _, entry := range manifest.Contributes.Menus["commandPalette"] {
		hidden[entry.Command] = entry.When == "false"
	}
	for _, command := range []string{"arandu.e2e.runJourney", "arandu.e2e.debugJourney"} {
		if !hidden[command] {
			t.Errorf("%s needs a file and a test, and must be hidden from the palette", command)
		}
	}
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestTheJourneyCommandsCallAruByTheirNames fixes the command lines the
// extension hands aru, so a change on either side is a test that fails rather
// than a button that does nothing.
func TestTheJourneyCommandsCallAruByTheirNames(t *testing.T) {
	var contract struct {
		Layers []struct {
			ID        string `json:"id"`
			Directory string `json:"directory"`
		} `json:"layers"`
		E2ETag      string   `json:"e2eTag"`
		TestArgs    []string `json:"testArgs"`
		E2EFlag     string   `json:"e2eFlag"`
		DriverFlag  string   `json:"driverFlag"`
		BrowserFlag string   `json:"browserFlag"`
		RecordArgs  []string `json:"recordArgs"`
		ExplainArgs []string `json:"explainArgs"`
		Drivers     []struct {
			Driver   string   `json:"driver"`
			Browsers []string `json:"browsers"`
			Module   string   `json:"module"`
		} `json:"drivers"`
		Env struct {
			Driver   string `json:"driver"`
			Browser  string `json:"browser"`
			Headed   string `json:"headed"`
			Trace    string `json:"trace"`
			TraceDir string `json:"traceDir"`
		} `json:"env"`
		TraceFile     string `json:"traceFile"`
		TraceVersion  int    `json:"traceVersion"`
		GoExtensionID string `json:"goExtensionId"`
	}
	readJSON(t, "src/testingContract.json", &contract)

	layers := []string{}
	for _, layer := range contract.Layers {
		layers = append(layers, layer.ID+"="+layer.Directory)
	}
	if got, want := strings.Join(layers, " "), "unit=tests/Unit feature=tests/Feature contract=tests/Contract e2e=tests/E2E conformance=tests/Conformance"; got != want {
		t.Errorf("layers = %s, want %s", got, want)
	}
	for name, pair := range map[string][2]string{
		"e2e tag":      {contract.E2ETag, "e2e"},
		"e2e flag":     {contract.E2EFlag, "--e2e"},
		"driver flag":  {contract.DriverFlag, "--driver"},
		"browser flag": {contract.BrowserFlag, "--browser"},
		"trace file":   {contract.TraceFile, "trace.json"},
		"go extension": {contract.GoExtensionID, "golang.go"},
		"driver env":   {contract.Env.Driver, "ARANDU_E2E_DRIVER"},
		"browser env":  {contract.Env.Browser, "ARANDU_E2E_BROWSER"},
		"headed env":   {contract.Env.Headed, "ARANDU_E2E_HEADED"},
		"trace env":    {contract.Env.Trace, "ARANDU_E2E_TRACE"},
		"trace dir":    {contract.Env.TraceDir, "ARANDU_E2E_TRACE_DIR"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", name, pair[0], pair[1])
		}
	}
	for name, pair := range map[string][2][]string{
		"aru test":        {contract.TestArgs, {"test"}},
		"aru e2e:record":  {contract.RecordArgs, {"e2e:record"}},
		"aru e2e:explain": {contract.ExplainArgs, {"e2e:explain"}},
	} {
		if strings.Join(pair[0], " ") != strings.Join(pair[1], " ") {
			t.Errorf("%s runs %v", name, pair[0])
		}
	}
	if contract.TraceVersion != 1 {
		t.Errorf("the trace reader reads version %d; the test kit writes 1", contract.TraceVersion)
	}
	if len(contract.Drivers) != 2 || contract.Drivers[0].Driver != "cdp" || contract.Drivers[0].Module != "" ||
		contract.Drivers[1].Driver != "playwright" || contract.Drivers[1].Module != "github.com/hyz-is/arandu-playwright" ||
		strings.Join(contract.Drivers[1].Browsers, ",") != "chromium,firefox,webkit" {
		t.Errorf("drivers = %+v", contract.Drivers)
	}
}

// TestNoTestRunsInAnUntrustedWorkspace keeps every path that executes the
// project's code -- a run, a debug session, a recording, an explanation --
// behind the workspace's trust, checked before the process starts.
func TestNoTestRunsInAnUntrustedWorkspace(t *testing.T) {
	source := readFile(t, "src/testing.ts")

	refuse := methodBody(t, source, "private refuse()")
	if !strings.Contains(refuse, "!vscode.workspace.isTrusted") {
		t.Fatal("refuse does not check the workspace's trust")
	}
	for _, method := range []struct{ name, starts string }{
		{"private async run(", "runAru("},
		{"private async debug(", "vscode.debug.startDebugging("},
	} {
		body := methodBody(t, source, method.name)
		check := strings.Index(body, "this.refuse()")
		start := strings.Index(body, method.starts)
		if check < 0 || start < 0 || check > start {
			t.Errorf("%s starts %s without asking refuse first", method.name, method.starts)
		}
		if !strings.Contains(body[check:start], "return;") {
			t.Errorf("%s does not stop when refused", method.name)
		}
	}
	ready := methodBody(t, source, "private async readyProject(")
	if !strings.Contains(ready, "!vscode.workspace.isTrusted") {
		t.Fatal("readyProject does not check the workspace's trust")
	}
	for _, method := range []string{"private async record(", "private async explain("} {
		body := methodBody(t, source, method)
		check := strings.Index(body, "await this.readyProject(")
		run := strings.Index(body, "this.runInTerminal(")
		if check < 0 || run < 0 || check > run {
			t.Errorf("%s runs aru without the trust check first", method)
		}
	}
	// The lens commands go through the same run and debug handlers.
	journey := methodBody(t, source, "private async runJourney(")
	if !strings.Contains(journey, "this.debug(request") || !strings.Contains(journey, "this.run(request") {
		t.Error("the journey lenses bypass the run and debug handlers and their trust check")
	}
}

// TestAruIsStartedWithoutAShell keeps every process the testing side starts
// from reading a test name, a tag or a path as shell syntax.
func TestAruIsStartedWithoutAShell(t *testing.T) {
	runner := readFile(t, "src/testRunner.ts")
	if !strings.Contains(runner, "shell: false") || strings.Contains(runner, "shell: true") {
		t.Error("the test runner does not start aru with shell: false")
	}
	if strings.Contains(runner, "exec(") || strings.Contains(runner, "execSync(") {
		t.Error("the test runner uses exec, which goes through a shell")
	}
	testing := readFile(t, "src/testing.ts")
	terminal := methodBody(t, testing, "private async runInTerminal(")
	for _, seam := range []string{"shellPath: aru.executable", "shellArgs: [...args]"} {
		if !strings.Contains(terminal, seam) {
			t.Errorf("the journey terminal does not run aru itself: missing %q", seam)
		}
	}
	if strings.Contains(testing, "sendText(") || strings.Contains(testing, "ShellExecution") {
		t.Error("a command line is typed into a shell instead of started as a process")
	}
}

// TestTheTraceViewerIsAStrictWebview fixes the webview's options and what it
// may ask the editor: scripts for its own nonce only, no command links, no
// forms, files only from the trace's directory, and one message -- explain.
func TestTheTraceViewerIsAStrictWebview(t *testing.T) {
	source := readFile(t, "src/testing.ts")
	show := methodBody(t, source, "private async showTrace(")
	for _, seam := range []string{
		"enableScripts: true",
		"enableCommandUris: false",
		"enableForms: false",
		"localResourceRoots: [vscode.Uri.file(dir)]",
		"safeTracePath(dir, step.screenshot)",
		"safeTracePath(dir, step.dom)",
		"panel.webview.asWebviewUri(",
		"cspSource: panel.webview.cspSource",
		"randomBytes(18)",
		`(message as { type?: unknown }).type === "explain"`,
	} {
		if !strings.Contains(show, seam) {
			t.Errorf("the trace viewer does not contain %q", seam)
		}
	}
	if strings.Count(show, "onDidReceiveMessage") != 1 {
		t.Error("the trace viewer answers more than one kind of message")
	}
	view := readFile(t, "src/traceView.ts")
	if strings.Contains(view, "innerHTML") || strings.Contains(view, "insertAdjacentHTML") || strings.Contains(view, "eval(") {
		t.Error("the page's script turns text into markup")
	}
}

// TestJourneysHaveRunDebugHeadedAndOpenLastTraceLenses fixes the lenses over a
// journey and that they appear on the E2E layer only.
func TestJourneysHaveRunDebugHeadedAndOpenLastTraceLenses(t *testing.T) {
	source := readFile(t, "src/testing.ts")
	lenses := methodBody(t, source, "private journeyLenses(")
	for _, seam := range []string{
		`"$(play) Run", command: "arandu.e2e.runJourney"`,
		`"$(debug-alt) Debug headed", command: "arandu.e2e.debugJourney"`,
		`"$(history) Open last trace", command: "arandu.e2e.openTrace"`,
		"!isE2E(layer)",
	} {
		if !strings.Contains(lenses, seam) {
			t.Errorf("the journey lenses do not contain %q", seam)
		}
	}
	debug := methodBody(t, source, "private async debug(")
	for _, seam := range []string{
		"vscode.extensions.getExtension(contract.goExtensionId) === undefined",
		"e2eEnvironment(choice, { headed: true, traceDir })",
		"debugConfiguration(",
		"{ testRun: run }",
	} {
		if !strings.Contains(debug, seam) {
			t.Errorf("debugging does not contain %q", seam)
		}
	}
	run := methodBody(t, source, "private async run(")
	if !strings.Contains(run, "e2eEnvironment(choice, { headed: false, traceDir })") {
		t.Error("an ordinary run of a journey is not headless, or writes no trace")
	}
}

// TestTheTestingViewFollowsTheSelectedProject keeps the tests on the same root
// as every other capability: the controller is created once, pointed at the
// project the selector chose, and watches only that project's tests.
func TestTheTestingViewFollowsTheSelectedProject(t *testing.T) {
	extension := readFile(t, "src/extension.ts")
	for _, seam := range []string{
		`import { AranduTesting } from "./testing"`,
		"new AranduTesting({",
		"project: () => this.projects.active",
		"devRunning: () => this.devTerminal !== undefined",
		"this.testing.setProject(project)",
		"this.testing,",
	} {
		if !strings.Contains(extension, seam) {
			t.Errorf("extension.ts does not contain %q", seam)
		}
	}
	testing := readFile(t, "src/testing.ts")
	for _, seam := range []string{
		"vscode.tests.createTestController(contract.controllerId, contract.controllerLabel)",
		"vscode.TestRunProfileKind.Run",
		"vscode.TestRunProfileKind.Debug",
		"configureHandler = () => void this.selectDriver()",
		"new vscode.RelativePattern(project.root, `${contract.testsRoot}/**/*${contract.testFileSuffix}`)",
		"new vscode.RelativePattern(project.root, `${layer.directory}/**/*${contract.testFileSuffix}`)",
		"adapterContract.projectDiscoveryExclude",
	} {
		if !strings.Contains(testing, seam) {
			t.Errorf("testing.ts does not contain %q", seam)
		}
	}
	record := methodBody(t, testing, "private async record(")
	for _, seam := range []string{"this.host.devRunning() || await isListening(url)", "recordTarget(input, running ? url : undefined)", "contract.recordArgs"} {
		if !strings.Contains(record, seam) {
			t.Errorf("recording does not reuse the running server: missing %q", seam)
		}
	}
}

// methodBody answers the text of a method, from its signature to the line
// that closes it at the method's indentation.
func methodBody(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, "  "+signature)
	if start < 0 {
		t.Fatalf("%s is not defined", signature)
	}
	rest := source[start:]
	end := strings.Index(rest, "\n  }\n")
	if end < 0 {
		t.Fatalf("%s is never closed", signature)
	}
	return rest[:end]
}
