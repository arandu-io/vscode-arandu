package extension_test

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheVSIXAuditorAcceptsOnlyThePublishedExtensionFiles(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "arandu.vsix")
	writeVSIX(t, archive, releaseVSIXFiles())
	command := exec.Command("go", "run", "./cmd/vsix-audit", archive)
	command.Dir = rootPath(t)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("audit a release-shaped VSIX: %v\n%s", err, output)
	}
}

func TestTheVSIXAuditorRejectsAnUnbundledNodeDependency(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "arandu.vsix")
	writeVSIX(t, archive, append(releaseVSIXFiles(), "extension/node_modules/vscode-languageclient/package.json"))
	command := exec.Command("go", "run", "./cmd/vsix-audit", archive)
	command.Dir = rootPath(t)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("VSIX auditor accepted an unbundled Node dependency")
	}
	if !strings.Contains(string(output), "extension/node_modules/vscode-languageclient/package.json") {
		t.Fatalf("VSIX auditor did not name the unbundled dependency:\n%s", output)
	}
}

func releaseVSIXFiles() []string {
	return []string{
		"[Content_Types].xml",
		"extension.vsixmanifest",
		"extension/package.json",
		"extension/readme.md",
		"extension/changelog.md",
		"extension/LICENSE.md",
		"extension/dist/extension.js",
		"extension/images/activity.svg",
		"extension/images/icon.png",
		"extension/images/logo.png",
		"extension/language-configuration.json",
		"extension/syntaxes/kyse.tmLanguage.json",
		"extension/snippets/kyse.json",
	}
}

func writeVSIX(t *testing.T, path string, names []string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, name := range names {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
