import * as vscode from "vscode";
import contract from "./projectGraphContract.json";
import type { AranduProject } from "./projects";
import type { ProjectGraph, ProjectGraphLocation, ProjectGraphNode } from "./projectGraphSchema";
import {
  buildProjectMapModel,
  nodeLocation,
  presentNode,
  relationLocation,
  type ProjectMapModel,
  type Relation,
  type RelationGroup,
} from "./projectMapModel";

interface ProjectEntry {
  readonly type: "project";
  readonly project: AranduProject | undefined;
}

interface GroupEntry {
  readonly type: "group";
  readonly id: string;
  readonly label: string;
}

interface NodeEntry {
  readonly type: "node";
  readonly node: ProjectGraphNode;
}

interface RelationGroupEntry {
  readonly type: "relations";
  readonly node: ProjectGraphNode;
  readonly group: RelationGroup;
}

interface RelationEntry {
  readonly type: "relation";
  readonly relation: Relation;
}

type ProjectMapEntry = ProjectEntry | GroupEntry | NodeEntry | RelationGroupEntry | RelationEntry;

const emptyModel: ProjectMapModel = { nodes: new Map(), children: new Map(), relations: new Map() };

export class ProjectMapProvider implements vscode.TreeDataProvider<ProjectMapEntry> {
  private readonly changed = new vscode.EventEmitter<ProjectMapEntry | undefined | null | void>();
  private graph: ProjectGraph | undefined;
  private model: ProjectMapModel = emptyModel;
  private project: AranduProject | undefined;

  public readonly onDidChangeTreeData = this.changed.event;

  public dispose(): void {
    this.changed.dispose();
  }

  public setProject(project: AranduProject | undefined): void {
    this.project = project;
    this.changed.fire();
  }

  public setGraph(graph: ProjectGraph | undefined): void {
    this.graph = graph;
    this.model = graph === undefined ? emptyModel : buildProjectMapModel(graph);
    this.changed.fire();
  }

  public getTreeItem(entry: ProjectMapEntry): vscode.TreeItem {
    if (entry.type === "project") {
      const item = new vscode.TreeItem(
        entry.project?.label ?? "Select Arandu Project",
        vscode.TreeItemCollapsibleState.None,
      );
      item.contextValue = "aranduProjectSelector";
      item.description = entry.project?.description ?? "Required";
      item.tooltip = entry.project?.root.fsPath ?? "Choose which Arandu project this workspace uses.";
      item.iconPath = new vscode.ThemeIcon("root-folder");
      item.command = {
        command: "arandu.project.select",
        title: "Select Arandu Project",
      };
      return item;
    }
    if (entry.type === "group") {
      const nodeCount = this.groupNodeIDs(entry.id).length;
      const item = new vscode.TreeItem(
        entry.label,
        nodeCount === 0 ? vscode.TreeItemCollapsibleState.None : vscode.TreeItemCollapsibleState.Collapsed,
      );
      item.contextValue = "aranduProjectMapGroup";
      item.description = nodeCount === 0 ? "0" : String(nodeCount);
      item.iconPath = new vscode.ThemeIcon(groupIcon(entry.id));
      return item;
    }
    if (entry.type === "relations") {
      const item = new vscode.TreeItem(entry.group.kind, vscode.TreeItemCollapsibleState.Collapsed);
      item.contextValue = `aranduProjectMapRelations.${entry.group.kind}`;
      item.description = String(entry.group.relations.length);
      item.tooltip = entry.group.meaning ?? entry.group.kind;
      item.iconPath = new vscode.ThemeIcon("references");
      return item;
    }
    if (entry.type === "relation") {
      const { relation } = entry;
      const other = presentNode(relation.other);
      const item = new vscode.TreeItem(other.label, vscode.TreeItemCollapsibleState.None);
      item.contextValue = `aranduProjectMapRelation.${relation.edge.kind}`;
      item.description = relation.other.kind;
      const arrow = relation.direction === "outgoing" ? "→" : "←";
      item.tooltip = `${relation.edge.kind} ${arrow} ${other.label}`;
      item.iconPath = new vscode.ThemeIcon(relation.direction === "outgoing" ? "arrow-right" : "arrow-left");
      const location = relationLocation(relation);
      if (location !== undefined) {
        item.command = openCommand(location);
      }
      return item;
    }

    const presentation = presentNode(entry.node);
    const item = new vscode.TreeItem(
      presentation.label,
      this.nodeChildren(entry.node).length === 0
        ? vscode.TreeItemCollapsibleState.None
        : vscode.TreeItemCollapsibleState.Collapsed,
    );
    item.contextValue = `aranduProjectMapNode.${entry.node.kind}`;
    item.description = presentation.description;
    item.tooltip = presentation.tooltip;
    item.iconPath = nodeIcon(entry.node);
    const location = nodeLocation(entry.node);
    if (location !== undefined) {
      item.resourceUri = vscode.Uri.parse(location.file, true);
      item.command = openCommand(location);
    }
    return item;
  }

