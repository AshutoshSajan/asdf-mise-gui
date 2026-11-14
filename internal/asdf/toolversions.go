package asdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToolVersionEntry is one tool line in a .tool-versions file.
type ToolVersionEntry struct {
	Tool     string   `json:"tool"`
	Versions []string `json:"versions"`
	Line     int      `json:"line"`
}

// ToolVersionsFile is a comment-preserving view of a .tool-versions file.
type ToolVersionsFile struct {
	Path       string             `json:"path"`
	Exists     bool               `json:"exists"`
	Entries    []ToolVersionEntry `json:"entries"`
	Lines      []string           `json:"-"`
	EOL        string             `json:"-"`
	HasNewline bool               `json:"-"`
}

// ReadToolVersions reads a file. A missing file is not an error.
func ReadToolVersions(path string) (*ToolVersionsFile, error) {
	file := &ToolVersionsFile{Path: path, EOL: "\n"}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return file, nil
		}
		return nil, err
	}

	content := string(data)
	file.Exists = true
	if strings.Contains(content, "\r\n") {
		file.EOL = "\r\n"
		content = strings.ReplaceAll(content, "\r\n", "\n")
	}
	file.HasNewline = strings.HasSuffix(content, "\n")
	if file.HasNewline {
		content = strings.TrimSuffix(content, "\n")
	}
	if content == "" {
		return file, nil
	}
	file.Lines = strings.Split(content, "\n")
	file.reindex()
	return file, nil
}

func (f *ToolVersionsFile) reindex() {
	f.Entries = f.Entries[:0]
	for index, line := range f.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		f.Entries = append(f.Entries, ToolVersionEntry{
			Tool:     fields[0],
			Versions: fields[1:],
			Line:     index,
		})
	}
}

// Set replaces a tool's versions in place, preserving comments and ordering.
func (f *ToolVersionsFile) Set(tool string, versions []string) {
	if len(versions) == 0 {
		f.Remove(tool)
		return
	}
	replacement := tool + " " + strings.Join(versions, " ")

	for index, line := range f.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && fields[0] == tool {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			f.Lines[index] = indent + replacement
			f.reindex()
			return
		}
	}

	f.Lines = append(f.Lines, replacement)
	f.reindex()
}

// Remove deletes a tool's line.
func (f *ToolVersionsFile) Remove(tool string) {
	filtered := f.Lines[:0]
	for _, line := range f.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			fields := strings.Fields(trimmed)
			if len(fields) > 0 && fields[0] == tool {
				continue
			}
		}
		filtered = append(filtered, line)
	}
	f.Lines = filtered
	f.reindex()
}

// ReplaceEntries rewrites all tool entries while keeping comment and blank
// lines. Existing tools keep their position; new tools are appended.
func (f *ToolVersionsFile) ReplaceEntries(entries []ToolVersionEntry) {
	desired := make(map[string][]string, len(entries))
	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Tool == "" || len(entry.Versions) == 0 {
			continue
		}
		if _, ok := desired[entry.Tool]; !ok {
			order = append(order, entry.Tool)
		}
		desired[entry.Tool] = entry.Versions
	}

	var result []string
	written := make(map[string]bool, len(order))
	for _, line := range f.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			result = append(result, line)
			continue
		}
		versions, keep := desired[fields[0]]
		if !keep {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		result = append(result, indent+fields[0]+" "+strings.Join(versions, " "))
		written[fields[0]] = true
	}
	for _, tool := range order {
		if written[tool] {
			continue
		}
		result = append(result, tool+" "+strings.Join(desired[tool], " "))
	}
	f.Lines = result
	f.HasNewline = len(result) > 0
	f.reindex()
}

// Write persists the file atomically and preserves its permissions.
func (f *ToolVersionsFile) Write() error {
	if f.Path == "" {
		return fmt.Errorf("no path set")
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(f.Path); err == nil {
		mode = info.Mode().Perm()
	}
	content := strings.Join(f.Lines, f.EOL)
	if f.HasNewline && len(f.Lines) > 0 {
		content += f.EOL
	}
	return WriteFileAtomic(f.Path, []byte(content), mode)
}

// FindToolVersions walks up from start and returns the nearest file.
func FindToolVersions(start string) (string, bool) {
	dir := start
	for {
		path := filepath.Join(dir, ToolVersionsFilename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// WriteFileAtomic writes to a temp file in the same directory, then renames.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".asdf-gui-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
