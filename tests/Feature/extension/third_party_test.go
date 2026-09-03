package extension_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// bundledModule is the comment esbuild writes above every module it inlines:
// `// node_modules/<package>/<file>`. Reading the package names out of the
// shipped bundle is the point -- a list typed into a test would agree with
// itself forever while the bundle moved on.
var bundledModule = regexp.MustCompile(`(?m)^// node_modules/(@[^/\s]+/[^/\s]+|[^@/\s][^/\s]*)/`)

// creditedSection matches the heading of one dependency entry in the notice:
// `## <package> -- <artifact it is bundled into>`. The trailing separator is
// what distinguishes a dependency entry from the closing section about Arandu's
// own files.
var creditedSection = regexp.MustCompile(`^([^\s]+) — `)

// licenseAnchors are the sentences a license text is not complete without. A
// notice that names a license and stops satisfies none of them: MIT and ISC
// both require their permission and notice text to travel with the copy, and
// Blue Oak requires the text or a link to it.
var licenseAnchors = map[string][]string{
	"MIT": {
		"Permission is hereby granted, free of charge",
		"The above copyright notice and this permission notice shall be included",
	},
	"ISC": {
		"Permission to use, copy, modify, and/or distribute this software",
	},
	"BlueOak-1.0.0": {
		"also gets the text of this license or a link to",
		"https://blueoakcouncil.org/license/1.0.0",
	},
}

// entry is one dependency's record in THIRD_PARTY.md.
type entry struct {
	version string
	license string
	body    string
}

// TestEveryBundledDependencyIsCredited is the guard that keeps the notice from
// rotting.
//
// The extension ships as a single bundled file with no node_modules beside it,
// so everyone who installs the VSIX receives these packages from us. A
// dependency added upstream arrives in the bundle as a one-line lockfile change
// nobody reviews as a legal one, and minimatch's Blue Oak license ends 30 days
// after somebody writes to say the notice is missing.
func TestEveryBundledDependencyIsCredited(t *testing.T) {
	credited := creditedPackages(t)
	resolved := resolvedVersions(t)
	closure := productionClosure(t, resolved)
	bundled := bundledPackages(t)

	if len(closure) == 0 && len(bundled) == 0 {
		t.Fatal("no third-party package was found in either the bundle or the lockfile, so this test is proving nothing")
	}

	// The bundle is the truth about what ships, but it is a build output and
	// `make test` runs before `make bundle`. The lockfile's production closure
	// is derived, not typed, and is available without building -- so the guard
	// still has something to check on a fresh checkout. Neither list is
	// maintained by hand.
	for _, name := range bundled {
		if _, ok := closure[name]; !ok {
			t.Errorf("%s is bundled into dist/extension.js and is not in the lockfile's production closure: "+
				"the fallback list this test uses when the bundle is absent no longer covers what ships", name)
		}
	}

	for _, name := range union(closure, bundled) {
		if _, ok := credited[name]; !ok {
			t.Errorf("%s is redistributed inside every VSIX and THIRD_PARTY.md does not credit it: "+
				"add its version, author, home, license and the full license text", name)
		}
	}
}

// TestTheCreditedVersionsAreTheResolvedOnes: an upgrade that leaves the notice
// behind publishes the wrong license for the wrong version, which is the same
// failure as having no notice at all.
//
// The versions are read out of package-lock.json, which is what npm ci installs
// and what esbuild therefore bundles. There is no second list to keep in step.
func TestTheCreditedVersionsAreTheResolvedOnes(t *testing.T) {
	credited := creditedPackages(t)
	resolved := resolvedVersions(t)

	if len(credited) == 0 {
		t.Fatal("THIRD_PARTY.md credits no dependency, so this test is proving nothing")
	}

	for _, name := range sortedKeys(credited) {
		want, ok := resolved[name]
		if !ok {
			t.Errorf("THIRD_PARTY.md credits %s and package-lock.json does not resolve it: "+
				"either the entry is stale or the package name is misspelled", name)
			continue
		}
		if got := credited[name].version; got != want {
			t.Errorf("THIRD_PARTY.md credits %s %s and package-lock.json resolves %s: "+
				"update the entry, and confirm the license did not change with the version", name, got, want)
		}
	}
}

// TestTheLicenseTextsAreComplete: naming a license is not the obligation. Each
// of the three here asks for its own text to be handed on with the copy, and an
// entry that says "MIT" and stops does not satisfy any of them.
func TestTheLicenseTextsAreComplete(t *testing.T) {
	credited := creditedPackages(t)
	if len(credited) == 0 {
		t.Fatal("THIRD_PARTY.md credits no dependency, so this test is proving nothing")
	}

	covered := make(map[string]bool, len(licenseAnchors))
	for _, name := range sortedKeys(credited) {
		record := credited[name]
		kind := licenseKind(record.license)
		if kind == "" {
			t.Errorf("THIRD_PARTY.md gives %s the license %q, which this test cannot check: "+
				"add its required text to licenseAnchors before shipping a fourth license", name, record.license)
			continue
		}
		covered[kind] = true
		for _, anchor := range licenseAnchors[kind] {
			if !strings.Contains(record.body, anchor) {
				t.Errorf("the %s entry for %s is missing the %s text %q", record.license, name, kind, anchor)
			}
		}
	}

	// The three licenses are not interchangeable, and a notice that quietly
	// lost its ISC or Blue Oak entry would still pass a test that only ever
	// looked at MIT.
	for _, kind := range sortedKeys(licenseAnchors) {
		if !covered[kind] {
			t.Errorf("no entry in THIRD_PARTY.md is licensed %s: this test no longer proves that license is honoured", kind)
		}
	}
}

