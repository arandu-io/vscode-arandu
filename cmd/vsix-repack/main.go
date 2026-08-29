// Command vsix-repack writes a VSIX with stable ordering and timestamps.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

var archiveEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: vsix-repack <input.vsix> <output.vsix>")
		os.Exit(2)
	}
	if err := repack(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func repack(input, output string) error {
	if input == output {
		return fmt.Errorf("input and output VSIX must be different files")
	}
	source, err := zip.OpenReader(input)
	if err != nil {
		return fmt.Errorf("open input VSIX: %w", err)
	}
	defer source.Close()

	files := append([]*zip.File(nil), source.File...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	destination, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output VSIX: %w", err)
	}
	archive := zip.NewWriter(destination)
	for _, file := range files {
		if err := copyFile(archive, file); err != nil {
			archive.Close()
			destination.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		destination.Close()
		return fmt.Errorf("finish output VSIX: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close output VSIX: %w", err)
	}
	return nil
}

func copyFile(destination *zip.Writer, source *zip.File) error {
	reader, err := source.Open()
	if err != nil {
		return fmt.Errorf("open %q: %w", source.Name, err)
	}
	defer reader.Close()

	header := &zip.FileHeader{Name: source.Name, Method: zip.Deflate}
	header.SetModTime(archiveEpoch)
	header.SetMode(0o644)
	writer, err := destination.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create %q: %w", source.Name, err)
	}
	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("copy %q: %w", source.Name, err)
	}
	return nil
}
