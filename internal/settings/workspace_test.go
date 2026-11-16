package settings

import (
	"path/filepath"
	"testing"
)

func TestSanitizeWorkspaces(t *testing.T) {
	got := sanitizeWorkspaces([]Workspace{
		{ID: " ws-1 ", Name: " App ", Projects: []string{" /work/app ", "/work/app", "", " /work/web "}},
		{ID: "", Name: "No id", Projects: []string{"/ignored"}},
		{ID: "ws-1", Name: "Duplicate id", Projects: []string{"/ignored"}},
		{ID: "ws-2", Name: "   ", Projects: []string{"/ignored"}},
	})
	if len(got) != 1 {
		t.Fatalf("sanitizeWorkspaces() kept %d workspaces, want 1: %+v", len(got), got)
	}
	item := got[0]
	if item.ID != "ws-1" || item.Name != "App" {
		t.Fatalf("unexpected workspace identity: %+v", item)
	}
	if len(item.Projects) != 2 || item.Projects[0] != "/work/app" || item.Projects[1] != "/work/web" {
		t.Fatalf("unexpected workspace projects: %+v", item.Projects)
	}
}

func TestStoreUpsertAndDeleteWorkspace(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark", CheckForUpdates: true},
	}

	list, err := store.UpsertWorkspace(Workspace{ID: "ws-1", Name: "One", Projects: []string{"/work/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "One" {
		t.Fatalf("unexpected workspaces after insert: %+v", list)
	}

	list, err = store.UpsertWorkspace(Workspace{ID: "ws-1", Name: "Renamed", Projects: []string{"/work/app", "/work/web"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Renamed" || len(list[0].Projects) != 2 {
		t.Fatalf("unexpected workspaces after update: %+v", list)
	}

	list, err = store.DeleteWorkspace("ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected workspaces to be empty after delete: %+v", list)
	}
}

func TestGetCopiesWorkspaceProjects(t *testing.T) {
	store := &Store{
		data: Settings{Workspaces: []Workspace{{ID: "ws-1", Name: "One", Projects: []string{"/work/app"}}}},
	}
	got := store.Get()
	got.Workspaces[0].Projects[0] = "/changed"
	if store.Get().Workspaces[0].Projects[0] != "/work/app" {
		t.Fatal("Get returned a shared workspace project slice")
	}
}
