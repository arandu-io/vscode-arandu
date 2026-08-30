import * as path from "node:path";
import * as vscode from "vscode";
import adapterContract from "./adapterContract.json";

export interface AranduProject {
  readonly key: string;
  readonly label: string;
  readonly description: string;
  readonly root: vscode.Uri;
  readonly folder: vscode.WorkspaceFolder;
}

interface AranduProjectPick extends vscode.QuickPickItem {
  readonly project: AranduProject;
}

export class AranduProjects {
  private readonly state: vscode.Memento;
  private selected: AranduProject | undefined;
  private discoveredCount = 0;

  public constructor(context: vscode.ExtensionContext) {
    this.state = context.workspaceState;
  }

  public get active(): AranduProject | undefined {
    return this.selected;
  }

  public get availableCount(): number {
    return this.discoveredCount;
  }

  public async resolve(): Promise<AranduProject | undefined> {
    const projects = await discoverAranduProjects();
    this.discoveredCount = projects.length;
    if (projects.length === 0) {
      await this.remember(undefined);
      return undefined;
    }

    const rememberedKey = this.selected?.key
      ?? this.state.get<string>(adapterContract.activeProjectStateKey);
    const remembered = projects.find((project) => project.key === rememberedKey);
    if (remembered !== undefined) {
      this.selected = remembered;
      return remembered;
    }

    if (projects.length === 1) {
      const project = projects[0];
      await this.remember(project);
      return project;
    }

    await this.remember(undefined);
    if (!adapterContract.explicitProjectSelectionWhenMultiple) {
      throw new Error("Selecting the first discovered Arandu project is not allowed.");
    }
    return undefined;
  }

  public async choose(): Promise<AranduProject | undefined> {
    const projects = await discoverAranduProjects();
    this.discoveredCount = projects.length;
    if (projects.length === 0) {
      await this.remember(undefined);
      return undefined;
    }
    return this.pick(projects);
  }

  private async pick(projects: readonly AranduProject[]): Promise<AranduProject | undefined> {
    const items: AranduProjectPick[] = projects.map((project) => ({
      label: project.label,
      description: project.description,
      detail: project.root.fsPath,
      picked: project.key === this.selected?.key,
      project,
    }));
    const picked = await vscode.window.showQuickPick(items, {
      title: "Select Arandu Project",
      placeHolder: "Choose the project used by Project Map, Doctor, aru lsp, and aru dev",
      matchOnDescription: true,
      matchOnDetail: true,
    });
    if (picked === undefined) {
      return undefined;
    }
    await this.remember(picked.project);
    return picked.project;
  }

  private async remember(project: AranduProject | undefined): Promise<void> {
    this.selected = project;
    if (project === undefined) {
      await this.state.update(adapterContract.activeProjectStateKey, undefined);
      return;
    }
    await this.state.update(adapterContract.activeProjectStateKey, project.key);
  }
}

export async function discoverAranduProjects(): Promise<readonly AranduProject[]> {
  const projects = new Map<string, AranduProject>();
  for (const folder of vscode.workspace.workspaceFolders ?? []) {
    if (folder.uri.scheme !== "file") {
      continue;
    }
    const pattern = new vscode.RelativePattern(folder, adapterContract.projectDiscoveryGlob);
    const markers = await vscode.workspace.findFiles(pattern, adapterContract.projectDiscoveryExclude);
    for (const marker of markers) {
      const root = vscode.Uri.file(path.dirname(marker.fsPath));
      const key = root.toString();
      if (projects.has(key)) {
        continue;
      }
      const relative = path.relative(folder.uri.fsPath, root.fsPath);
      const label = path.basename(root.fsPath);
      projects.set(key, {
        key,
        label,
        description: relative === "" ? root.fsPath : `${folder.name}/${relative}`,
        root,
        folder: { uri: root, name: label, index: folder.index },
      });
    }
  }
  return [...projects.values()].sort((left, right) => left.key < right.key ? -1 : left.key > right.key ? 1 : 0);
}
