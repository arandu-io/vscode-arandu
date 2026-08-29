package extension_test

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRepackingRemovesArchiveTimeFromTheVSIX(t *testing.T) {
	root := t.TempDir()
	firstRaw := filepath.Join(root, "first.raw.vsix")
	secondRaw := filepath.Join(root, "second.raw.vsix")
	first := filepath.Join(root, "first.vsix")
	second := filepath.Join(root, "second.vsix")
	files := map[string]string{
		"[Content_Types].xml":    "types",
		"extension/package.json": "manifest",
	}
	writeDatedVSIX(t, firstRaw, files, time.Date(2026, 8, 29, 18, 0, 0, 0, time.UTC))
	writeDatedVSIX(t, secondRaw, files, time.Date(2026, 8, 29, 19, 0, 0, 0, time.UTC))
	for _, pair := range [][2]string{{firstRaw, first}, {secondRaw, second}} {
		command := exec.Command("go", "run", "./cmd/vsix-repack", pair[0], pair[1])
		command.Dir = rootPath(t)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("repack VSIX: %v\n%s", err, output)
		}
	}
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("repacked VSIX still depends on source archive time")
	}
}

func writeDatedVSIX(t *testing.T, path string, files map[string]string, date time.Time) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(date)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
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
