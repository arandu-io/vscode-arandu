import adapterContract from "./adapterContract.json";

// isRelevantProjectPath reports whether a change to the file at `relative`, a
// slash-separated path inside the selected project, can change what Doctor
// reports, and so whether the Project Map has to be refreshed.
//
// Doctor parses every Go file of the project, so any `.go` path counts, not
// only the ones under the directories the contract names. It skips the same
// directories Doctor's walk skips, at any depth, and the views `aru` writes
// under storage: refreshing on a build's own output would answer every view
// save twice.
export function isRelevantProjectPath(relative: string): boolean {
  if (relative === "" || relative === ".." || relative.startsWith("../") || relative.startsWith("/")) {
    return false;
  }
  const directories = relative.split("/").slice(0, -1);
  if (directories.some((name) => adapterContract.skippedDirectories.includes(name))) {
    return false;
  }
  if (adapterContract.skippedPaths.some((prefix) => relative.startsWith(prefix))) {
    return false;
  }
  if (relative.endsWith(adapterContract.relevantSuffix)) {
    return true;
  }
  return adapterContract.relevantPaths.some((candidate) => {
    if (candidate.endsWith("/")) {
      return relative.startsWith(candidate);
    }
    return relative === candidate || (candidate === "arandu.mod.toml" && relative.endsWith(`/${candidate}`));
  });
}
