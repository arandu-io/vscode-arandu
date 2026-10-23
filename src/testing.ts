import { randomBytes, randomUUID } from "node:crypto";
import { mkdir, open, readFile, readdir, stat } from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import * as vscode from "vscode";
import adapterContract from "./adapterContract.json";
import { resolveAruExecutable } from "./aru";
import { devServerURL, isListening, recordTarget } from "./devServer";
import { describeChoice, driverChoices, e2eEnvironment, resolveChoice } from "./e2eDrivers";
import type { AranduProject } from "./projects";
import contract from "./testingContract.json";
import { listWithAru, scannedFile, scannedListing, type ListedFile, type TestListing } from "./testInventory";
import {
  debugConfiguration,
  editorLine,
  failureLine,
  isE2E,
  layerOf,
  moduleOf,
  parentKey,
  parseTestFile,
  resultKey,
  testBatches,
  testLayers,
  type DriverChoice,
  type TestLayer,
  type TestSelection,
} from "./testLayout";
import { ResultCollector, runAru } from "./testRunner";
import { parseTrace, safeTracePath, traceBases, traceDirectory, type Trace } from "./traceModel";
import { renderTraceHtml } from "./traceView";

// What the testing side needs from the rest of the extension.
export interface TestingHost {
  readonly context: vscode.ExtensionContext;
  readonly output: vscode.LogOutputChannel;
  readonly project: () => AranduProject | undefined;
  readonly devRunning: () => boolean;
}

interface TestData {
  readonly kind: "layer" | "file" | "test" | "subtest";
  readonly layer: TestLayer;
  readonly relative: string;
  // name is the test's full name, TestX/sub for a subtest, and empty above.
  readonly name: string;
  readonly tags: readonly string[];
  // packagePath is the import path go test reports for the file's package,
  // or empty when it is not known.
  readonly packagePath: string;
}

interface TracePick extends vscode.QuickPickItem {
  readonly dir: string | undefined;
}

const untrustedMessage = "Trust this workspace to run its tests. Arandu never runs a test in an untrusted workspace.";
const browseLabel = "$(folder-opened) Browse for a trace…";

// AranduTesting is the project's tests in the editor: the Testing view by
// layer, running through aru test and debugging through the Go extension, the
// lenses over journeys, the trace viewer, and recording and explaining
// journeys with aru.
export class AranduTesting implements vscode.Disposable {
  private readonly controller = vscode.tests.createTestController(contract.controllerId, contract.controllerLabel);
  private readonly data = new WeakMap<vscode.TestItem, TestData>();
  private readonly lensesChanged = new vscode.EventEmitter<void>();
  private readonly disposables: vscode.Disposable[] = [];
  private readonly panels = new Map<string, vscode.WebviewPanel>();
  private readonly runProfile: vscode.TestRunProfile;
  private readonly debugProfile: vscode.TestRunProfile;
  private projectDisposables: vscode.Disposable[] = [];
  private projectKey: string | undefined;
  // layers is the layers of the last listing, in the order the view shows them.
  private layers: readonly TestLayer[] = testLayers;
  // source is where the last listing came from: aru, which is asked again when
  // a test file changes, or the editor's own reading, which rereads the file.
  private source: TestListing["source"] = "scanner";
  private generation = 0;
  private rediscoverTimer: ReturnType<typeof setTimeout> | undefined;

  public constructor(private readonly host: TestingHost) {
    this.controller.resolveHandler = async (item) => {
      if (item === undefined) {
        await this.discover();
      }
    };
    this.controller.refreshHandler = () => this.discover();
    this.runProfile = this.controller.createRunProfile(
      "Run",
      vscode.TestRunProfileKind.Run,
      (request, token) => this.run(request, token),
      true,
    );
    this.debugProfile = this.controller.createRunProfile(
      "Debug",
      vscode.TestRunProfileKind.Debug,
      (request, token) => this.debug(request, token),
      true,
    );
    this.runProfile.configureHandler = () => void this.selectDriver();
    this.debugProfile.configureHandler = () => void this.selectDriver();
    this.disposables.push(
      this.controller,
      this.runProfile,
      this.debugProfile,
      this.lensesChanged,
      vscode.languages.registerCodeLensProvider(
        { language: "go", scheme: "file", pattern: `**/${testLayers.find(isE2E)?.directory ?? "tests/E2E"}/**/*${contract.testFileSuffix}` },
        {
          onDidChangeCodeLenses: this.lensesChanged.event,
          provideCodeLenses: (document) => this.journeyLenses(document),
        },
      ),
      // Trust is what lets aru list the tests; until then the editor reads them.
      vscode.workspace.onDidGrantWorkspaceTrust(() => void this.discover()),
      vscode.commands.registerCommand("arandu.testing.refresh", () => this.discover()),
      vscode.commands.registerCommand("arandu.e2e.selectDriver", () => this.selectDriver()),
      vscode.commands.registerCommand("arandu.e2e.record", () => this.record()),
      vscode.commands.registerCommand("arandu.e2e.explain", (dir?: unknown) => this.explain(typeof dir === "string" ? dir : undefined)),
      vscode.commands.registerCommand("arandu.e2e.openTrace", (target?: unknown) => this.openTrace(target)),
      vscode.commands.registerCommand("arandu.e2e.runJourney", (uri: unknown, name: unknown) => this.runJourney(uri, name, this.runProfile)),
      vscode.commands.registerCommand("arandu.e2e.debugJourney", (uri: unknown, name: unknown) => this.runJourney(uri, name, this.debugProfile)),
    );
  }

