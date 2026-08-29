// Command repository-audit confines Node and TypeScript to the editor adapter.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var forbiddenNames = map[string]struct{}{
	".mypy_cache":         {},
	".pytest_cache":       {},
	".python-version":     {},
	"Pipfile":             {},
	"__pycache__":         {},
	"npm-shrinkwrap.json": {},
	"pnpm-lock.yaml":      {},
	"poetry.lock":         {},
	"pyproject.toml":      {},
	"pytest.ini":          {},
	"requirements.txt":    {},
	"tox.ini":             {},
	"yarn.lock":           {},
}

var forbiddenExtensions = map[string]struct{}{
	".cjs": {},
	".js":  {},
	".jsx": {},
	".mjs": {},
	".py":  {},
	".pyc": {},
	".ts":  {},
	".tsx": {},
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: repository-audit <repository>")
		os.Exit(2)
	}
	violations, err := audit(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(violations) != 0 {
		for _, violation := range violations {
			fmt.Fprintf(os.Stderr, "repository contains tooling outside the editor adapter: %s\n", violation)
		}
		os.Exit(1)
	}
	fmt.Println("Node and TypeScript are confined to the bundled editor adapter; Python is absent.")
}

func audit(root string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository: %w", err)
	}
	var violations []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() && (name == ".git" || name == "node_modules") {
			return filepath.SkipDir
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, forbidden := forbiddenNames[name]; forbidden {
			violations = append(violations, filepath.ToSlash(relative))
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(name, "vite.config.") {
			violations = append(violations, filepath.ToSlash(relative))
			return nil
		}
		extension := strings.ToLower(filepath.Ext(name))
		if _, forbidden := forbiddenExtensions[extension]; forbidden && !allowedAdapterFile(filepath.ToSlash(relative), extension) {
			violations = append(violations, filepath.ToSlash(relative))
		}
		if name == "package.json" && relative != "package.json" {
			violations = append(violations, filepath.ToSlash(relative))
		}
		if name == "package-lock.json" && relative != "package-lock.json" {
			violations = append(violations, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repository: %w", err)
	}
	sort.Strings(violations)
	return violations, nil
}

func allowedAdapterFile(relative, extension string) bool {
	if extension == ".ts" {
		return strings.HasPrefix(relative, "src/")
	}
	if extension == ".js" {
		return relative == "dist/extension.js"
	}
	return false
}