// TestTheNoticeTravelsInsideTheVSIX: a notice that stays in the repository
// satisfies nothing. Blue Oak requires the text to reach everyone who receives
// any part of the software, and what they receive is the archive.
func TestTheNoticeTravelsInsideTheVSIX(t *testing.T) {
	var manifest struct {
		Files []string `json:"files"`
	}
	readJSON(t, "package.json", &manifest)
	if !contains(manifest.Files, "THIRD_PARTY.md") {
		t.Error("package.json does not list THIRD_PARTY.md in files: the notice would not be packaged into the VSIX")
	}

	auditor, err := os.ReadFile(rootPath(t, "cmd", "vsix-audit", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(auditor), `"extension/THIRD_PARTY.md"`) {
		t.Error("the VSIX auditor's allowlist does not require extension/THIRD_PARTY.md: " +
			"a release that drops the notice would pass the audit")
	}
}

// creditedPackages parses THIRD_PARTY.md into one record per dependency entry.
func creditedPackages(t *testing.T) map[string]entry {
	t.Helper()
	raw, err := os.ReadFile(rootPath(t, "THIRD_PARTY.md"))
	if err != nil {
		t.Fatalf("THIRD_PARTY.md is the notice the bundled dependencies require: %v", err)
	}

	// Entries are separated by a rule, not by their heading: the Blue Oak
	// license text carries `##` headings of its own, and splitting on those
	// would cut minimatch's entry in half and leave its Notices clause outside
	// the section this test then declares complete.
	credited := make(map[string]entry)
	for _, section := range strings.Split(string(raw), "\n---\n") {
		section = strings.TrimSpace(section)
		heading, _, _ := strings.Cut(strings.TrimPrefix(section, "## "), "\n")
		match := creditedSection.FindStringSubmatch(strings.TrimSpace(heading))
		if !strings.HasPrefix(section, "## ") || match == nil {
			continue
		}
		name := match[1]
		if _, duplicate := credited[name]; duplicate {
			t.Errorf("THIRD_PARTY.md has more than one entry for %s", name)
		}
		credited[name] = entry{
			version: tableValue(t, name, section, "Version"),
			license: tableValue(t, name, section, "License"),
			body:    section,
		}
	}
	return credited
}

// tableValue reads one row out of the two-column table an entry opens with.
func tableValue(t *testing.T, name, section, field string) string {
	t.Helper()
	row := regexp.MustCompile(`(?m)^\|\s*` + regexp.QuoteMeta(field) + `\s*\|\s*(.+?)\s*\|\s*$`)
	match := row.FindStringSubmatch(section)
	if match == nil {
		t.Errorf("the THIRD_PARTY.md entry for %s has no %s row", name, field)
		return ""
	}
	return match[1]
}

// bundledPackages reads the package names out of dist/extension.js. The bundle
// is a build output, so it is absent on a fresh checkout; that is not a failure
// here, because the lockfile closure covers the same ground.
func bundledPackages(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(rootPath(t, "dist", "extension.js"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}

	found := make(map[string]struct{})
	for _, match := range bundledModule.FindAllStringSubmatch(string(raw), -1) {
		found[match[1]] = struct{}{}
	}
	return sortedKeys(found)
}

// resolvedVersions is every package package-lock.json pins, by name.
func resolvedVersions(t *testing.T) map[string]string {
	t.Helper()
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	readJSON(t, "package-lock.json", &lock)

	resolved := make(map[string]string, len(lock.Packages))
	for path, pkg := range lock.Packages {
		name, ok := strings.CutPrefix(path, "node_modules/")
		if !ok || strings.Contains(name, "/node_modules/") {
			continue
		}
		resolved[name] = pkg.Version
	}
	return resolved
}

// productionClosure walks package-lock.json from the manifest's runtime
// dependencies. Development dependencies -- esbuild, vsce, typescript -- build
// the extension and are not redistributed with it, so they owe no notice.
func productionClosure(t *testing.T, resolved map[string]string) map[string]struct{} {
	t.Helper()
	var lock struct {
		Packages map[string]struct {
			Dependencies         map[string]string `json:"dependencies"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
		} `json:"packages"`
	}
	readJSON(t, "package-lock.json", &lock)

	closure := make(map[string]struct{})
	var walk func(name string)
	walk = func(name string) {
		if _, seen := closure[name]; seen {
			return
		}
		pkg, ok := lock.Packages["node_modules/"+name]
		if !ok {
			t.Errorf("package-lock.json reaches %s and does not pin it", name)
			return
		}
		closure[name] = struct{}{}
		for _, deps := range []map[string]string{pkg.Dependencies, pkg.OptionalDependencies} {
			for dependency := range deps {
				walk(dependency)
			}
		}
	}
	for dependency := range lock.Packages[""].Dependencies {
		walk(dependency)
	}
	return closure
}

// licenseKind maps the license named in an entry onto the anchors that prove
// its text is present.
func licenseKind(license string) string {
	switch {
	case strings.Contains(license, "BlueOak") || strings.Contains(license, "Blue Oak"):
		return "BlueOak-1.0.0"
	case strings.Contains(license, "ISC"):
		return "ISC"
	case strings.Contains(license, "MIT"):
		return "MIT"
	default:
		return ""
	}
}

func union(left map[string]struct{}, right []string) []string {
	merged := make(map[string]struct{}, len(left)+len(right))
	for name := range left {
		merged[name] = struct{}{}
	}
	for _, name := range right {
		merged[name] = struct{}{}
	}
	return sortedKeys(merged)
}

func sortedKeys[V any](values map[string]V) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
