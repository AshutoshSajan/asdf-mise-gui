package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-gui/internal/core"
	"github.com/AshutoshSajan/asdf-gui/internal/ops"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ProjectService reads and edits .tool-versions files.
type ProjectService struct {
	rt    *core.Runtime
	tools *ToolsService
}

func NewProjectService(rt *core.Runtime, tools *ToolsService) *ProjectService {
	return &ProjectService{rt: rt, tools: tools}
}

type RecentProject struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	Exists          bool   `json:"exists"`
	HasToolVersions bool   `json:"hasToolVersions"`
}

type ProjectView struct {
	Dir            string                  `json:"dir"`
	FilePath       string                  `json:"filePath"`
	FileExists     bool                    `json:"fileExists"`
	FileInProject  bool                    `json:"fileInProject"`
	UsingParent    bool                    `json:"usingParent"`
	Entries        []asdf.ToolVersionEntry `json:"entries"`
	GlobalFilePath string                  `json:"globalFilePath"`
	GlobalExists   bool                    `json:"globalExists"`
	GlobalEntries  []asdf.ToolVersionEntry `json:"globalEntries"`
	Current        []asdf.CurrentTool      `json:"current"`
	Missing        []string                `json:"missing"`
	Plugins        []string                `json:"plugins"`
	Recent         []RecentProject         `json:"recent"`
	Warnings       []string                `json:"warnings"`
	LoadedAt       string                  `json:"loadedAt"`
}

