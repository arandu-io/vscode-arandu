package extension_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheRepositoryAuditorRejectsPythonTooling(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "contract_test.py")
	if err := os.WriteFile(path, []byte("assert True\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "run", "./cmd/repository-audit", root)
	command.Dir = rootPath(t)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("repository audit accepted Python test tooling")
	}
	if !strings.Contains(string(output), "contract_test.py") {
		t.Fatalf("audit did not name the rejected file:\n%s", output)
	}
}
