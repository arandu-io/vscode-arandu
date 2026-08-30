package extension_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTheReleaseVersionComesFromTheExtensionManifest(t *testing.T) {
	command := exec.Command("go", "run", "./cmd/manifest-version", "package.json")
	command.Dir = rootPath(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("read manifest version: %v\n%s", err, output)
	}
	if got, want := strings.TrimSpace(string(output)), "0.1.1"; got != want {
		t.Fatalf("manifest version = %q, want %q", got, want)
	}
}
