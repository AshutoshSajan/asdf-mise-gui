package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsService manages persisted overrides and discovery preferences.
type SettingsService struct {
	rt *core.Runtime
}

func NewSettingsService(rt *core.Runtime) *SettingsService {
	return &SettingsService{rt: rt}
}

type SettingsView struct {
	Settings     settings.Settings `json:"settings"`
	Detection    asdf.Detection    `json:"detection"`
	SettingsPath string            `json:"settingsPath"`
	LogsDir      string            `json:"logsDir"`
	ConfigDir    string            `json:"configDir"`
}

func (s *SettingsService) Get() SettingsView {
	logsDir, _ := settings.LogsDir()
	configDir, _ := settings.ConfigDir()
	return SettingsView{
		Settings:     s.rt.Settings(),
		Detection:    s.rt.Detection(),
		SettingsPath: s.rt.SettingsStore().Path(),
		LogsDir:      logsDir,
		ConfigDir:    configDir,
	}
}

func (s *SettingsService) Save(next settings.Settings) (SettingsView, error) {
	existing := s.rt.Settings()
	next.RecentProjects = existing.RecentProjects

	next.BinaryPath = strings.TrimSpace(expandHome(next.BinaryPath))
	next.DataDir = strings.TrimSpace(expandHome(next.DataDir))
	next.DefaultProjectDir = strings.TrimSpace(expandHome(next.DefaultProjectDir))

	switch next.Engine {
	case "auto", "asdf", "mise":
	case "":
		next.Engine = "auto"
	default:
		return s.Get(), fmt.Errorf("invalid engine %q: choose auto, asdf, or mise", next.Engine)
	}

	switch next.Theme {
	case "system", "light", "dark":
	default:
		return s.Get(), fmt.Errorf("invalid theme %q", next.Theme)
	}

	if next.BinaryPath != "" {
		abs, err := filepath.Abs(next.BinaryPath)
		if err != nil {
			return s.Get(), err
		}
		next.BinaryPath = abs
		info, err := os.Stat(abs)
		if err != nil {
			return s.Get(), fmt.Errorf("version manager binary not found: %s", abs)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return s.Get(), fmt.Errorf("%s is not an executable file", abs)
		}
		probe := asdf.DetectEngine(next.Engine, abs, next.DataDir)
		if !probe.Found {
			return s.Get(), fmt.Errorf("that binary is not a working version manager: %s", probe.Error)
		}
	}

	if next.DataDir != "" {
		abs, err := filepath.Abs(next.DataDir)
		if err != nil {
			return s.Get(), err
		}
		next.DataDir = abs
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return s.Get(), fmt.Errorf("data dir is not a directory: %s", abs)
		} else if err != nil {
			if parent, parentErr := os.Stat(filepath.Dir(abs)); parentErr != nil || !parent.IsDir() {
				return s.Get(), fmt.Errorf("parent of data dir does not exist: %s", filepath.Dir(abs))
			}
		}
	}

	if next.DefaultProjectDir != "" {
		abs, err := filepath.Abs(next.DefaultProjectDir)
		if err != nil {
			return s.Get(), err
		}
		next.DefaultProjectDir = abs
		if err := ensureDir(abs); err != nil {
			return s.Get(), fmt.Errorf("default project directory does not exist: %s", abs)
		}
	}

	if _, err := s.rt.SaveSettings(next); err != nil {
		return s.Get(), err
	}
	return s.Get(), nil
}

func (s *SettingsService) Reset() (SettingsView, error) {
	next := s.rt.Settings()
	next.Engine = "auto"
	next.BinaryPath = ""
	next.DataDir = ""
	next.DefaultProjectDir = ""
	if _, err := s.rt.SaveSettings(next); err != nil {
		return s.Get(), err
	}
	return s.Get(), nil
}

// PickBinary opens the native file picker for a version manager binary.
func (s *SettingsService) PickBinary() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return app.Dialog.OpenFile().
		SetTitle("Choose the asdf or mise binary").
		CanChooseDirectories(false).
		CanChooseFiles(true).
		PromptForSingleSelection()
}

// PickDataDir opens the native folder picker for the engine data directory.
func (s *SettingsService) PickDataDir() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return app.Dialog.OpenFile().
		SetTitle("Choose the version manager data directory").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}

// PickProjectDir opens the native folder picker for the default project.
func (s *SettingsService) PickProjectDir() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return app.Dialog.OpenFile().
		SetTitle("Choose the default project directory").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}
