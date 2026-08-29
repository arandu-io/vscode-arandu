import * as path from "node:path";
import * as vscode from "vscode";
import { LanguageClient, State, type LanguageClientOptions, type ServerOptions } from "vscode-languageclient/node";
import adapterContract from "./adapterContract.json";
import { resolveAruExecutable } from "./aru";
import { ProjectMapProvider } from "./projectMap";
import graphContract from "./projectGraphContract.json";
import { parseProjectGraph } from "./projectGraphSchema";

const configureAction = "Configure Aru Path";
const retryAction = "Retry";
const showOutputAction = "Show Output";

let activeController: AranduController | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  activeController = new AranduController(context);
  context.subscriptions.push(activeController);
  await activeController.restart();
}

export async function deactivate(): Promise<void> {
  await activeController?.disposeAsync();
  activeController = undefined;
}

class AranduController implements vscode.Disposable {
  private readonly output = vscode.window.createOutputChannel("Arandu", { log: true });
  private readonly doctorDiagnostics = vscode.languages.createDiagnosticCollection(adapterContract.diagnosticsCollection);
  private readonly provider = new ProjectMapProvider();
  private readonly tree = vscode.window.createTreeView("arandu.projectMap", { treeDataProvider: this.provider });
  private readonly status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  private readonly devStatus = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 99);
  private readonly permanentDisposables: vscode.Disposable[] = [];
  private runtimeDisposables: vscode.Disposable[] = [];
  private client: LanguageClient | undefined;
  private refreshTimer: NodeJS.Timeout | undefined;
  private devTerminal: vscode.Terminal | undefined;
  private restartQueue: Promise<void> = Promise.resolve();
  private stopping = false;
  private failurePromptOpen = false;
  private disposed = false;

  public constructor(_context: vscode.ExtensionContext) {
    this.status.name = "Arandu";
    this.status.show();
    this.devStatus.name = "Arandu Development Server";
    this.devStatus.show();
    this.setDevRunning(false);
    this.permanentDisposables.push(
      this.output,
      this.doctorDiagnostics,
      this.provider,
      this.tree,
      this.status,
      this.devStatus,
      vscode.commands.registerCommand("arandu.projectMap.refresh", () => this.refresh()),
      vscode.commands.registerCommand("arandu.languageServer.restart", () => this.restart()),
      vscode.commands.registerCommand("arandu.aru.configure", () => this.configureAruPath()),
      vscode.commands.registerCommand("arandu.output.show", () => this.output.show(true)),
      vscode.commands.registerCommand("arandu.dev.start", () => this.startDev()),
      vscode.commands.registerCommand("arandu.dev.stop", () => this.stopDev()),
      vscode.commands.registerCommand("arandu.dev.restart", () => this.restartDev()),
      vscode.window.onDidCloseTerminal((terminal) => {
        if (terminal === this.devTerminal) {
          this.devTerminal = undefined;
          this.setDevRunning(false);
        }
      }),
      vscode.workspace.onDidGrantWorkspaceTrust(() => void this.restart()),
      vscode.workspace.onDidChangeWorkspaceFolders(() => void this.restart()),
      vscode.workspace.onDidChangeConfiguration((event) => {
        if (event.affectsConfiguration("arandu.aru.path")) {
          void this.restart();
        }
      }),
    );
  }

  public dispose(): void {
    void this.disposeAsync();
  }

  public async disposeAsync(): Promise<void> {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.stopDev();
    await this.stopRuntime();
    for (const disposable of this.permanentDisposables.splice(0)) {
      disposable.dispose();
    }
  }

  public restart(): Promise<void> {
    this.restartQueue = this.restartQueue
      .then(async () => {
        if (this.disposed) {
          return;
        }
        await this.stopRuntime();
        await this.startRuntime();
      })
      .catch(async (error: unknown) => {
        await this.stopRuntime();
        await this.reportFailure(error, true);
      });
    return this.restartQueue;
  }

  private async startRuntime(): Promise<void> {
    const folder = await findAranduWorkspace();
    this.provider.setGraph(undefined);
    this.doctorDiagnostics.clear();
    if (folder === undefined) {
      this.setInactive("$(circle-slash) Arandu: no project", "Open a filesystem workspace containing arandu.toml.");
      return;
    }
    if (adapterContract.trustedWorkspacesOnly && !vscode.workspace.isTrusted) {
      this.setInactive("$(lock) Arandu: trust required", "Trust this workspace to start aru lsp.");
      this.status.command = "workbench.trust.manage";
      this.tree.message = "Trust this workspace to start the Arandu language server.";
      return;
    }
    if (adapterContract.filesystemWorkspacesOnly && folder.uri.scheme !== "file") {
      this.setInactive("$(circle-slash) Arandu: local files required", "Aru requires a local filesystem workspace.");
      return;
    }

    this.setStarting();
    const aru = await resolveAruExecutable(folder);
    this.output.info(`Starting ${aru.executable} lsp in ${folder.uri.fsPath} (${aru.source}).`);
    const watcher = this.createProjectWatcher(folder);
    const serverOptions: ServerOptions = {
      command: aru.executable,
      args: adapterContract.serverArgs,
      options: { cwd: folder.uri.fsPath, shell: false },
    };
    const clientOptions: LanguageClientOptions = {
      documentSelector: [{ language: "kyse", scheme: "file" }],
      diagnosticCollectionName: "arandu",
      outputChannel: this.output,
      workspaceFolder: folder,
    };
    const client = new LanguageClient("arandu", "Arandu", serverOptions, clientOptions);
    this.client = client;
    this.runtimeDisposables.push(
      client.onDidChangeState((event) => {
        if (this.client !== client || this.stopping) {
          return;
        }
        if (event.newState === State.Starting) {
          this.setStarting();
        } else if (event.newState === State.StartFailed || event.newState === State.Stopped) {
          void this.reportFailure(new Error("The Arandu language server stopped unexpectedly."), true);
        }
      }),
      watcher,
    );
    await client.start();
    this.setReady(aru.executable);
    await this.refreshGraph();
  }

  private async stopRuntime(): Promise<void> {
    if (this.refreshTimer !== undefined) {
      clearTimeout(this.refreshTimer);
      this.refreshTimer = undefined;
    }
    for (const disposable of this.runtimeDisposables.splice(0)) {
      disposable.dispose();
    }
    const client = this.client;
    this.client = undefined;
    if (client !== undefined) {
      this.stopping = true;
      try {
        await client.stop();
      } catch (error: unknown) {
        this.output.warn(`Stopping aru lsp failed: ${errorMessage(error)}`);
      } finally {
        this.stopping = false;
      }
    }
  }

  private async refresh(): Promise<void> {
    if (this.client === undefined || this.client.state !== State.Running) {
      await this.restart();
      return;
    }
    await this.refreshGraph();
  }

  private async refreshGraph(): Promise<void> {
    const client = this.client;
    if (client === undefined || client.state !== State.Running) {
      return;
    }
    try {
      const response = await client.sendRequest<unknown>(graphContract.request);
      if (this.client !== client) {
        return;
      }
      const graph = parseProjectGraph(response);
      this.provider.setGraph(graph);
      this.publishDoctorDiagnostics(graph);
      this.tree.message = undefined;
      this.tree.description = `${graph.nodes.length}`;
      this.output.info(`Project Map refreshed: ${graph.nodes.length} nodes.`);
    } catch (error: unknown) {
      if (this.client !== client || this.stopping || this.disposed) {
        return;
      }
      this.provider.setGraph(undefined);
      this.doctorDiagnostics.clear();
      await this.reportFailure(error, true);
    }
  }

  private createProjectWatcher(folder: vscode.WorkspaceFolder): vscode.FileSystemWatcher {
    const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(folder, "**/*"));
    const schedule = (uri: vscode.Uri): void => {
      if (!isRelevantProjectPath(folder, uri)) {
        return;
      }
      if (this.refreshTimer !== undefined) {
        clearTimeout(this.refreshTimer);
      }
      this.refreshTimer = setTimeout(() => {
        this.refreshTimer = undefined;
        void this.refreshGraph();
      }, adapterContract.debounceMilliseconds);
    };
    this.runtimeDisposables.push(
      watcher.onDidCreate(schedule),
      watcher.onDidChange(schedule),
      watcher.onDidDelete(schedule),
    );
    return watcher;
  }

  private setStarting(): void {
    this.status.text = "$(sync~spin) Arandu: starting";
    this.status.tooltip = "Starting aru lsp.";
    this.status.command = "arandu.output.show";
    this.tree.message = "Starting the Arandu language server…";
  }

  private setReady(executable: string): void {
    this.status.text = "$(check) Arandu: ready";
    this.status.tooltip = `Language intelligence and Project Map are ready via ${executable}.`;
    this.status.command = "arandu.projectMap.refresh";
    this.tree.message = undefined;
  }

  private setInactive(text: string, tooltip: string): void {
    this.status.text = text;
    this.status.tooltip = tooltip;
    this.status.command = undefined;
    this.tree.description = undefined;
    this.tree.message = tooltip;
  }

  private async reportFailure(error: unknown, prompt: boolean): Promise<void> {
    const message = errorMessage(error);
    this.output.error(message);
    this.status.text = "$(error) Arandu: action needed";
    this.status.tooltip = message;
    this.status.command = "arandu.aru.configure";
    this.tree.description = undefined;
    this.tree.message = message;
    if (!prompt || this.failurePromptOpen || this.disposed) {
      return;
    }
    this.failurePromptOpen = true;
    try {
      const action = await vscode.window.showErrorMessage(
        `Arandu: ${message}`,
        retryAction,
        configureAction,
        showOutputAction,
      );
      if (action === retryAction) {
        void this.restart();
      } else if (action === configureAction) {
        await this.configureAruPath();
      } else if (action === showOutputAction) {
        this.output.show(true);
      }
    } finally {
      this.failurePromptOpen = false;
    }
  }

  private async configureAruPath(): Promise<void> {
    const folder = await findAranduWorkspace();
    if (folder === undefined) {
      await vscode.window.showWarningMessage("Open an Arandu filesystem workspace before configuring Aru.");
      return;
    }
    const selection = await vscode.window.showOpenDialog({
      canSelectFiles: true,
      canSelectFolders: false,
      canSelectMany: false,
      defaultUri: vscode.Uri.file("/opt/homebrew/bin"),
      openLabel: "Use as Aru executable",
      title: "Select the Aru executable",
    });
    if (selection?.[0] === undefined) {
      return;
    }
    await vscode.workspace
      .getConfiguration("arandu", folder.uri)
      .update("aru.path", selection[0].fsPath, vscode.ConfigurationTarget.WorkspaceFolder);
    if (this.devTerminal === undefined) {
      this.setDevRunning(false);
    }
  }

  private async startDev(): Promise<void> {
    if (this.devTerminal !== undefined) {
      this.devTerminal.show(false);
      return;
    }
    if (!adapterContract.manualDevOnly) {
      throw new Error("The Arandu development server must remain an explicit editor action.");
    }
    const folder = await findAranduWorkspace();
    if (folder === undefined) {
      await vscode.window.showErrorMessage("Open an Arandu filesystem workspace before starting aru dev.");
      return;
    }
    if (!vscode.workspace.isTrusted) {
      await vscode.window.showErrorMessage("Trust this workspace before starting aru dev.");
      return;
    }
    try {
      const aru = await resolveAruExecutable(folder);
      const terminal = vscode.window.createTerminal({
        name: "Arandu Dev",
        cwd: folder.uri,
        shellPath: aru.executable,
        shellArgs: adapterContract.devArgs,
        iconPath: new vscode.ThemeIcon("server-process"),
        isTransient: true,
      });
      this.devTerminal = terminal;
      this.setDevRunning(true);
      this.output.info(`Starting ${aru.executable} dev in ${folder.uri.fsPath} by explicit user command.`);
      terminal.show(false);
    } catch (error: unknown) {
      const message = errorMessage(error);
      this.output.error(`Cannot start aru dev: ${message}`);
      this.devStatus.text = "$(error) Arandu Dev: action needed";
      this.devStatus.tooltip = message;
      this.devStatus.command = "arandu.aru.configure";
      const action = await vscode.window.showErrorMessage(`Arandu Dev: ${message}`, configureAction, showOutputAction);
      if (action === configureAction) {
        await this.configureAruPath();
      } else if (action === showOutputAction) {
        this.output.show(true);
      }
    }
  }

  private stopDev(): void {
    const terminal = this.devTerminal;
    this.devTerminal = undefined;
    terminal?.dispose();
    this.setDevRunning(false);
  }

  private async restartDev(): Promise<void> {
    this.stopDev();
    await this.startDev();
  }

  private setDevRunning(running: boolean): void {
    void vscode.commands.executeCommand("setContext", "arandu.dev.running", running);
    if (running) {
      this.devStatus.text = "$(debug-stop) Arandu Dev: running";
      this.devStatus.tooltip = "Stop the Arandu development server.";
      this.devStatus.command = "arandu.dev.stop";
    } else {
      this.devStatus.text = "$(play) Arandu Dev";
      this.devStatus.tooltip = "Start aru dev in this trusted workspace.";
      this.devStatus.command = "arandu.dev.start";
    }
  }

  private publishDoctorDiagnostics(graph: ReturnType<typeof parseProjectGraph>): void {
    this.doctorDiagnostics.clear();
    const diagnosticsGroup = graph.groups.find((group) => group.id === "diagnostics");
    if (diagnosticsGroup === undefined) {
      return;
    }
    const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
    const diagnosticsByURI = new Map<string, { uri: vscode.Uri; diagnostics: vscode.Diagnostic[] }>();
    for (const nodeID of diagnosticsGroup.nodeIds) {
      const node = nodes.get(nodeID);
      if (node?.file === undefined) {
        continue;
      }
      const uri = vscode.Uri.parse(node.file, true);
      const start = new vscode.Position(node.line ?? 0, node.column ?? 0);
      const end = new vscode.Position(start.line, start.character + 1);
      const severity = node.level === "error"
        ? vscode.DiagnosticSeverity.Error
        : vscode.DiagnosticSeverity.Warning;
      const message = node.detail === undefined ? node.label : `${node.label}: ${node.detail}`;
      const diagnostic = new vscode.Diagnostic(new vscode.Range(start, end), message, severity);
      diagnostic.source = adapterContract.diagnosticsCollection;
      diagnostic.code = node.kind;
      const key = uri.toString();
      const entry = diagnosticsByURI.get(key) ?? { uri, diagnostics: [] };
      entry.diagnostics.push(diagnostic);
      diagnosticsByURI.set(key, entry);
    }
    for (const entry of diagnosticsByURI.values()) {
      this.doctorDiagnostics.set(entry.uri, entry.diagnostics);
    }
  }
}

async function findAranduWorkspace(): Promise<vscode.WorkspaceFolder | undefined> {
  for (const folder of vscode.workspace.workspaceFolders ?? []) {
    if (folder.uri.scheme !== "file") {
      continue;
    }
    try {
      await vscode.workspace.fs.stat(vscode.Uri.joinPath(folder.uri, "arandu.toml"));
      return folder;
    } catch {
      // This workspace folder is not an Arandu project root.
    }
  }
  return undefined;
}

function isRelevantProjectPath(folder: vscode.WorkspaceFolder, uri: vscode.Uri): boolean {
  if (uri.scheme !== "file") {
    return false;
  }
  const relative = path.relative(folder.uri.fsPath, uri.fsPath).split(path.sep).join("/");
  if (relative === "" || relative.startsWith("../")) {
    return false;
  }
  return adapterContract.relevantPaths.some((candidate) => {
    if (candidate.endsWith("/")) {
      return relative.startsWith(candidate);
    }
    return relative === candidate || (candidate === "arandu.mod.toml" && relative.endsWith(`/${candidate}`));
  });
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