  public dispose(): void {
    this.cancelRediscover();
    for (const disposable of this.projectDisposables.splice(0)) {
      disposable.dispose();
    }
    for (const panel of this.panels.values()) {
      panel.dispose();
    }
    this.panels.clear();
    for (const disposable of this.disposables.splice(0)) {
      disposable.dispose();
    }
  }

  // setProject points the Testing view at the selected project, or empties it.
  public setProject(project: AranduProject | undefined): void {
    if (project?.key === this.projectKey) {
      return;
    }
    this.projectKey = project?.key;
    this.cancelRediscover();
    for (const disposable of this.projectDisposables.splice(0)) {
      disposable.dispose();
    }
    this.controller.items.replace([]);
    this.lensesChanged.fire();
    if (project === undefined) {
      return;
    }
    // Every test file of the project, because aru lists the tests beside the
    // code too, under Other.
    const watcher = vscode.workspace.createFileSystemWatcher(
      new vscode.RelativePattern(project.root, `**/*${contract.testFileSuffix}`),
    );
    this.projectDisposables.push(
      watcher,
      watcher.onDidCreate((uri) => this.fileChanged(uri, false)),
      watcher.onDidChange((uri) => this.fileChanged(uri, false)),
      watcher.onDidDelete((uri) => this.fileChanged(uri, true)),
    );
    void this.discover();
  }

  // ---- discovery ---------------------------------------------------------

  // discover lists the selected project's tests and shows them: from aru test
  // --list --json when the workspace is trusted and aru has it, and otherwise
  // from the editor's own reading of the five layer directories.
  private async discover(): Promise<void> {
    this.cancelRediscover();
    const generation = ++this.generation;
    const project = this.host.project();
    if (project === undefined) {
      this.controller.items.replace([]);
      return;
    }
    const listing = await this.listWithAru(project) ?? await this.scanProject(project);
    if (generation !== this.generation || this.host.project()?.key !== project.key) {
      return;
    }
    this.source = listing.source;
    this.layers = listing.layers;
    this.controller.items.replace([]);
    for (const file of listing.files) {
      this.showFile(project, file);
    }
    for (const unreadable of listing.unreadable) {
      this.host.output.warn(`aru test --list could not parse ${unreadable.file}, so its tests are not shown: ${unreadable.error}`);
    }
    await this.describeE2ELayer();
  }

  // listWithAru asks aru for the tests, and answers undefined -- after saying
  // why -- when the editor has to read them itself. aru is never started in an
  // untrusted workspace.
  private async listWithAru(project: AranduProject): Promise<TestListing | undefined> {
    if (!vscode.workspace.isTrusted || project.root.scheme !== "file") {
      return undefined;
    }
    let executable: string;
    try {
      executable = (await resolveAruExecutable(project.folder)).executable;
    } catch (error: unknown) {
      this.host.output.info(`Reading the tests from the sources: ${errorMessage(error)}`);
      return undefined;
    }
    const answer = await listWithAru(executable, project.root.fsPath);
    if (answer.listing === undefined) {
      this.host.output.info(`Reading the tests from the sources: ${answer.reason}.`);
      return undefined;
    }
    this.host.output.info(`Listed ${answer.listing.files.reduce((sum, file) => sum + file.tests.length, 0)} tests with ${executable} test --list --json (${answer.version}).`);
    return answer.listing;
  }

  // scanProject reads the tests of the five layer directories itself, for an
  // aru that cannot list them.
  private async scanProject(project: AranduProject): Promise<TestListing> {
    const uris: vscode.Uri[] = [];
    for (const layer of testLayers) {
      const pattern = new vscode.RelativePattern(project.root, `${layer.directory}/**/*${contract.testFileSuffix}`);
      uris.push(...await vscode.workspace.findFiles(pattern, adapterContract.projectDiscoveryExclude));
    }
    const files: Array<{ relative: string; source: string }> = [];
    for (const uri of uris.sort((left, right) => left.fsPath < right.fsPath ? -1 : left.fsPath > right.fsPath ? 1 : 0)) {
      const relative = this.relativeToProject(uri);
      if (relative === undefined) {
        continue;
      }
      try {
        files.push({ relative, source: await readFile(uri.fsPath, "utf8") });
      } catch {
        // A file that went away between the search and the read has no tests.
      }
    }
    return scannedListing(files, moduleOf(await readText(path.join(project.root.fsPath, "go.mod"))));
  }

