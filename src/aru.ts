import { constants } from "node:fs";
import { access, stat } from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";
import adapterContract from "./adapterContract.json";
import updateContract from "./updateContract.json";

export interface AruResolution {
  readonly executable: string;
  readonly source: string;
}

export async function resolveAruExecutable(folder: vscode.WorkspaceFolder): Promise<AruResolution> {
  const configured = vscode.workspace
    .getConfiguration("arandu", folder.uri)
    .get<string>("aru.path", "")
    .trim();
  if (configured !== "") {
    if (!path.isAbsolute(configured)) {
      throw new Error("arandu.aru.path must be an absolute path.");
    }
    if (!(await isExecutable(configured))) {
      throw new Error(`The configured Aru executable is not executable: ${configured}`);
    }
    return { executable: configured, source: adapterContract.aruPathOrder[0] };
  }

  const candidates: Array<{ executable: string; source: string }> = [];
  for (const directory of (process.env.PATH ?? "").split(path.delimiter)) {
    if (directory !== "") {
      candidates.push({ executable: path.join(directory, executableName()), source: adapterContract.aruPathOrder[1] });
    }
  }
  candidates.push(
    { executable: adapterContract.aruPathOrder[2], source: "Homebrew Apple Silicon" },
    { executable: adapterContract.aruPathOrder[3], source: "Homebrew Intel" },
  );

  const seen = new Set<string>();
  for (const candidate of candidates) {
    if (!seen.has(candidate.executable) && (await isExecutable(candidate.executable))) {
      return candidate;
    }
    seen.add(candidate.executable);
  }
  throw new Error(
    "Aru was not found in arandu.aru.path, PATH, /opt/homebrew/bin/aru, or /usr/local/bin/aru. Install it with Homebrew or configure its absolute path.",
  );
}

export async function resolveHomebrewExecutable(): Promise<AruResolution> {
  const candidates: AruResolution[] = [];
  for (const directory of (process.env.PATH ?? "").split(path.delimiter)) {
    if (directory !== "") {
      candidates.push({ executable: path.join(directory, "brew"), source: updateContract.brewPathOrder[0] });
    }
  }
  for (const executable of updateContract.brewPathOrder.slice(1)) {
    candidates.push({ executable, source: executable });
  }

  const seen = new Set<string>();
  for (const candidate of candidates) {
    if (!seen.has(candidate.executable) && (await isExecutable(candidate.executable))) {
      return candidate;
    }
    seen.add(candidate.executable);
  }
  throw new Error("Homebrew was not found in PATH, /opt/homebrew/bin/brew, or /usr/local/bin/brew.");
}

async function isExecutable(candidate: string): Promise<boolean> {
  try {
    await access(candidate, constants.X_OK);
    return (await stat(candidate)).isFile();
  } catch {
    return false;
  }
}

function executableName(): string {
  return process.platform === "win32" ? "aru.exe" : "aru";
}
