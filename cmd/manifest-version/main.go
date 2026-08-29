// Command manifest-version prints the release version owned by package.json.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: manifest-version <package.json>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read extension manifest: %v\n", err)
		os.Exit(1)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "decode extension manifest: %v\n", err)
		os.Exit(1)
	}
	if manifest.Version == "" || strings.TrimSpace(manifest.Version) != manifest.Version {
		fmt.Fprintln(os.Stderr, "extension manifest has no valid version")
		os.Exit(1)
	}
	fmt.Println(manifest.Version)
}