  // fileChanged follows a test file that was written, created or deleted: aru
  // is asked again a moment later, so a save that touches several files lists
  // once, and the editor's own reading rereads the one file.
  private fileChanged(uri: vscode.Uri, deleted: boolean): void {
    if (this.source === "aru") {
      this.cancelRediscover();
      this.rediscoverTimer = setTimeout(() => {
        this.rediscoverTimer = undefined;
        void this.discover();
      }, contract.rediscoverDelayMilliseconds);
      return;
    }
    if (deleted) {
      this.removeFile(uri);
    } else {
      void this.updateFile(uri);
    }
  }

  private cancelRediscover(): void {
    if (this.rediscoverTimer !== undefined) {
      clearTimeout(this.rediscoverTimer);
      this.rediscoverTimer = undefined;
    }
  }

  private relativeToProject(uri: vscode.Uri): string | undefined {
    const project = this.host.project();
    if (project === undefined || uri.scheme !== "file") {
      return undefined;
    }
    const relative = path.relative(project.root.fsPath, uri.fsPath);
    if (relative === "" || relative.startsWith("..") || path.isAbsolute(relative)) {
      return undefined;
    }
    return relative.split(path.sep).join("/");
  }

  // updateFile rereads one test file with the editor's own reading.
  private async updateFile(uri: vscode.Uri): Promise<void> {
    const project = this.host.project();
    const relative = this.relativeToProject(uri);
    if (project === undefined || relative === undefined || layerOf(relative) === undefined) {
      return;
    }
    let source: string;
    try {
      source = await readFile(uri.fsPath, "utf8");
    } catch {
      this.removeFile(uri);
      return;
    }
    const file = scannedFile(relative, moduleOf(await readText(path.join(project.root.fsPath, "go.mod"))), source);
    if (file === undefined || file.tests.length === 0) {
      this.removeFile(uri);
      return;
    }
    this.showFile(project, file);
  }

  // showFile puts a file's tests, and their subtests, under its layer.
  private showFile(project: AranduProject, file: ListedFile): void {
    const uri = vscode.Uri.joinPath(project.root, ...file.relative.split("/"));
    const layerItem = this.layerItem(file.layer);
    const base = { layer: file.layer, relative: file.relative, tags: file.tags, packagePath: file.packagePath };
    const label = file.layer.directory === "" ? file.relative : file.relative.slice(file.layer.directory.length + 1);
    const fileItem = layerItem.children.get(uri.toString()) ?? this.controller.createTestItem(uri.toString(), label, uri);
    this.data.set(fileItem, { ...base, kind: "file", name: "" });
    const children: vscode.TestItem[] = [];
    for (const test of file.tests) {
      const item = this.controller.createTestItem(`${uri.toString()}#${test.name}`, test.name, uri);
      item.range = new vscode.Range(editorLine(test.line), 0, editorLine(test.line), 0);
      this.data.set(item, { ...base, kind: "test", name: test.name });
      // Subtests come depth first, so each one's parent is already shown: the
      // nearest one whose name is a prefix of its own.
      const shown = new Map<string, vscode.TestItem>([[test.name, item]]);
      for (const subtest of test.subtests) {
        let parent: string | undefined = subtest.name.slice(0, subtest.name.lastIndexOf("/"));
        while (parent !== undefined && !shown.has(parent)) {
          parent = parent.includes("/") ? parent.slice(0, parent.lastIndexOf("/")) : undefined;
        }
        const parentItem = shown.get(parent ?? test.name) ?? item;
        const label = subtest.name.slice((parent ?? test.name).length + 1);
        const child = this.controller.createTestItem(`${uri.toString()}#${subtest.name}`, label, uri);
        child.range = new vscode.Range(editorLine(subtest.line), 0, editorLine(subtest.line), 0);
        this.data.set(child, { ...base, kind: "subtest", name: subtest.name });
        parentItem.children.add(child);
        shown.set(subtest.name, child);
      }
      children.push(item);
    }
    fileItem.children.replace(children);
    layerItem.children.add(fileItem);
  }

  private removeFile(uri: vscode.Uri): void {
    const layerItems: vscode.TestItem[] = [];
    this.controller.items.forEach((layerItem) => layerItems.push(layerItem));
    for (const layerItem of layerItems) {
      layerItem.children.delete(uri.toString());
      if (layerItem.children.size === 0) {
        this.controller.items.delete(layerItem.id);
      }
    }
  }

  private layerItem(layer: TestLayer): vscode.TestItem {
    const id = `layer:${layer.id}`;
    const existing = this.controller.items.get(id);
    if (existing !== undefined) {
      return existing;
    }
    const item = this.controller.createTestItem(id, layer.label);
    item.sortText = String(this.layers.findIndex((candidate) => candidate.id === layer.id)).padStart(3, "0");
    this.data.set(item, { kind: "layer", layer, relative: layer.directory, name: "", tags: [], packagePath: "" });
    // Items are added in layer order, and the Testing view keeps it.
    const ordered: vscode.TestItem[] = [];
    this.controller.items.forEach((candidate) => ordered.push(candidate));
    ordered.push(item);
    ordered.sort((left, right) => (left.sortText ?? "").localeCompare(right.sortText ?? ""));
    this.controller.items.replace(ordered);
    if (isE2E(layer)) {
      void this.describeE2ELayer();
    }
    return item;
  }

