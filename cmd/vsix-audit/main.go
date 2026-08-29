// Command vsix-audit verifies the exact files shipped in an Arandu VSIX.
package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

var releaseFiles = map[string]struct{}{
	"[Content_Types].xml":                     {},
	"extension.vsixmanifest":                  {},
	"extension/LICENSE.md":                    {},
	"extension/changelog.md":                  {},
	"extension/images/icon.png":               {},
	"extension/language-configuration.json":   {},
	"extension/package.json":                  {},
	"extension/readme.md":                     {},
	"extension/snippets/kyse.json":            {},
	"extension/syntaxes/kyse.tmLanguage.json": {},
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: vsix-audit <extension.vsix>")
		os.Exit(2)
	}
	count, err := audit(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("VSIX contents match the release allowlist: %d files\n", count)
}

func audit(path string) (int, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return 0, fmt.Errorf("open VSIX: %w", err)
	}
	defer archive.Close()

	seen := make(map[string]struct{}, len(archive.File))
	for _, file := range archive.File {
		name := strings.TrimPrefix(file.Name, "/")
		if name != file.Name || strings.HasSuffix(name, "/") {
			return 0, fmt.Errorf("VSIX contains a non-file entry %q", file.Name)
		}
		if _, ok := releaseFiles[name]; !ok {
			return 0, fmt.Errorf("VSIX contains an unapproved file %q", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return 0, fmt.Errorf("VSIX contains %q more than once", name)
		}
		seen[name] = struct{}{}
	}

	var missing []string
	for name := range releaseFiles {
		if _, ok := seen[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return 0, errors.New("VSIX is missing approved files: " + strings.Join(missing, ", "))
	}
	return len(seen), nil
}
