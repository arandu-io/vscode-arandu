import { execFile } from "node:child_process";
import * as https from "node:https";
import * as vscode from "vscode";
import { resolveAruExecutable, resolveHomebrewExecutable, type AruResolution } from "./aru";
import { isNewerStableVersion, parseStableVersion } from "./semver";
import updateContract from "./updateContract.json";

interface CachedRelease {
  readonly checkedAt: number;
  readonly version: string;
}

interface AvailableUpdate {
  readonly resolution: AruResolution;
  readonly installed: string;
  readonly latest: string;
}

const notNowAction = "Not Now";
const configureAction = "Configure Aru Path";

export class AruUpdateManager implements vscode.Disposable {
  private readonly status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 98);
  private readonly globalState: vscode.Memento;
  private readonly disposables: vscode.Disposable[] = [];
  private available: AvailableUpdate | undefined;
  private updateExecution: vscode.TaskExecution | undefined;
  private checkQueue: Promise<void> = Promise.resolve();
  private promptOpen = false;
  private disposed = false;

  public constructor(
    context: vscode.ExtensionContext,
    private readonly output: vscode.LogOutputChannel,
    private readonly findWorkspace: () => Promise<vscode.WorkspaceFolder | undefined>,
  ) {
    this.globalState = context.globalState;
    this.status.name = "Aru Update";
    this.disposables.push(
      this.status,
      vscode.commands.registerCommand("arandu.aru.updateWithHomebrew", () => this.updateWithHomebrew()),
    );
  }

  public dispose(): void {
    this.disposed = true;
    for (const disposable of this.disposables.splice(0)) {
      disposable.dispose();
    }
  }

  public check(resolution: AruResolution): Promise<void> {
    this.checkQueue = this.checkQueue
      .then(() => this.performCheck(resolution))
      .catch((error: unknown) => {
        this.output.debug(`Aru update check skipped: ${errorMessage(error)}`);
      });
    return this.checkQueue;
  }

  private async performCheck(resolution: AruResolution): Promise<void> {
    if (this.disposed) {
      return;
    }
    const reported = await readAruVersion(resolution.executable);
    const installed = reported === undefined ? undefined : parseStableVersion(reported);
    if (installed === undefined) {
      this.clearAvailableUpdate();
      return;
    }
    const latest = await latestStableRelease(this.globalState, this.output);
    if (latest === undefined || this.disposed) {
      return;
    }
    if (!isNewerStableVersion(latest, installed.text)) {
      this.clearAvailableUpdate();
      return;
    }

    this.available = { resolution, installed: installed.text, latest };
    const updateAction = resolution.managedByHomebrew ? updateContract.updateAction : configureAction;
    const message = resolution.managedByHomebrew
      ? `Aru ${latest} is available; ${installed.text} is installed.`
      : `Aru ${latest} is available; ${installed.text} at ${resolution.executable} is not managed by Homebrew.`;
    this.status.text = `$(cloud-download) Aru ${latest} available`;
    this.status.tooltip = resolution.managedByHomebrew
      ? `${message} Update only when you choose.`
      : `${message} Configure Aru Path to use an updated executable.`;
    this.status.command = resolution.managedByHomebrew ? "arandu.aru.updateWithHomebrew" : "arandu.aru.configure";
    this.status.show();

    const dismissed = this.globalState.get<string>(updateContract.dismissedVersionKey);
    if (dismissed === latest || this.promptOpen) {
      return;
    }
    this.promptOpen = true;
    try {
      const action = await vscode.window.showWarningMessage(
        message,
        updateAction,
        notNowAction,
      );
      try {
        await this.globalState.update(updateContract.dismissedVersionKey, latest);
      } catch (error: unknown) {
        this.output.debug(`Aru update dismissal was not saved: ${errorMessage(error)}`);
      }
      if (action === updateContract.updateAction) {
        await this.updateWithHomebrew();
      } else if (action === configureAction) {
        await vscode.commands.executeCommand("arandu.aru.configure");
      }
    } finally {
      this.promptOpen = false;
    }
  }

  private clearAvailableUpdate(): void {
    this.available = undefined;
    this.status.hide();
  }

  private async updateWithHomebrew(): Promise<void> {
    if (!updateContract.manualUpdateOnly) {
      throw new Error("Aru updates must remain explicit editor actions.");
    }
    if (this.updateExecution !== undefined) {
      await vscode.window.showInformationMessage("The Homebrew update for Aru is already running.");
      return;
    }
    const available = this.available;
    if (available === undefined) {
      await vscode.window.showInformationMessage("No Aru update is currently available.");
      return;
    }
    if (!available.resolution.managedByHomebrew) {
      const action = await vscode.window.showWarningMessage(
        "The active Aru executable is not managed by Homebrew. Configure Aru Path to use an updated executable.",
        configureAction,
      );
      if (action === configureAction) {
        await vscode.commands.executeCommand("arandu.aru.configure");
      }
      return;
    }
    if (!vscode.workspace.isTrusted) {
      await vscode.window.showWarningMessage("Trust this workspace before updating Aru with Homebrew.");
      return;
    }
    const folder = await this.findWorkspace();
    if (folder === undefined) {
      await vscode.window.showWarningMessage("Open an Arandu filesystem workspace before updating Aru.");
      return;
    }

    let active: AruResolution;
    try {
      active = await resolveAruExecutable(folder);
    } catch (error: unknown) {
      await vscode.window.showErrorMessage(`Arandu: ${errorMessage(error)}`);
      return;
    }
    if (active.executable !== available.resolution.executable || !active.managedByHomebrew) {
      await vscode.window.showWarningMessage("The active Aru executable changed. Wait for its update check before updating with Homebrew.");
      return;
    }

    let brew;
    try {
      brew = await resolveHomebrewExecutable();
    } catch (error: unknown) {
      await vscode.window.showErrorMessage(`Arandu: ${errorMessage(error)}`);
      return;
    }

    const task = new vscode.Task(
      { type: "arandu", task: "update-aru" },
      folder,
      "Update Aru with Homebrew",
      "Arandu",
      new vscode.ProcessExecution(brew.executable, updateContract.brewUpgradeArgs, { cwd: folder.uri.fsPath }),
    );
    task.presentationOptions = {
      clear: true,
      echo: true,
      focus: true,
      panel: vscode.TaskPanelKind.Dedicated,
      reveal: vscode.TaskRevealKind.Always,
      showReuseMessage: false,
    };

    try {
      const execution = await vscode.tasks.executeTask(task);
      this.updateExecution = execution;
      const exitCode = await this.waitForTask(execution);
      if (exitCode !== 0) {
        await vscode.window.showErrorMessage("Homebrew did not complete the Aru update. Review the visible task output.");
        return;
      }
      await vscode.window.showInformationMessage("Homebrew finished updating Aru.");
      await this.performCheck(active);
    } catch (error: unknown) {
      await vscode.window.showErrorMessage(`Cannot update Aru with Homebrew: ${errorMessage(error)}`);
    } finally {
      this.updateExecution = undefined;
    }
  }

  private waitForTask(execution: vscode.TaskExecution): Promise<number | undefined> {
    return new Promise((resolve) => {
      const listener = vscode.tasks.onDidEndTaskProcess((event) => {
        if (event.execution !== execution) {
          return;
        }
        listener.dispose();
        resolve(event.exitCode);
      });
    });
  }
}

