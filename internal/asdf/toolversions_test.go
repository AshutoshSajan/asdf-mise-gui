package asdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadToolVersions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ToolVersionsFilename)
	content := "# pinned by hand\n\nnodejs 22.0.0 20.11.0\nruby 3.2.2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := ReadToolVersions(path)
	if err != nil {
		t.Fatal(err)
	}
	if !file.Exists {
		t.Fatal("expected exists")
	}
	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %+v", file.Entries)
	}
	if file.Entries[0].Tool != "nodejs" || strings.Join(file.Entries[0].Versions, ",") != "22.0.0,20.11.0" {
		t.Fatalf("unexpected first entry: %+v", file.Entries[0])
	}
	if file.Entries[1].Tool != "ruby" {
		t.Fatalf("unexpected second entry: %+v", file.Entries[1])
	}
}

func TestReadToolVersionsMissing(t *testing.T) {
	file, err := ReadToolVersions(filepath.Join(t.TempDir(), ToolVersionsFilename))
	if err != nil {
		t.Fatal(err)
	}
	if file.Exists {
		t.Fatal("expected not exists")
	}
}

func TestToolVersionsSetPreservesComments(t *testing.T) {
	file := &ToolVersionsFile{
		EOL:        "\n",
		HasNewline: true,
		Lines:      []string{"# keep me", "", "nodejs 20.0.0", "ruby 3.2.2"},
	}
	file.reindex()
	file.Set("nodejs", []string{"22.0.0"})
	file.Set("python", []string{"3.12.0"})
	if file.Lines[0] != "# keep me" {
		t.Fatalf("comment lost: %v", file.Lines)
	}
	if file.Lines[2] != "nodejs 22.0.0" {
		t.Fatalf("nodejs not replaced: %v", file.Lines)
	}
	if file.Lines[len(file.Lines)-1] != "python 3.12.0" {
		t.Fatalf("python not appended: %v", file.Lines)
	}
}

func TestToolVersionsRemove(t *testing.T) {
	file := &ToolVersionsFile{
		EOL:   "\n",
		Lines: []string{"# keep", "nodejs 20.0.0", "ruby 3.2.2"},
	}
	file.reindex()
	file.Remove("nodejs")
	if strings.Join(file.Lines, "\n") != "# keep\nruby 3.2.2" {
		t.Fatalf("unexpected lines: %v", file.Lines)
	}
}

func TestToolVersionsReplaceEntries(t *testing.T) {
	file := &ToolVersionsFile{
		EOL:   "\n",
		Lines: []string{"# head", "nodejs 20.0.0", "", "ruby 3.2.2", "# tail"},
	}
	file.reindex()
	file.ReplaceEntries([]ToolVersionEntry{
		{Tool: "nodejs", Versions: []string{"22.0.0"}},
		{Tool: "python", Versions: []string{"3.12.0"}},
	})
	got := strings.Join(file.Lines, "\n")
	want := "# head\nnodejs 22.0.0\n\n# tail\npython 3.12.0"
	if got != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", got, want)
	}
}

func TestToolVersionsWriteCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ToolVersionsFilename)
	if err := os.WriteFile(path, []byte("nodejs 20.0.0\r\nruby 3.2.2\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := ReadToolVersions(path)
	if err != nil {
		t.Fatal(err)
	}
	file.Set("nodejs", []string{"22.0.0"})
	if err := file.Write(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "nodejs 22.0.0\r\nruby 3.2.2\r\n" {
		t.Fatalf("unexpected content: %q", string(raw))
	}
}

func TestWriteFileAtomicPreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected mode: %o", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "new" {
		t.Fatalf("unexpected content: %q", raw)
	}
}

func TestFindToolVersions(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ToolVersionsFilename)
	if err := os.WriteFile(path, []byte("nodejs 22.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, ok := FindToolVersions(child)
	if !ok || found != path {
		t.Fatalf("expected %s, got %s (%v)", path, found, ok)
	}
}

func TestParseVersionLine(t *testing.T) {
	if got := parseVersionLine("asdf version v0.20.2 (revision cb72590)"); got != "v0.20.2" {
		t.Fatalf("unexpected version: %s", got)
	}
}
