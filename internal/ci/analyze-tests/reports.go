package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxExtractedArtifactFileSize = 100 << 20

// downloadAndExtract downloads one artifact zip and extracts safe relative file
// paths into dest.
func downloadAndExtract(ctx context.Context, gh *githubClient, a artifact, dest string) error {
	data, err := gh.downloadArtifact(ctx, a.ID)
	if err != nil {
		return err
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.UncompressedSize64 > maxExtractedArtifactFileSize {
			return fmt.Errorf("artifact file %q is too large: %d bytes", f.Name, f.UncompressedSize64)
		}
		if err := root.MkdirAll(filepath.Dir(f.Name), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := root.OpenFile(f.Name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		limited := io.LimitReader(rc, maxExtractedArtifactFileSize+1)
		n, copyErr := io.Copy(out, limited)
		if err := errors.Join(out.Close(), rc.Close(), copyErr); err != nil {
			return err
		}
		if n > maxExtractedArtifactFileSize {
			return fmt.Errorf("artifact file %q exceeds %d bytes", f.Name, maxExtractedArtifactFileSize)
		}
	}
	return nil
}

// parseReports reads go test JSON report files from an extracted artifact.
// Only package-level test pass and fail events are retained.
func parseReports(dir string) ([]testEvent, error) {
	var events []testEvent
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "-go-test-report.json") {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		file, err := root.Open(rel)
		if err != nil {
			return err
		}
		parsed, err := parseReportFile(file)
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		events = append(events, parsed...)
		return nil
	})
	return events, err
}

// parseReportFile decodes the newline-delimited go test JSON events in file.
func parseReportFile(file io.Reader) ([]testEvent, error) {
	var events []testEvent
	dec := json.NewDecoder(file)
	for {
		var rec struct {
			Action  string `json:"Action"`
			Package string `json:"Package"`
			Test    string `json:"Test"`
		}
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if (rec.Action == "pass" || rec.Action == "fail") && rec.Test != "" {
			if rec.Package == "" {
				rec.Package = "unknown"
			}
			events = append(events, testEvent{Action: rec.Action, Package: rec.Package, Test: rec.Test})
		}
	}
	return events, nil
}
