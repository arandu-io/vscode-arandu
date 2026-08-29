import * as vscode from "vscode";
import contract from "./projectGraphContract.json";
import type { ProjectGraph, ProjectGraphNode } from "./projectGraphSchema";

interface GroupEntry {
  readonly type: "group";
  readonly id: string;
  readonly label: string;
}

interface NodeEntry {
  readonly type: "node";
  readonly node: ProjectGraphNode;
}

type ProjectMapEntry = GroupEntry | NodeEntry;

export class ProjectMapProvider implements vscode.TreeDataProvider<ProjectMapEntry> {
  private readonly changed = new vscode.EventEmitter<ProjectMapEntry | undefined | null | void>();
  private graph: ProjectGraph | undefined;
  private readonly nodes = new Map<string, ProjectGraphNode>();
  private readonly childIDs = new Map<string, readonly string[]>();

  public readonly onDidChangeTreeData = this.changed.event;

  public dispose(): void {
    this.changed.dispose();
  }

  public setGraph(graph: ProjectGraph | undefined): void {
    this.graph = graph;
    this.nodes.clear();
    this.childIDs.clear();
    if (graph !== undefined) {
      for (const node of graph.nodes) {
        this.nodes.set(node.id, node);
      }
      const mutableChildren = new Map<string, string[]>();
      for (const edge of graph.edges) {
        const children = mutableChildren.get(edge.from) ?? [];
        children.push(edge.to);
        mutableChildren.set(edge.from, children);
      }
      for (const [id, children] of mutableChildren) {
        this.childIDs.set(id, children);
      }
    }
    this.changed.fire();
  }

  public getTreeItem(entry: ProjectMapEntry): vscode.TreeItem {
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

    const childCount = this.childIDs.get(entry.node.id)?.length ?? 0;
    const item = new vscode.TreeItem(
      entry.node.label,
      childCount === 0 ? vscode.TreeItemCollapsibleState.None : vscode.TreeItemCollapsibleState.Collapsed,
    );
    item.contextValue = `aranduProjectMapNode.${entry.node.kind}`;
    item.description = entry.node.detail;
    item.tooltip = entry.node.detail === undefined
      ? entry.node.label
      : `${entry.node.label}\n${entry.node.detail}`;
    item.iconPath = nodeIcon(entry.node);
    if (entry.node.file !== undefined) {
      const uri = vscode.Uri.parse(entry.node.file, true);
      const position = new vscode.Position(entry.node.line ?? 0, entry.node.column ?? 0);
      item.resourceUri = uri;
      item.command = {
        command: "vscode.open",
        title: "Open Source",
        arguments: [uri, { preview: true, selection: new vscode.Range(position, position) }],
      };
    }
    return item;
  }

  public getChildren(entry?: ProjectMapEntry): ProjectMapEntry[] {
    if (entry === undefined) {
      return contract.groups.map((group) => ({ type: "group", id: group.id, label: group.label }));
    }
    if (entry.type === "group") {
      return this.groupNodeIDs(entry.id).flatMap((id) => {
        const node = this.nodes.get(id);
        return node === undefined ? [] : [{ type: "node" as const, node }];
      });
    }
    return (this.childIDs.get(entry.node.id) ?? []).flatMap((id) => {
      const node = this.nodes.get(id);
      return node === undefined ? [] : [{ type: "node" as const, node }];
    });
  }

  private groupNodeIDs(groupID: string): readonly string[] {
    return this.graph?.groups.find((group) => group.id === groupID)?.nodeIds ?? [];
  }
}

function groupIcon(groupID: string): string {
  const icons: Readonly<Record<string, string>> = {
    "application-features": "symbol-module",
    http: "globe",
    database: "database",
    views: "browser",
    async: "server-process",
    console: "terminal",
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
  return new vscode.ThemeIcon(node.file === undefined ? "symbol-object" : "go-to-file");
}
