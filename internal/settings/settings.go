package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings are user-controlled overrides. Empty values mean "auto-detect".
type Settings struct {
	Engine            string   `json:"engine"`
	BinaryPath        string   `json:"binaryPath"`
	DataDir           string   `json:"dataDir"`
	DefaultProjectDir string   `json:"defaultProjectDir"`
	Theme             string   `json:"theme"`
	CheckForUpdates   bool     `json:"checkForUpdates"`
	RecentProjects    []string `json:"recentProjects"`
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
	// Save updates the in-memory value before writing, so a failed migration
	// write still leaves the imported settings usable for this session.
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
	if value.RecentProjects == nil {
		value.RecentProjects = []string{}
	}
	if len(value.RecentProjects) > 12 {
		value.RecentProjects = value.RecentProjects[:12]
	}
	return value
}

func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := s.data
	copied.RecentProjects = append([]string(nil), s.data.RecentProjects...)
	return copied
}

func (s *Store) Path() string { return s.path }

func (s *Store) Save(next Settings) error {
	next = sanitize(next)
	s.mu.Lock()
	s.data = next
	s.mu.Unlock()

	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return writeAtomic(s.path, raw, 0o600)
}

// AddRecentProject moves dir to the front of the recent list.
func (s *Store) AddRecentProject(dir string) error {
	if dir == "" {
		return nil
	}
	s.mu.Lock()
	recent := []string{dir}
	for _, existing := range s.data.RecentProjects {
		if existing != dir {
			recent = append(recent, existing)
		}
	}
	if len(recent) > 12 {
		recent = recent[:12]
	}
	s.data.RecentProjects = recent
	next := s.data
	s.mu.Unlock()
	return s.Save(next)
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