  private async describeE2ELayer(): Promise<void> {
    const layer = testLayers.find(isE2E);
    const item = layer === undefined ? undefined : this.controller.items.get(`layer:${layer.id}`);
    if (item === undefined) {
      return;
    }
    item.description = describeChoice(await this.currentChoice());
  }

  // ---- running -----------------------------------------------------------

  private leaves(request: vscode.TestRunRequest): vscode.TestItem[] {
    const excluded = new Set((request.exclude ?? []).map((item) => item.id));
    const out: vscode.TestItem[] = [];
    const visit = (item: vscode.TestItem): void => {
      if (excluded.has(item.id)) {
        return;
      }
      const kind = this.data.get(item)?.kind;
      if (kind === "test" || kind === "subtest") {
        out.push(item);
        return;
      }
      item.children.forEach(visit);
    };
    if (request.include === undefined) {
      this.controller.items.forEach(visit);
    } else {
      request.include.forEach(visit);
    }
    return out;
  }

  // collectSubtests adds the subtests shown under a test to the items results
  // are matched to, keeping an item already there.
  private collectSubtests(item: vscode.TestItem, byKey: Map<string, vscode.TestItem>): void {
    item.children.forEach((child) => {
      const data = this.data.get(child);
      if (data?.kind !== "subtest") {
        return;
      }
      const key = resultKey(data.packagePath, data.name);
      if (!byKey.has(key)) {
        byKey.set(key, child);
      }
      this.collectSubtests(child, byKey);
    });
  }

  // itemFor answers the item a result key belongs to or, for output, the
  // nearest test above it that has one.
  private static itemFor(byKey: ReadonlyMap<string, vscode.TestItem>, key: string | undefined, nearest: boolean): vscode.TestItem | undefined {
    for (let candidate = key; candidate !== undefined; candidate = nearest ? parentKey(candidate) : undefined) {
      const item = byKey.get(candidate);
      if (item !== undefined) {
        return item;
      }
    }
    return undefined;
  }

  // refuse answers why tests cannot run here, or undefined when they can. An
  // untrusted workspace never runs a test: a test is the project's code.
  private refuse(): string | undefined {
    if (this.host.project() === undefined) {
      return "Select an Arandu project before running its tests.";
    }
    if (!vscode.workspace.isTrusted) {
      return untrustedMessage;
    }
    return undefined;
  }

  private async run(request: vscode.TestRunRequest, token: vscode.CancellationToken): Promise<void> {
    const run = this.controller.createTestRun(request);
    const leaves = this.leaves(request);
    const finished = new Set<vscode.TestItem>();
    try {
      const refusal = this.refuse();
      const project = this.host.project();
      if (refusal !== undefined || project === undefined) {
        for (const item of leaves) {
          run.errored(item, new vscode.TestMessage(refusal ?? untrustedMessage));
        }
        return;
      }
      const root = project.root.fsPath;
      const byKey = new Map<string, vscode.TestItem>();
      const selections: TestSelection[] = [];
      for (const item of leaves) {
        const data = this.data.get(item);
        if (data === undefined) {
          continue;
        }
        if (data.packagePath === "") {
          run.errored(item, new vscode.TestMessage(`${path.join(root, "go.mod")} declares no module, so the test cannot be matched to its package.`));
          finished.add(item);
          continue;
        }
        byKey.set(resultKey(data.packagePath, data.name), item);
        selections.push({ layer: data.layer.id, packagePath: data.packagePath, name: data.name, tags: data.tags });
      }
      // The subtests under a test that runs report their own results.
      leaves.forEach((item) => this.collectSubtests(item, byKey));
      const choice = await this.currentChoice();
      const aru = await resolveAruExecutable(project.folder);
      const traceDir = await this.runTraceDir();
      for (const batch of testBatches(selections, choice)) {
        const entries = batch.tests.flatMap((test) => {
          const item = byKey.get(resultKey(test.packagePath, test.name));
          return item === undefined ? [] : [{ test, item }];
        });
        const items = entries.map((entry) => entry.item);
        if (token.isCancellationRequested) {
          break;
        }
        items.forEach((item) => run.enqueued(item));
        const env = batch.e2e ? e2eEnvironment(choice, { headed: false, traceDir }) : {};
        run.appendOutput(crlf(`$ aru ${batch.args.join(" ")}\n`));
        this.host.output.info(`Running ${aru.executable} ${batch.args.join(" ")} in ${root}.`);
        const collector = new ResultCollector();
        const failedPackages = new Map<string, string>();
        let tail = "";
        const abort = new AbortController();
        const cancel = token.onCancellationRequested(() => abort.abort());
        const code = await runAru(aru.executable, batch.args, { cwd: root, env, signal: abort.signal }, {
          onOutput: (text) => {
            tail = (tail + text).slice(-4000);
            run.appendOutput(crlf(text));
          },
          onEvent: (event) => {
            const update = collector.feed(event);
            if (update === undefined) {
              return;
            }
            if (update.kind === "package-failed") {
              failedPackages.set(update.packagePath, update.output);
              return;
            }
            const item = AranduTesting.itemFor(byKey, update.key, update.kind === "output");
            switch (update.kind) {
              case "output":
                run.appendOutput(crlf(update.text), undefined, item);
                return;
              case "started":
                if (item !== undefined) {
                  run.started(item);
                }
                return;
              case "passed":
                if (item !== undefined) {
                  run.passed(item, update.elapsedMs);
                  finished.add(item);
                }
                return;
              case "skipped":
                if (item !== undefined) {
                  run.skipped(item);
                  finished.add(item);
                }
                return;
              case "failed":
                if (item !== undefined) {
                  run.failed(item, this.failureMessages(item, update.output, batch.e2e, traceDir), update.elapsedMs);
                  finished.add(item);
                }
                return;
            }
          },
        });
        cancel.dispose();
        for (const { test, item } of entries) {
          if (finished.has(item)) {
            continue;
          }
          finished.add(item);
          const packageOutput = failedPackages.get(test.packagePath);
          if (code === null && abort.signal.aborted) {
            run.skipped(item);
          } else if (packageOutput !== undefined) {
            run.errored(item, new vscode.TestMessage(packageOutput || `The package ${test.packagePath} failed to build.`));
          } else if (code !== 0) {
            run.errored(item, new vscode.TestMessage(`aru test ended${code === null ? "" : ` with exit code ${code}`} before reporting this test.\n\n${tail}`));
          } else {
            run.skipped(item);
          }
        }
      }
    } catch (error: unknown) {
      const message = errorMessage(error);
      this.host.output.error(`Running tests failed: ${message}`);
      for (const item of leaves) {
        if (!finished.has(item)) {
          run.errored(item, new vscode.TestMessage(message));
        }
      }
    } finally {
      run.end();
    }
  }