function readAruVersion(executable: string): Promise<string | undefined> {
  return new Promise((resolve) => {
    execFile(executable, ["version"], {
      encoding: "utf8",
      timeout: updateContract.versionTimeoutMilliseconds,
      windowsHide: true,
    }, (error, stdout) => {
      resolve(error === null ? stdout.trim() : undefined);
    });
  });
}

async function latestStableRelease(state: vscode.Memento, output: vscode.LogOutputChannel): Promise<string | undefined> {
  const now = Date.now();
  const cached = validCache(state.get<unknown>(updateContract.cacheKey));
  if (cached !== undefined && cached.checkedAt <= now && now - cached.checkedAt < updateContract.cacheMilliseconds) {
    return cached.version;
  }

  let version: string | undefined;
  try {
    version = await requestLatestStableRelease();
  } catch (error: unknown) {
    output.debug(`Aru release request skipped: ${errorMessage(error)}`);
    return undefined;
  }
  if (version === undefined) {
    return undefined;
  }
  try {
    await state.update(updateContract.cacheKey, { checkedAt: now, version } satisfies CachedRelease);
  } catch (error: unknown) {
    output.debug(`Aru release cache was not saved: ${errorMessage(error)}`);
  }
  return version;
}

function validCache(value: unknown): CachedRelease | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const candidate = value as { checkedAt?: unknown; version?: unknown };
  if (typeof candidate.checkedAt !== "number" || !Number.isFinite(candidate.checkedAt) || typeof candidate.version !== "string") {
    return undefined;
  }
  const version = parseStableVersion(candidate.version);
  return version === undefined ? undefined : { checkedAt: candidate.checkedAt, version: version.text };
}

function requestLatestStableRelease(): Promise<string | undefined> {
  return new Promise((resolve, reject) => {
    const request = https.get(updateContract.latestReleaseURL, {
      headers: {
        Accept: "application/vnd.github+json",
        "User-Agent": "arandu-vscode-extension",
        "X-GitHub-Api-Version": "2022-11-28",
      },
    }, (response) => {
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`GitHub release request returned HTTP ${response.statusCode ?? "unknown"}.`));
        return;
      }
      response.setEncoding("utf8");
      let body = "";
      response.on("data", (chunk: string) => {
        body += chunk;
        if (body.length > 256 * 1024) {
          request.destroy(new Error("GitHub release response exceeded 256 KiB."));
        }
      });
      response.on("aborted", () => reject(new Error("GitHub release response ended early.")));
      response.on("error", reject);
      response.on("end", () => {
        try {
          const payload = JSON.parse(body) as { draft?: unknown; prerelease?: unknown; tag_name?: unknown };
          if (payload.draft !== false || payload.prerelease !== false || typeof payload.tag_name !== "string") {
            resolve(undefined);
            return;
          }
          resolve(parseStableVersion(payload.tag_name)?.text);
        } catch (error: unknown) {
          reject(error);
        }
      });
    });
    const timer = setTimeout(() => {
      request.destroy(new Error("GitHub release request timed out."));
    }, updateContract.requestTimeoutMilliseconds);
    request.on("close", () => clearTimeout(timer));
    request.on("error", reject);
  });
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