// Open resolves a directory, its nearest .tool-versions file, and asdf's
// current view in that directory. remember adds it to the recent list.
func (s *ProjectService) Open(dir string, remember bool) (ProjectView, error) {
	resolved, err := s.resolveDir(dir)
	if err != nil {
		return ProjectView{}, err
	}
	if remember {
		_ = s.rt.SettingsStore().AddRecentProject(resolved)
	}

	view := ProjectView{
		Dir:            resolved,
		GlobalFilePath: filepath.Join(homeDir(), asdf.ToolVersionsFilename),
		Entries:        []asdf.ToolVersionEntry{},
		GlobalEntries:  []asdf.ToolVersionEntry{},
		Current:        []asdf.CurrentTool{},
		Missing:        []string{},
		Plugins:        []string{},
		Warnings:       []string{},
		LoadedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	ownPath := filepath.Join(resolved, asdf.ToolVersionsFilename)
	ownFile, err := asdf.ReadToolVersions(ownPath)
	if err != nil {
		return ProjectView{}, err
	}
	effectivePath := ownPath
	effectiveFile := ownFile
	if ownFile.Exists {
		view.FileInProject = true
	} else if nearest, found := asdf.FindToolVersions(resolved); found {
		view.UsingParent = true
		effectivePath = nearest
		effectiveFile, err = asdf.ReadToolVersions(nearest)
		if err != nil {
			return ProjectView{}, err
		}
	}
	view.FilePath = effectivePath
	view.FileExists = effectiveFile.Exists
	view.Entries = effectiveFile.Entries
	if view.Entries == nil {
		view.Entries = []asdf.ToolVersionEntry{}
	}

	if global, err := asdf.ReadToolVersions(view.GlobalFilePath); err == nil {
		view.GlobalExists = global.Exists
		view.GlobalEntries = global.Entries
		if view.GlobalEntries == nil {
			view.GlobalEntries = []asdf.ToolVersionEntry{}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if result, runErr := s.rt.Run(ctx, resolved, "current", "--no-header"); runErr == nil {
		view.Current = asdf.ParseCurrent(result.Stdout)
	}
	if result, runErr := s.rt.Run(ctx, resolved, "plugin", "list"); runErr == nil {
		view.Plugins = asdf.ParsePluginNames(result.Stdout)
	}

	for _, tool := range view.Current {
		if !tool.Installed {
			view.Missing = append(view.Missing, tool.Name)
		}
	}
	view.Missing = uniqueSorted(view.Missing)

	pluginSet := map[string]bool{}
	for _, plugin := range view.Plugins {
		pluginSet[plugin] = true
	}
	var missingPlugins []string
	for _, entry := range view.Entries {
		if !pluginSet[entry.Tool] {
			missingPlugins = append(missingPlugins, entry.Tool)
		}
	}
	if len(missingPlugins) > 0 {
		view.Warnings = append(view.Warnings, fmt.Sprintf("No plugin installed for: %s", strings.Join(uniqueSorted(missingPlugins), ", ")))
	}
	if !view.FileExists {
		view.Warnings = append(view.Warnings, "This directory has no .tool-versions file yet.")
	}
	if view.UsingParent {
		view.Warnings = append(view.Warnings, fmt.Sprintf("Using the nearest file in a parent directory: %s", view.FilePath))
	}
	if !s.rt.Detection().Found {
		view.Warnings = append(view.Warnings, "asdf is not available, so current versions could not be resolved.")
	}
	view.Recent = s.Recent()
	return view, nil
}

// Save rewrites the project's own .tool-versions file (never a parent file).
func (s *ProjectService) Save(dir string, entries []asdf.ToolVersionEntry) (ProjectView, error) {
	resolved, err := s.resolveDir(dir)
	if err != nil {
		return ProjectView{}, err
	}
	seen := map[string]bool{}
	clean := make([]asdf.ToolVersionEntry, 0, len(entries))
	for _, entry := range entries {
		tool := strings.TrimSpace(entry.Tool)
		if err := asdf.ValidateName("tool", tool); err != nil {
			return ProjectView{}, err
		}
		if seen[tool] {
			return ProjectView{}, fmt.Errorf("tool %q appears more than once", tool)
		}
		seen[tool] = true
		if len(entry.Versions) == 0 {
			return ProjectView{}, fmt.Errorf("tool %q has no version", tool)
		}
		for _, version := range entry.Versions {
			if err := asdf.ValidateVersion(version); err != nil {
				return ProjectView{}, err
			}
		}
		clean = append(clean, asdf.ToolVersionEntry{Tool: tool, Versions: entry.Versions})
	}

	path := filepath.Join(resolved, asdf.ToolVersionsFilename)
	file, err := asdf.ReadToolVersions(path)
	if err != nil {
		return ProjectView{}, err
	}
	file.ReplaceEntries(clean)
	if err := file.Write(); err != nil {
		return ProjectView{}, err
	}
	_ = s.rt.SettingsStore().AddRecentProject(resolved)
	return s.Open(resolved, false)
}

// ChooseDirectory opens the native folder picker.
func (s *ProjectService) ChooseDirectory() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application is not ready")
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("Choose a project directory").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (s *ProjectService) Reveal(dir string) error {
	resolved, err := s.resolveDir(dir)
	if err != nil {
		return err
	}
	return openInFileManager(resolved, false)
}

func (s *ProjectService) Recent() []RecentProject {
	projects := s.rt.Settings().RecentProjects
	out := make([]RecentProject, 0, len(projects))
	for _, path := range projects {
		info, err := os.Stat(path)
		exists := err == nil && info.IsDir()
		hasFile := false
		if exists {
			if _, err := os.Stat(filepath.Join(path, asdf.ToolVersionsFilename)); err == nil {
				hasFile = true
			}
		}
		out = append(out, RecentProject{
			Path:            path,
			Name:            filepath.Base(path),
			Exists:          exists,
			HasToolVersions: hasFile,
		})
	}
	return out
}

func (s *ProjectService) InstallMissing(dir string) (ops.Operation, error) {
	return s.tools.InstallAll(dir)
}

func (s *ProjectService) resolveDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = strings.TrimSpace(s.rt.Settings().DefaultProjectDir)
	}
	if dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	if dir == "" {
		dir = homeDir()
	}
	abs, err := filepath.Abs(expandHome(dir))
	if err != nil {
		return "", err
	}
	if err := ensureDir(abs); err != nil {
		return "", fmt.Errorf("project directory does not exist: %s", abs)
	}
	return abs, nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