  private failureMessages(item: vscode.TestItem, output: string, e2e: boolean, traceDir: string | undefined): vscode.TestMessage[] {
    const message = new vscode.TestMessage(output.trim() === "" ? "The test failed without output." : output);
    if (item.uri !== undefined) {
      const line = failureLine(output, path.basename(item.uri.fsPath));
      message.location = new vscode.Location(item.uri, new vscode.Position(line ?? item.range?.start.line ?? 0, 0));
    }
    const messages = [message];
    const data = this.data.get(item);
    if (e2e && data !== undefined) {
      const dir = traceDirectory(traceDir ?? traceBases(undefined, process.env, os.tmpdir())[0] ?? "", data.name);
      const link = new vscode.MarkdownString(
        `Trace: \`${dir.replace(/`/g, "")}\` — [Open trace](command:arandu.e2e.openTrace?${encodeURIComponent(JSON.stringify([data.name]))})`
          + ` · [Explain failure](command:arandu.e2e.explain?${encodeURIComponent(JSON.stringify([dir]))})`,
      );
      link.isTrusted = { enabledCommands: ["arandu.e2e.openTrace", "arandu.e2e.explain"] };
      const trace = new vscode.TestMessage(link);
      trace.location = message.location;
      messages.push(trace);
    }
    return messages;
  }

  // debug runs the tests under Delve through the Go extension, one package per
  // session, with -tags e2e and a visible browser for a journey.
  private async debug(request: vscode.TestRunRequest, token: vscode.CancellationToken): Promise<void> {
    const run = this.controller.createTestRun(request);
    const leaves = this.leaves(request);
    try {
      const refusal = this.refuse();
      const project = this.host.project();
      if (refusal !== undefined || project === undefined) {
        leaves.forEach((item) => run.errored(item, new vscode.TestMessage(refusal ?? untrustedMessage)));
        return;
      }
      if (vscode.extensions.getExtension(contract.goExtensionId) === undefined) {
        leaves.forEach((item) => run.errored(item, new vscode.TestMessage(
          `Debugging runs through the Go extension (${contract.goExtensionId}) and Delve. Install it to debug tests.`,
        )));
        return;
      }
      const choice = await this.currentChoice();
      const traceDir = await this.runTraceDir();
      const groups = new Map<string, { data: TestData; items: vscode.TestItem[] }>();
      for (const item of leaves) {
        const data = this.data.get(item);
        if (data === undefined) {
          continue;
        }
        // A subtest is debugged alone, as it runs alone.
        const key = `${path.posix.dirname(data.relative)}|${[...data.tags].sort().join(",")}|${data.name.includes("/") ? data.name : ""}`;
        const group = groups.get(key) ?? { data, items: [] };
        group.items.push(item);
        groups.set(key, group);
      }
      const folder = vscode.workspace.getWorkspaceFolder(project.root);
      for (const { data, items } of groups.values()) {
        if (token.isCancellationRequested) {
          break;
        }
        const e2e = isE2E(data.layer);
        const marker = randomUUID();
        const names = items.map((item) => this.data.get(item)?.name ?? "").filter((name) => name !== "");
        const configuration = debugConfiguration({
          name: `Arandu: debug ${names.length === 1 ? names[0] : path.posix.dirname(data.relative)}`,
          packageDir: path.join(project.root.fsPath, ...path.posix.dirname(data.relative).split("/")),
          names,
          tags: data.tags,
          e2e,
          env: e2e ? e2eEnvironment(choice, { headed: true, traceDir }) : {},
          marker,
        }) as vscode.DebugConfiguration;
        items.forEach((item) => run.started(item));
        const listeners: vscode.Disposable[] = [];
        const ended = new Promise<void>((resolve) => {
          listeners.push(
            vscode.debug.onDidTerminateDebugSession((session) => {
              if (session.configuration.aranduTestRun === marker) {
                resolve();
              }
            }),
            token.onCancellationRequested(() => resolve()),
          );
        });
        try {
          const started = await vscode.debug.startDebugging(folder, configuration, { testRun: run });
          if (!started) {
            items.forEach((item) => run.errored(item, new vscode.TestMessage("The Go extension did not start the debug session.")));
            continue;
          }
          // The session's own output says how the tests ended; the run closes
          // when Delve does, without inventing a result for them.
          await ended;
        } finally {
          listeners.forEach((listener) => listener.dispose());
        }
      }
    } catch (error: unknown) {
      const message = errorMessage(error);
      this.host.output.error(`Debugging tests failed: ${message}`);
      leaves.forEach((item) => run.errored(item, new vscode.TestMessage(message)));
    } finally {
      run.end();
    }
  }

  private async runJourney(uri: unknown, name: unknown, profile: vscode.TestRunProfile): Promise<void> {
    if (typeof uri !== "string" || typeof name !== "string") {
      return;
    }
    const fileUri = vscode.Uri.parse(uri, true);
    let item = this.findTest(fileUri, name);
    if (item === undefined) {
      await (this.source === "aru" ? this.discover() : this.updateFile(fileUri));
      item = this.findTest(fileUri, name);
    }
    if (item === undefined) {
      await vscode.window.showWarningMessage(`${name} is not a test of the selected Arandu project's E2E layer.`);
      return;
    }
    const cancellation = new vscode.CancellationTokenSource();
    try {
      const request = new vscode.TestRunRequest([item], undefined, profile);
      await (profile.kind === vscode.TestRunProfileKind.Debug ? this.debug(request, cancellation.token) : this.run(request, cancellation.token));
    } finally {
      cancellation.dispose();
    }
  }

  private findTest(uri: vscode.Uri, name: string): vscode.TestItem | undefined {
    let found: vscode.TestItem | undefined;
    this.controller.items.forEach((layerItem) => {
      found ??= layerItem.children.get(uri.toString())?.children.get(`${uri.toString()}#${name}`);
    });
    return found;
  }

  // journeyLenses puts Run, Debug headed and Open last trace over every journey
  // of the selected project's E2E layer.
  private journeyLenses(document: vscode.TextDocument): vscode.CodeLens[] {
    const relative = this.relativeToProject(document.uri);
    const layer = relative === undefined ? undefined : layerOf(relative);
    if (layer === undefined || !isE2E(layer)) {
      return [];
    }
    const lenses: vscode.CodeLens[] = [];
    for (const test of parseTestFile(document.getText()).tests) {
      const range = new vscode.Range(editorLine(test.line), 0, editorLine(test.line), 0);
      const args = [document.uri.toString(), test.name];
      lenses.push(
        new vscode.CodeLens(range, { title: "$(play) Run", command: "arandu.e2e.runJourney", arguments: args }),
        new vscode.CodeLens(range, { title: "$(debug-alt) Debug headed", command: "arandu.e2e.debugJourney", arguments: args }),
        new vscode.CodeLens(range, { title: "$(history) Open last trace", command: "arandu.e2e.openTrace", arguments: [test.name] }),
      );
    }
    return lenses;
  }

  // ---- driver and browser ------------------------------------------------

  private async goMod(): Promise<string> {
    const project = this.host.project();
    return project === undefined ? "" : readText(path.join(project.root.fsPath, "go.mod"));
  }

  private async currentChoice(): Promise<DriverChoice | undefined> {
    return resolveChoice(this.host.context.workspaceState.get(contract.driverStateKey), await this.goMod());
  }

  // selectDriver picks the driver and browser journeys run with. The browsers of
  // a driver from a module are offered only while go.mod requires that module.
  private async selectDriver(): Promise<void> {
    const goMod = await this.goMod();
    const current = await this.currentChoice();
    const items: Array<vscode.QuickPickItem & { choice: DriverChoice | undefined }> = [
      {
        label: "Project default",
        description: "the driver the test case registers, with its default browser",
        picked: current === undefined,
        choice: undefined,
      },
      ...driverChoices(goMod).map((choice) => ({
        label: describeChoice(choice),
        description: contract.drivers.find((entry) => entry.driver === choice.driver)?.module || "native driver, the installed Chrome",
        picked: current?.driver === choice.driver && current.browser === choice.browser,
        choice,
      })),
    ];
    const picked = await vscode.window.showQuickPick(items, {
      title: "Arandu: E2E Driver and Browser",
      placeHolder: `Journeys run with ${describeChoice(current)}`,
    });
    if (picked === undefined) {
      return;
    }
    await this.host.context.workspaceState.update(contract.driverStateKey, picked.choice);
    await this.describeE2ELayer();
  }

  // ---- record and explain ------------------------------------------------

  private async readyProject(action: string): Promise<AranduProject | undefined> {
    const project = this.host.project();
    if (project === undefined) {
      await vscode.window.showErrorMessage(`Select an Arandu filesystem project before ${action}.`);
      return undefined;
    }
    if (!vscode.workspace.isTrusted) {
      await vscode.window.showErrorMessage(`Trust this workspace before ${action}.`);
      return undefined;
    }
    return project;
  }

  // record runs aru e2e:record in a terminal the person sees, starting from the
  // development server that is already answering when there is one.
  private async record(): Promise<void> {
    const project = await this.readyProject("recording a journey");
    if (project === undefined) {
      return;
    }
    const url = devServerURL(await readText(path.join(project.root.fsPath, ".env")));
    const running = this.host.devRunning() || await isListening(url);
    const input = await vscode.window.showInputBox({
      title: "Arandu: Record Journey",
      prompt: running
        ? `The development server answers at ${url}; recording starts there. Type a path, a route name or another address to start elsewhere.`
        : "No development server answers; aru serves the application for the recording. Type an address, a path or a route name to start from, or leave it empty.",
      value: running ? url : "",
      placeHolder: "http://localhost:8080/login, /login or login",
    });
    if (input === undefined) {
      return;
    }
    const target = recordTarget(input, running ? url : undefined);
    await this.runInTerminal(project, "Arandu Record Journey", "record", [...contract.recordArgs, ...(target === undefined ? [] : [target])]);
  }

  // explain runs aru e2e:explain on a trace, chosen from the recent ones when
  // none is given.
  private async explain(dir: string | undefined): Promise<void> {
    const project = await this.readyProject("explaining a failure");
    if (project === undefined) {
      return;
    }
    const traceDir = dir ?? await this.pickTrace("Explain which trace?");
    if (traceDir === undefined) {
      return;
    }
    await this.runInTerminal(project, "Arandu Explain Failure", "lightbulb", [...contract.explainArgs, traceDir]);
  }

  private async runInTerminal(project: AranduProject, name: string, icon: string, args: readonly string[]): Promise<void> {
    try {
      const aru = await resolveAruExecutable(project.folder);
      const terminal = vscode.window.createTerminal({
        name,
        cwd: project.root,
        shellPath: aru.executable,
        shellArgs: [...args],
        iconPath: new vscode.ThemeIcon(icon),
        isTransient: true,
      });
      this.host.output.info(`Running ${aru.executable} ${args.join(" ")} in ${project.root.fsPath} by explicit user command.`);
      terminal.show(false);
    } catch (error: unknown) {
      const message = errorMessage(error);
      this.host.output.error(`Cannot run aru ${args.join(" ")}: ${message}`);
      await vscode.window.showErrorMessage(`Arandu: ${message}`);
    }
  }

  // ---- traces ------------------------------------------------------------

  // runTraceDir is where a run from the editor writes its traces: the
  // directory the person set in ARANDU_E2E_TRACE_DIR is left alone, and
  // otherwise each workspace keeps its own, so two projects with a test of the
  // same name never read each other's trace.
  private async runTraceDir(): Promise<string | undefined> {
    if ((process.env[contract.env.traceDir] ?? "") !== "") {
      return undefined;
    }
    const storage = this.host.context.storageUri;
    if (storage === undefined || storage.scheme !== "file") {
      return undefined;
    }
    const dir = path.join(storage.fsPath, "e2e-traces");
    await mkdir(dir, { recursive: true });
    return dir;
  }

  private async bases(): Promise<string[]> {
    return traceBases(await this.runTraceDir(), process.env, os.tmpdir());
  }

  private async openTrace(target: unknown): Promise<void> {
    let dir: string | undefined;
    if (typeof target === "string" && target !== "") {
      dir = await this.lastTrace(target);
      if (dir === undefined) {
        await vscode.window.showInformationMessage(
          `No trace was written for ${target} yet. Run it from the editor, which writes one after every run, or with ARANDU_E2E_TRACE=1.`,
        );
        return;
      }
    } else if (target instanceof vscode.Uri) {
      dir = path.basename(target.fsPath) === contract.traceFile ? path.dirname(target.fsPath) : target.fsPath;
    } else {
      dir = await this.pickTrace("Open which trace?");
    }
    if (dir !== undefined) {
      await this.showTrace(dir);
    }
  }

  // lastTrace answers the most recently written trace of the named test among
  // every directory a trace may be under.
  private async lastTrace(test: string): Promise<string | undefined> {
    let best: { dir: string; time: number } | undefined;
    for (const base of await this.bases()) {
      const dir = traceDirectory(base, test);
      const time = await modified(path.join(dir, contract.traceFile));
      if (time !== undefined && (best === undefined || time > best.time)) {
        best = { dir, time };
      }
    }
    return best?.dir;
  }

  private async pickTrace(title: string): Promise<string | undefined> {
    const found: Array<{ dir: string; time: number }> = [];
    for (const base of await this.bases()) {
      await collectTraces(base, 6, found);
    }
    found.sort((left, right) => right.time - left.time);
    const items: TracePick[] = found.slice(0, 50).map((entry) => ({
      label: path.basename(entry.dir),
      description: new Date(entry.time).toLocaleString(),
      detail: entry.dir,
      dir: entry.dir,
    }));
    items.push({ label: browseLabel, dir: undefined });
    const picked = await vscode.window.showQuickPick(items, { title: `Arandu: ${title}`, matchOnDetail: true });
    if (picked === undefined) {
      return undefined;
    }
    if (picked.dir !== undefined) {
      return picked.dir;
    }
    const selection = await vscode.window.showOpenDialog({
      canSelectFiles: false,
      canSelectFolders: true,
      canSelectMany: false,
      openLabel: "Open trace",
      title: "Select a trace directory (the one holding trace.json)",
    });
    return selection?.[0]?.fsPath;
  }

  // showTrace opens the trace viewer on a trace directory. The page runs under
  // a strict content security policy with scripts limited to its nonce, reads
  // images only from the trace's directory, and can ask the editor for one
  // thing: to explain the failure.
  private async showTrace(dir: string): Promise<void> {
    let trace: Trace;
    try {
      trace = parseTrace(JSON.parse(await readFile(path.join(dir, contract.traceFile), "utf8")));
    } catch (error: unknown) {
      await vscode.window.showErrorMessage(`Arandu: cannot read the trace in ${dir}: ${errorMessage(error)}`);
      return;
    }
    const existing = this.panels.get(dir);
    const panel = existing ?? vscode.window.createWebviewPanel(
      "arandu.trace",
      `Trace: ${trace.test || path.basename(dir)}`,
      vscode.ViewColumn.Beside,
      {
        enableScripts: true,
        enableCommandUris: false,
        enableForms: false,
        localResourceRoots: [vscode.Uri.file(dir)],
      },
    );
    if (existing === undefined) {
      this.panels.set(dir, panel);
      panel.onDidDispose(() => this.panels.delete(dir));
      panel.webview.onDidReceiveMessage((message: unknown) => {
        if (typeof message === "object" && message !== null && (message as { type?: unknown }).type === "explain") {
          void this.explain(dir);
        }
      });
    }
    const images = new Map<string, string>();
    const doms = new Map<string, string>();
    for (const step of trace.steps) {
      const screenshot = step.screenshot === "" ? undefined : safeTracePath(dir, step.screenshot);
      if (screenshot !== undefined && await modified(screenshot) !== undefined) {
        images.set(step.screenshot, panel.webview.asWebviewUri(vscode.Uri.file(screenshot)).toString());
      }
      const dom = step.dom === "" ? undefined : safeTracePath(dir, step.dom);
      if (dom !== undefined) {
        const text = await readLimited(dom, contract.traceDomLimitBytes);
        if (text !== undefined) {
          doms.set(step.dom, text);
        }
      }
    }
    panel.title = `Trace: ${trace.test || path.basename(dir)}`;
    panel.webview.html = renderTraceHtml(trace, {
      cspSource: panel.webview.cspSource,
      nonce: randomBytes(18).toString("base64"),
      image: (relative) => images.get(relative),
      dom: (relative) => doms.get(relative),
    });
    panel.reveal(vscode.ViewColumn.Beside, existing !== undefined);
  }
}

