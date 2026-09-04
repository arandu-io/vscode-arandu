package extension_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheReleaseVersionComesFromTheExtensionManifest: the release workflow reads
// the version out of package.json, and this proves the reader answers what the
// file says.
//
// It compares the command against the manifest rather than against a version
// written here. A literal would have to be edited on every release, and the one
// release where somebody edits the manifest and forgets this file is the release
// where a passing suite means nothing -- which is what happened: the manifest
// moved to 0.1.5 and this test still asked for 0.1.3, so it failed on a
// correct tree and went on failing until somebody read it.
//
// What the reader has to get right is the field, not the number.
func TestTheReleaseVersionComesFromTheExtensionManifest(t *testing.T) {
	root := rootPath(t)

	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse package.json: %v", err)
	}
	if manifest.Version == "" {
		t.Fatal("package.json declares no version, and the release workflow reads one from it")
	}

	command := exec.Command("go", "run", "./cmd/manifest-version", "package.json")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("read manifest version: %v\n%s", err, output)
	}

	if got := strings.TrimSpace(string(output)); got != manifest.Version {
		t.Fatalf("the reader answered %q and the manifest declares %q.\n"+
			"The release is tagged with what the reader says, so the two disagreeing is a release tagged with a version the extension does not carry.", got, manifest.Version)
	}
}
