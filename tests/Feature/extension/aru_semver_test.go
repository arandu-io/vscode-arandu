package extension_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTheExtensionComparesStableAruVersionsAsSemanticVersions(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "semver.cjs")
	build := exec.Command(
		filepath.Join(rootPath(t), "node_modules", ".bin", "esbuild"),
		"src/semver.ts",
		"--bundle",
		"--platform=node",
		"--format=cjs",
		"--outfile="+bundle,
	)
	build.Dir = rootPath(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("bundle semantic version seam: %v\n%s", err, output)
	}

	check := exec.Command("node", "-e", `
const semver = require(process.env.ARANDU_SEMVER_MODULE);
const comparisons = [
  ["aru 0.9.9", "v0.10.0", true],
  ["aru v1.2.3+brew.1", "1.2.4", true],
  ["aru 1.2.3", "v1.2.3", false],
  ["aru 2.0.0", "v1.99.99", false],
  ["aru 999999999999999999.0.0", "v1000000000000000000.0.0", true],
];
for (const [installed, latest, want] of comparisons) {
  const got = semver.isNewerStableVersion(latest, installed);
  if (got !== want) {
    throw new Error(latest + " newer than " + installed + " = " + got + ", want " + want);
  }
}
for (const invalid of ["aru HEAD", "aru dev", "aru 1.2.3-dev", "aru 01.2.3", "aru 1.2"]) {
  if (semver.parseStableVersion(invalid) !== undefined) {
    throw new Error("accepted non-stable version " + invalid);
  }
}
`)
	check.Env = append(os.Environ(), "ARANDU_SEMVER_MODULE="+bundle)
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("semantic version contract: %v\n%s", err, output)
	}
}
