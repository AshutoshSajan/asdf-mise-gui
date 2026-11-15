package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateSettings(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "asdf-gui", "settings.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"engine":"mise","binaryPath":"/opt/mise","theme":"light","recentProjects":["/work/app"]}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store := &Store{
		path: filepath.Join(dir, appName, "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark"},
	}
	if !migrateSettings(store, legacyPath) {
		t.Fatal("expected migration to report success")
	}

	got := store.Get()
	if got.Engine != "mise" || got.BinaryPath != "/opt/mise" || got.Theme != "light" {
		t.Fatalf("unexpected migrated settings: %+v", got)
	}
	if len(got.RecentProjects) != 1 || got.RecentProjects[0] != "/work/app" {
		t.Fatalf("unexpected recent projects: %+v", got.RecentProjects)
	}
	if !got.CheckForUpdates {
		t.Fatal("legacy settings should enable update checks by default")
	}

	raw, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("expected migrated settings file: %v", err)
	}
	var persisted Settings
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("migrated file is not valid JSON: %v", err)
	}
	if persisted.Engine != "mise" || persisted.Theme != "light" {
		t.Fatalf("unexpected persisted settings: %+v", persisted)
	}

	// The legacy file must survive so a downgrade still starts with its config.
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy settings file was removed: %v", err)
	}
}

func TestDecodeSettingsDefaultsUpdateChecks(t *testing.T) {
	var got Settings
	if err := decodeSettings([]byte(`{"engine":"mise","theme":"light"}`), &got); err != nil {
		t.Fatal(err)
	}
	if !got.CheckForUpdates {
		t.Fatal("settings without checkForUpdates should enable update checks")
	}
}

func TestDecodeSettingsPreservesUpdateOptOut(t *testing.T) {
	var got Settings
	if err := decodeSettings([]byte(`{"engine":"asdf","checkForUpdates":false}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.CheckForUpdates {
		t.Fatal("an explicit checkForUpdates:false must be preserved")
	}
}

func TestDecodeSettingsNullUpdateChecksDefaultsToTrue(t *testing.T) {
	var got Settings
	if err := decodeSettings([]byte(`{"checkForUpdates":null}`), &got); err != nil {
		t.Fatal(err)
	}
	if !got.CheckForUpdates {
		t.Fatal("null checkForUpdates should fall back to enabled")
	}
}

func TestMigrateSettingsMissingLegacyFile(t *testing.T) {
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "asdf", Theme: "dark"},
	}
	if migrateSettings(store, filepath.Join(t.TempDir(), "missing.json")) {
		t.Fatal("expected missing legacy file to be ignored")
	}
	if got := store.Get(); got.Engine != "asdf" || got.Theme != "dark" {
		t.Fatalf("settings changed without a legacy file: %+v", got)
	}
}

func TestMigrateSettingsInvalidLegacyFile(t *testing.T) {
	legacyPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(legacyPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{
		path: filepath.Join(t.TempDir(), "settings.json"),
		data: Settings{Engine: "auto", Theme: "dark"},
	}
	if migrateSettings(store, legacyPath) {
		t.Fatal("expected invalid legacy file to be ignored")
	}
	if got := store.Get(); got.Engine != "auto" || got.Theme != "dark" {
		t.Fatalf("settings changed from an invalid legacy file: %+v", got)
	}
}
