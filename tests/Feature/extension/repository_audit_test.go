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

func TestTheRepositoryAuditorAllowsThePinnedEditorAdapter(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "package-lock.json", "{}\n")
	writeAuditFile(t, root, "src/extension.ts", "export function activate(): void {}\n")
	writeAuditFile(t, root, "dist/extension.js", "exports.activate = () => {};\n")
	command := exec.Command("go", "run", "./cmd/repository-audit", root)
	command.Dir = rootPath(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("repository audit rejected the confined editor adapter: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "no disallowed runtime is present") {
		t.Fatalf("repository audit success did not use the no-runtime contract:\n%s", output)
	}
}

func TestTheRepositoryAuditorRejectsTypeScriptOutsideTheEditorAdapter(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "scripts/build.ts", "export {};\n")
	command := exec.Command("go", "run", "./cmd/repository-audit", root)
	command.Dir = rootPath(t)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("repository audit accepted TypeScript outside src")
	}
	if !strings.Contains(string(output), "scripts/build.ts") {
		t.Fatalf("audit did not name the escaped TypeScript file:\n%s", output)
	}
}

func writeAuditFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestTheRepositoryIgnoresTheBinaryOfEveryCommand keeps a stray `go build` at
// the root out of a commit: it names the binary after the command directory,
// and two of them were once committed that way.
func TestTheRepositoryIgnoresTheBinaryOfEveryCommand(t *testing.T) {
	raw, err := os.ReadFile(rootPath(t, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	ignored := make(map[string]bool)
	for _, line := range strings.Split(string(raw), "\n") {
		ignored[strings.TrimSpace(line)] = true
	}
	commands, err := os.ReadDir(rootPath(t, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		if !command.IsDir() {
			continue
		}
		// Anchored to the root: an unanchored name would also ignore the
		// command's own source directory under cmd/.
		if !ignored["/"+command.Name()] {
			t.Errorf(".gitignore does not ignore /%s, the binary go build writes for cmd/%s", command.Name(), command.Name())
		}
	}
}