  public getChildren(entry?: ProjectMapEntry): ProjectMapEntry[] {
    if (entry === undefined) {
      // The groups are the server's, in its order. Until a map has arrived
      // the first schema's groups stand in, which is what an older aru
      // answers anyway.
      const groups = this.graph?.groups ?? contract.groups;
      return [
        { type: "project", project: this.project },
        ...groups.map((group) => ({ type: "group" as const, id: group.id, label: group.label })),
      ];
    }
    if (entry.type === "project" || entry.type === "relation") {
      return [];
    }
    if (entry.type === "group") {
      return this.groupNodeIDs(entry.id).flatMap((id) => {
        const node = this.model.nodes.get(id);
        return node === undefined ? [] : [{ type: "node" as const, node }];
      });
    }
    if (entry.type === "relations") {
      return entry.group.relations.map((relation) => ({ type: "relation" as const, relation }));
    }
    return this.nodeChildren(entry.node);
  }

  private nodeChildren(node: ProjectGraphNode): ProjectMapEntry[] {
    const contained = (this.model.children.get(node.id) ?? []).flatMap((id) => {
      const child = this.model.nodes.get(id);
      return child === undefined ? [] : [{ type: "node" as const, node: child }];
    });
    const relations = (this.model.relations.get(node.id) ?? []).map((group) => ({
      type: "relations" as const,
      node,
      group,
    }));
    return [...contained, ...relations];
  }

  private groupNodeIDs(groupID: string): readonly string[] {
    return this.graph?.groups.find((group) => group.id === groupID)?.nodeIds ?? [];
  }
}

function openCommand(location: ProjectGraphLocation): vscode.Command {
  const uri = vscode.Uri.parse(location.file, true);
  const selection = new vscode.Range(location.line, location.column, location.endLine, location.endColumn);
  return {
    command: "vscode.open",
    title: "Open Source",
    arguments: [uri, { preview: true, selection }],
  };
}

function groupIcon(groupID: string): string {
  const icons: Readonly<Record<string, string>> = {
    "application-features": "symbol-module",
    http: "globe",
    database: "database",
    views: "browser",
    async: "server-process",
    integrations: "plug",
    console: "terminal",
    tests: "beaker",
    "native-screens": "device-desktop",
    "native-capabilities": "verified-filled",
    "community-modules": "extensions",
    diagnostics: "issues",
  };
  return icons[groupID] ?? "folder";
}

function nodeIcon(node: ProjectGraphNode): vscode.ThemeIcon {
  if (node.level === "error") {
    return new vscode.ThemeIcon("error", new vscode.ThemeColor("problemsErrorIcon.foreground"));
  }
  if (node.level === "warning") {
    return new vscode.ThemeIcon("warning", new vscode.ThemeColor("problemsWarningIcon.foreground"));
  }
  if (node.generated === true) {
    return new vscode.ThemeIcon("gear");
  }
  return new vscode.ThemeIcon(node.file === undefined ? "symbol-object" : "go-to-file");
}
