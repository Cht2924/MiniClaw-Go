package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"miniclaw-go/internal/core"
)

type FilePolicy struct {
	ProjectRoot  string
	AllowedRoots []string
}

func RegisterFileTools(reg *Registry, policy FilePolicy) {
	reg.Register(core.ToolDescriptor{
		Name:        "read_file",
		Description: "Read a UTF-8 text file within the allowed project roots.",
		Source:      "native",
		InputSchema: schema(
			prop("path", "string", "File path to read."),
			required("path"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		path, err := resolveAllowedPath(policy, input.Path, false)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return truncate(string(data), 8000), nil
	})

	reg.Register(core.ToolDescriptor{
		Name:        "write_file",
		Description: "Write a UTF-8 text file within the allowed project roots.",
		Source:      "native",
		InputSchema: schema(
			prop("path", "string", "File path to write."),
			prop("content", "string", "New file content."),
			required("path", "content"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		path, err := resolveAllowedPath(policy, input.Path, true)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(input.Content), path), nil
	})

	reg.Register(core.ToolDescriptor{
		Name:        "list_dir",
		Description: "List files and directories under an allowed path.",
		Source:      "native",
		InputSchema: schema(
			prop("path", "string", "Directory path to list."),
			required("path"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		path, err := resolveAllowedPath(policy, input.Path, false)
		if err != nil {
			return "", err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}
		var lines []string
		for _, entry := range entries {
			kind := "file"
			if entry.IsDir() {
				kind = "dir"
			}
			lines = append(lines, fmt.Sprintf("%s\t%s", kind, entry.Name()))
		}
		return strings.Join(lines, "\n"), nil
	})

	reg.Register(core.ToolDescriptor{
		Name:        "search_files",
		Description: "Search file names and file contents for a query under an allowed path.",
		Source:      "native",
		InputSchema: schema(
			prop("query", "string", "Text query to search for."),
			prop("path", "string", "Base directory for search."),
			required("query", "path"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		root, err := resolveAllowedPath(policy, input.Path, false)
		if err != nil {
			return "", err
		}
		input.Query = strings.ToLower(strings.TrimSpace(input.Query))
		if input.Query == "" {
			return "", fmt.Errorf("query is required")
		}

		var matches []string
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(policy.ProjectRoot, path)
			nameMatch := strings.Contains(strings.ToLower(d.Name()), input.Query)
			contentMatch := false
			if !nameMatch {
				data, readErr := os.ReadFile(path)
				if readErr == nil {
					contentMatch = strings.Contains(strings.ToLower(string(data)), input.Query)
				}
			}
			if nameMatch || contentMatch {
				matches = append(matches, filepath.ToSlash(rel))
			}
			if len(matches) >= 20 {
				return fs.SkipAll
			}
			return nil
		})
		if err != nil && err != fs.SkipAll {
			return "", err
		}
		if len(matches) == 0 {
			return "no matches found", nil
		}
		return strings.Join(matches, "\n"), nil
	})
}

func resolveAllowedPath(policy FilePolicy, input string, allowCreate bool) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("path is required")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(policy.ProjectRoot, path)
	}
	path = filepath.Clean(path)

	for _, root := range policy.AllowedRoots {
		root = filepath.Clean(root)
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			if !allowCreate {
				if _, err := os.Stat(path); err != nil {
					return "", err
				}
			}
			return path, nil
		}
	}

	return "", fmt.Errorf("path outside allowed roots: %s", input)
}
