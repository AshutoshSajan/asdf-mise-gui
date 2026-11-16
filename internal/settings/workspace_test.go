package settings

import (
	"fmt"
	"path/filepath"
	"sync"
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

func TestUpsertWorkspaceRejectsLimit(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark"},
	}
	for index := 0; index < MaxWorkspaces; index++ {
		workspace := Workspace{
			ID:       fmt.Sprintf("ws-%d", index),
			Name:     fmt.Sprintf("Workspace %d", index),
			Projects: []string{"/work/app"},
		}
		if _, err := store.UpsertWorkspace(workspace); err != nil {
			t.Fatalf("workspace %d: %v", index, err)
		}
	}

	extra := Workspace{ID: "ws-extra", Name: "Overflow", Projects: []string{"/work/app"}}
	if _, err := store.UpsertWorkspace(extra); err == nil {
		t.Fatal("expected the workspace cap to reject a new workspace")
	}
	if len(store.Workspaces()) != MaxWorkspaces {
		t.Fatalf("workspace count = %d, want %d", len(store.Workspaces()), MaxWorkspaces)
	}

	// Updating an existing workspace must still work at the cap.
	if _, err := store.UpsertWorkspace(Workspace{
		ID:       "ws-0",
		Name:     "Renamed",
		Projects: []string{"/work/app"},
	}); err != nil {
		t.Fatalf("update at cap: %v", err)
	}
}

func TestUpsertWorkspaceReportsUnpersistedRecord(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark"},
	}
	if _, err := store.UpsertWorkspace(Workspace{ID: "ws-empty", Name: "  ", Projects: []string{"/work/app"}}); err == nil {
		t.Fatal("expected an invalid workspace to fail instead of reporting success")
	}
	if len(store.Workspaces()) != 0 {
		t.Fatalf("invalid workspace was persisted: %+v", store.Workspaces())
	}
}

func TestStoreUpdatePreservesConcurrentChanges(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark"},
	}

	var wg sync.WaitGroup
	errs := make(chan error, MaxWorkspaces)
	for index := 0; index < MaxWorkspaces; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs <- store.Update(func(next *Settings) error {
				next.Workspaces = append(next.Workspaces, Workspace{
					ID:       fmt.Sprintf("ws-%d", index),
					Name:     fmt.Sprintf("Workspace %d", index),
					Projects: []string{"/work/app"},
				})
				return nil
			})
		}(index)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent update: %v", err)
		}
	}
	if got := len(store.Workspaces()); got != MaxWorkspaces {
		t.Fatalf("workspace count = %d, want %d", got, MaxWorkspaces)
	}
}

func TestStoreUpdateLeavesMemoryUnchangedWhenWriteFails(t *testing.T) {
	store := &Store{
		path: t.TempDir(), // a directory cannot be replaced by the settings file
		data: Settings{Engine: "auto", Theme: "dark"},
	}
	err := store.Update(func(next *Settings) error {
		next.Theme = "light"
		return nil
	})
	if err == nil {
		t.Fatal("expected the write to fail")
	}
	if got := store.Get().Theme; got != "dark" {
		t.Fatalf("in-memory theme changed after a failed write: %q", got)
	}
}

func TestAddRecentProjectPreservesWorkspaces(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{
			Engine:         "auto",
			Theme:          "dark",
			Workspaces:     []Workspace{{ID: "ws-1", Name: "One", Projects: []string{"/work/app"}}},
			RecentProjects: []string{"/work/web"},
		},
	}
	if err := store.AddRecentProject("/work/app"); err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if len(got.Workspaces) != 1 || got.Workspaces[0].ID != "ws-1" {
		t.Fatalf("recent project update dropped workspaces: %+v", got.Workspaces)
	}
	if len(got.RecentProjects) != 2 || got.RecentProjects[0] != "/work/app" {
		t.Fatalf("unexpected recent projects: %+v", got.RecentProjects)
	}
}
