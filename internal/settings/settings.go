package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Settings are user-controlled overrides. Empty values mean "auto-detect".
type Settings struct {
	Engine            string      `json:"engine"`
	BinaryPath        string      `json:"binaryPath"`
	DataDir           string      `json:"dataDir"`
	DefaultProjectDir string      `json:"defaultProjectDir"`
	Theme             string      `json:"theme"`
	CheckForUpdates   bool        `json:"checkForUpdates"`
	RecentProjects    []string    `json:"recentProjects"`
	Workspaces        []Workspace `json:"workspaces"`
	RunnerMode        string      `json:"runnerMode"`
	WSLDistro         string      `json:"wslDistro"`
}

// Workspace groups project directories so the app can report on several
// projects without opening them one at a time.
type Workspace struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"`
}

const (
	appName       = "asdf-mise-gui"
	legacyAppName = "asdf-gui"
)

// Store persists settings as JSON in the platform config directory.
type Store struct {
	mu   sync.RWMutex
	path string
	data Settings
}

func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

func LogsDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName, "logs"), nil
}

func legacySettingsPath() (string, bool) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(dir, legacyAppName, "settings.json"), true
}

// Load reads settings, creating defaults when no file exists.
func Load() (*Store, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	store := &Store{
		path: filepath.Join(dir, "settings.json"),
		data: Settings{Theme: "dark", CheckForUpdates: true},
	}
	raw, err := os.ReadFile(store.path)
	if err != nil {
		if os.IsNotExist(err) {
			if legacyPath, ok := legacySettingsPath(); ok {
				migrateSettings(store, legacyPath)
			}
			return store, nil
		}
		return nil, err
	}
	if err := decodeSettings(raw, &store.data); err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	store.data = sanitize(store.data)
	return store, nil
}

// decodeSettings preserves the opt-out default for update checks. Settings
// written before the updater existed have no checkForUpdates key, and a plain
// bool would decode those as false.
func decodeSettings(raw []byte, value *Settings) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return err
	}
	if field, ok := fields["checkForUpdates"]; !ok || string(field) == "null" {
		value.CheckForUpdates = true
	}
	return nil
}

// migrateSettings imports a pre-rename settings file the first time the new
// config location is used. The legacy file is left in place so downgrading
// still finds its configuration.
func migrateSettings(store *Store, legacyPath string) bool {
	raw, err := os.ReadFile(legacyPath)
	if err != nil {
		return false
	}
	var legacy Settings
	if err := decodeSettings(raw, &legacy); err != nil {
		return false
	}
	// A failed migration write leaves the in-memory defaults in place; the
	// legacy file is untouched, so the next start can retry the migration.
	_ = store.Save(legacy)
	return true
}

func sanitize(value Settings) Settings {
	switch value.Engine {
	case "asdf", "mise":
	default:
		value.Engine = "auto"
	}
	switch value.Theme {
	case "system", "light", "dark":
	default:
		value.Theme = "dark"
	}
	switch value.RunnerMode {
	case "native", "wsl":
	default:
		value.RunnerMode = "auto"
	}
	value.WSLDistro = strings.TrimSpace(value.WSLDistro)
	if value.RecentProjects == nil {
		value.RecentProjects = []string{}
	}
	if len(value.RecentProjects) > 12 {
		value.RecentProjects = value.RecentProjects[:12]
	}
	value.Workspaces = sanitizeWorkspaces(value.Workspaces)
	return value
}

// Workspace limits keep the settings file and scan results bounded. They are
// exported so the service validates the same limits the store enforces.
const (
	MaxWorkspaces        = 20
	MaxWorkspaceProjects = 40
)

func sanitizeWorkspaces(items []Workspace) []Workspace {
	out := make([]Workspace, 0, len(items))
	seenIDs := map[string]bool{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		if id == "" || name == "" || seenIDs[id] {
			continue
		}
		projects := make([]string, 0, len(item.Projects))
		seenPaths := map[string]bool{}
		for _, project := range item.Projects {
			project = strings.TrimSpace(project)
			if project == "" || seenPaths[project] {
				continue
			}
			seenPaths[project] = true
			projects = append(projects, project)
			if len(projects) == MaxWorkspaceProjects {
				break
			}
		}
		seenIDs[id] = true
		out = append(out, Workspace{ID: id, Name: name, Projects: projects})
		if len(out) == MaxWorkspaces {
			break
		}
	}
	return out
}

