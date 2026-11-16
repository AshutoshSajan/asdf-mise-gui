package services

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/settings"
)

func TestScanWorkspace(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	web := filepath.Join(root, "web")
	for _, dir := range []string{app, web} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missingDir := filepath.Join(root, "missing")

	open := func(dir string, remember bool) (ProjectView, error) {
		if remember {
			t.Fatalf("workspace scans must not touch the recent list")
		}
		view := ProjectView{
			Dir:           dir,
			Engine:        "mise",
			FilePath:      filepath.Join(dir, "mise.toml"),
			FileExists:    true,
			FileInProject: true,
			ConfigFormat:  "toml",
			Entries:       []asdf.ToolVersionEntry{{Tool: "node", Versions: []string{"22"}}},
			Current:       []asdf.CurrentTool{{Name: "node", Versions: []string{"22.0.0"}, Installed: true}},
			Missing:       []string{},
			Warnings:      []string{},
		}
		if dir == app {
			view.Missing = []string{"python"}
		}
		if dir == web {
			view.Missing = []string{"python", "ruby"}
		}
		return view, nil
	}

	view := scanWorkspace(settings.Workspace{
		ID:       "ws-1",
		Name:     "Test",
		Projects: []string{app, web, missingDir},
	}, open, "mise")

	if view.ID != "ws-1" || view.Name != "Test" || view.ScannedAt == "" {
		t.Fatalf("unexpected workspace identity: %+v", view)
	}
	if len(view.Projects) != 3 {
		t.Fatalf("got %d projects, want 3", len(view.Projects))
	}
	if view.PinnedCount != 2 {
		t.Fatalf("PinnedCount = %d, want 2", view.PinnedCount)
	}
	if view.MissingProjects != 2 {
		t.Fatalf("MissingProjects = %d, want 2", view.MissingProjects)
	}
	if want := []string{"python", "ruby"}; !reflect.DeepEqual(view.Missing, want) {
		t.Fatalf("Missing = %v, want %v", view.Missing, want)
	}
	if view.Projects[0].ConfigFile != filepath.Join(app, "mise.toml") {
		t.Fatalf("unexpected config file: %s", view.Projects[0].ConfigFile)
	}
	if view.Projects[2].Exists {
		t.Fatal("missing directory should not be marked as existing")
	}
	if len(view.Projects[2].Warnings) == 0 {
		t.Fatal("missing directory should carry a warning")
	}
}

func TestScanWorkspaceReportsOpenErrors(t *testing.T) {
	dir := t.TempDir()
	open := func(string, bool) (ProjectView, error) {
		return ProjectView{}, errors.New("engine failed")
	}
	view := scanWorkspace(settings.Workspace{
		ID:       "ws-1",
		Name:     "Test",
		Projects: []string{dir},
	}, open, "asdf")

	if len(view.Projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(view.Projects))
	}
	project := view.Projects[0]
	if !project.Exists {
		t.Fatal("existing directory should be marked as existing")
	}
	if len(project.Warnings) != 1 || project.Warnings[0] != "engine failed" {
		t.Fatalf("unexpected warnings: %+v", project.Warnings)
	}
	if project.Entries == nil || project.Current == nil || project.Missing == nil {
		t.Fatal("workspace project slices must not be nil")
	}
}