async function readText(file: string): Promise<string> {
  try {
    return await readFile(file, "utf8");
  } catch {
    return "";
  }
}

async function modified(file: string): Promise<number | undefined> {
  try {
    const info = await stat(file);
    return info.isFile() ? info.mtimeMs : undefined;
  } catch {
    return undefined;
  }
}

// readLimited reads at most limit bytes of a file, and says so at the end when
// the file was longer.
async function readLimited(file: string, limit: number): Promise<string | undefined> {
  let handle;
  try {
    handle = await open(file, "r");
    const info = await handle.stat();
    if (!info.isFile()) {
      return undefined;
    }
    const buffer = Buffer.alloc(Math.min(limit, info.size));
    await handle.read(buffer, 0, buffer.length, 0);
    const text = buffer.toString("utf8");
    return info.size > limit ? `${text}\n… truncated at ${Math.round(limit / 1024)} KiB of ${Math.round(info.size / 1024)} KiB` : text;
  } catch {
    return undefined;
  } finally {
    await handle?.close();
  }
}

async function collectTraces(dir: string, depth: number, found: Array<{ dir: string; time: number }>): Promise<void> {
  if (depth < 0 || found.length > 500) {
    return;
  }
  const time = await modified(path.join(dir, contract.traceFile));
  if (time !== undefined) {
    found.push({ dir, time });
  }
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    if (entry.isDirectory() && entry.name !== "steps") {
      await collectTraces(path.join(dir, entry.name), depth - 1, found);
    }
  }
}

function crlf(text: string): string {
  return text.replace(/\r?\n/g, "\r\n");
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