func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSettings(s.data)
}

func cloneSettings(value Settings) Settings {
	value.RecentProjects = append([]string(nil), value.RecentProjects...)
	value.Workspaces = cloneWorkspaces(value.Workspaces)
	return value
}

func cloneWorkspaces(items []Workspace) []Workspace {
	out := make([]Workspace, 0, len(items))
	for _, item := range items {
		copied := item
		copied.Projects = append([]string(nil), item.Projects...)
		out = append(out, copied)
	}
	return out
}

func (s *Store) Path() string { return s.path }

// Save replaces all settings. Callers that change part of the settings should
// use Update so concurrent changes are not overwritten.
func (s *Store) Save(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(next)
}

// Update applies mutate to a copy of the current settings and persists the
// result in one transaction. A failed write leaves both the in-memory and
// on-disk settings unchanged. The callback must not call back into the store.
func (s *Store) Update(mutate func(*Settings) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneSettings(s.data)
	if err := mutate(&next); err != nil {
		return err
	}
	return s.saveLocked(next)
}

// saveLocked writes settings and only publishes them to readers after the
// write succeeds. The caller must hold s.mu.
func (s *Store) saveLocked(next Settings) error {
	next = sanitize(next)
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := writeAtomic(s.path, raw, 0o600); err != nil {
		return err
	}
	s.data = next
	return nil
}

// AddRecentProject moves dir to the front of the recent list.
func (s *Store) AddRecentProject(dir string) error {
	if dir == "" {
		return nil
	}
	return s.Update(func(next *Settings) error {
		recent := []string{dir}
		for _, existing := range next.RecentProjects {
			if existing != dir {
				recent = append(recent, existing)
			}
		}
		if len(recent) > 12 {
			recent = recent[:12]
		}
		next.RecentProjects = recent
		return nil
	})
}

func (s *Store) Workspaces() []Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWorkspaces(s.data.Workspaces)
}

func (s *Store) Workspace(id string) (Workspace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.data.Workspaces {
		if item.ID == id {
			copied := item
			copied.Projects = append([]string(nil), item.Projects...)
			return copied, true
		}
	}
	return Workspace{}, false
}

// UpsertWorkspace adds or replaces a workspace and returns the persisted list.
// Adding a workspace when the store is full fails instead of silently dropping
// the new entry during sanitization.
func (s *Store) UpsertWorkspace(next Workspace) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := cloneSettings(s.data)
	replaced := false
	for index := range data.Workspaces {
		if data.Workspaces[index].ID == next.ID {
			data.Workspaces[index] = next
			replaced = true
			break
		}
	}
	if !replaced {
		if len(data.Workspaces) >= MaxWorkspaces {
			return nil, fmt.Errorf("workspace limit reached (%d); delete one first", MaxWorkspaces)
		}
		data.Workspaces = append(data.Workspaces, next)
	}
	if err := s.saveLocked(data); err != nil {
		return nil, err
	}
	persisted := cloneWorkspaces(s.data.Workspaces)
	for _, item := range persisted {
		if item.ID == next.ID {
			return persisted, nil
		}
	}
	return nil, fmt.Errorf("workspace %q was not persisted", next.ID)
}

func (s *Store) DeleteWorkspace(id string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := cloneWorkspaces(s.data.Workspaces)
	kept := make([]Workspace, 0, len(current))
	for _, item := range current {
		if item.ID != id {
			kept = append(kept, item)
		}
	}
	if len(kept) == len(current) {
		return cloneWorkspaces(s.data.Workspaces), nil
	}
	data := cloneSettings(s.data)
	data.Workspaces = kept
	if err := s.saveLocked(data); err != nil {
		return nil, err
	}
	return cloneWorkspaces(s.data.Workspaces), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
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
	return os.Rename(name, path)
}
