package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/ops"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ProjectService reads and edits project config files. asdf projects use
// .tool-versions; mise projects use mise.toml and keep .tool-versions support.
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
	ConfigFile      string `json:"configFile"`
}

type ProjectView struct {
	Dir            string                  `json:"dir"`
	Engine         string                  `json:"engine"`
	FilePath       string                  `json:"filePath"`
	FileExists     bool                    `json:"fileExists"`
	FileInProject  bool                    `json:"fileInProject"`
	UsingParent    bool                    `json:"usingParent"`
	ConfigFormat   string                  `json:"configFormat"`
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
	Queued         bool                    `json:"queued"`
	Operation      *ops.Operation          `json:"operation,omitempty"`
}

// Open resolves a directory, its nearest config file, and the engine's current
// view in that directory. remember adds it to the recent list.
func (s *ProjectService) Open(dir string, remember bool) (ProjectView, error) {
	resolved, err := s.resolveDir(dir)
	if err != nil {
		return ProjectView{}, err
	}
	if remember {
		_ = s.rt.SettingsStore().AddRecentProject(resolved)
	}

	engine := s.rt.Engine()
	view := ProjectView{
		Dir:      resolved,
		Engine:   engine,
		Entries:  []asdf.ToolVersionEntry{},
		Current:  []asdf.CurrentTool{},
		Missing:  []string{},
		Plugins:  []string{},
		Warnings: []string{},
		LoadedAt: time.Now().UTC().Format(time.RFC3339),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if engine == asdf.EngineMise {
		s.openMise(ctx, resolved, &view)
	} else {
		s.openASDF(resolved, &view)
	}

	if current, runErr := s.rt.Current(ctx, resolved); runErr == nil {
		view.Current = current
	}
	if plugins, runErr := s.rt.PluginNames(ctx, resolved); runErr == nil {
		view.Plugins = plugins
	}

	view.Missing = missingTools(view.Current)

	if engine == asdf.EngineASDF {
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
	}
	if !view.FileExists {
		if engine == asdf.EngineMise {
			view.Warnings = append(view.Warnings, "This directory has no mise config file yet.")
		} else {
			view.Warnings = append(view.Warnings, "This directory has no .tool-versions file yet.")
		}
	}
	if view.UsingParent {
		view.Warnings = append(view.Warnings, fmt.Sprintf("Using the nearest config file in a parent directory: %s", view.FilePath))
	}
	if !s.rt.Detection().Found {
		view.Warnings = append(view.Warnings, fmt.Sprintf("%s is not available, so current versions could not be resolved.", engine))
	}
	view.Recent = s.Recent()
	return view, nil
}

func (s *ProjectService) openASDF(resolved string, view *ProjectView) {
	view.ConfigFormat = "tool-versions"
	view.GlobalFilePath = filepath.Join(homeDir(), asdf.ToolVersionsFilename)

	ownPath := filepath.Join(resolved, asdf.ToolVersionsFilename)
	ownFile, err := asdf.ReadToolVersions(ownPath)
	if err != nil {
		view.Warnings = append(view.Warnings, err.Error())
		return
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
			view.Warnings = append(view.Warnings, err.Error())
			return
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
}

func (s *ProjectService) openMise(ctx context.Context, resolved string, view *ProjectView) {
	view.ConfigFormat = "toml"
	view.GlobalFilePath = asdf.MiseGlobalConfigPath()

	localPath, localFound := asdf.MiseLocalConfigPath(resolved)
	effectivePath, effectiveFound := asdf.MiseEffectiveConfigPath(resolved)
	if localFound {
		view.FileInProject = true
		view.FilePath = localPath
	} else if effectiveFound {
		view.UsingParent = true
		view.FilePath = effectivePath
	} else {
		view.FilePath = asdf.MiseWriteTarget(resolved)
	}
	view.FileExists = effectiveFound
	if asdf.ReadsMiseToolVersions(view.FilePath) {
		view.ConfigFormat = "tool-versions"
	}

	list, listErr := s.rt.MiseList(ctx, resolved)
	if listErr != nil {
		view.Warnings = append(view.Warnings, fmt.Sprintf("mise could not read the project config: %s", listErr))
	}

	if view.FileExists {
		if view.ConfigFormat == "tool-versions" {
			file, err := asdf.ReadToolVersions(view.FilePath)
			if err != nil {
				view.Warnings = append(view.Warnings, err.Error())
			} else {
				view.Entries = file.Entries
			}
		} else {
			view.Entries = asdf.MiseEntriesForSource(list, view.FilePath)
			if len(view.Entries) == 0 {
				entries, err := asdf.ReadMiseToolsTOML(view.FilePath)
				if err != nil {
					view.Warnings = append(view.Warnings, err.Error())
				} else {
					view.Entries = entries
				}
			}
		}
	}
	if view.Entries == nil {
		view.Entries = []asdf.ToolVersionEntry{}
	}

	if view.GlobalFilePath != "" {
		if _, err := os.Stat(view.GlobalFilePath); err == nil {
			view.GlobalExists = true
		}
		view.GlobalEntries = asdf.MiseEntriesForSource(list, view.GlobalFilePath)
		if len(view.GlobalEntries) == 0 && view.GlobalExists {
			if entries, err := asdf.ReadMiseToolsTOML(view.GlobalFilePath); err == nil {
				view.GlobalEntries = entries
			}
		}
		if view.GlobalEntries == nil {
			view.GlobalEntries = []asdf.ToolVersionEntry{}
		}
	}
}

// Save rewrites a project config. asdf and .tool-versions files are edited as
// text, preserving comments and ordering. mise TOML files are written by
// queued `mise use` commands so the CLI remains the source of truth for its
// format.
func (s *ProjectService) Save(dir string, entries []asdf.ToolVersionEntry) (ProjectView, error) {
	resolved, err := s.resolveDir(dir)
	if err != nil {
		return ProjectView{}, err
	}
	clean, err := cleanEntries(entries)
	if err != nil {
		return ProjectView{}, err
	}

	target := filepath.Join(resolved, asdf.ToolVersionsFilename)
	if s.rt.IsMise() {
		target = asdf.MiseWriteTarget(resolved)
	}
	if !s.rt.IsMise() || asdf.ReadsMiseToolVersions(target) {
		file, err := asdf.ReadToolVersions(target)
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

	return s.saveMiseTOML(resolved, target, clean)
}

func (s *ProjectService) saveMiseTOML(resolved, target string, entries []asdf.ToolVersionEntry) (ProjectView, error) {
	current, err := asdf.ReadMiseToolsTOML(target)
	if err != nil {
		return ProjectView{}, err
	}
	desired := make(map[string]*asdf.ToolVersionEntry, len(entries))
	for index := range entries {
		desired[entries[index].Tool] = &entries[index]
	}

	var steps []ops.Step
	for _, existing := range current {
		if _, keep := desired[existing.Tool]; keep {
			continue
		}
		steps = append(steps, ops.Step{Args: []string{"use", "--path", target, "--remove", existing.Tool}})
	}
	if len(entries) > 0 {
		args := []string{"use", "--path", target}
		for _, entry := range entries {
			for _, version := range entry.Versions {
				args = append(args, entry.Tool+"@"+version)
			}
		}
		steps = append(steps, ops.Step{Args: args})
	}

	view, err := s.Open(resolved, false)
	if err != nil {
		return ProjectView{}, err
	}
	view.FilePath = target
	view.FileInProject = true
	view.UsingParent = false
	view.ConfigFormat = "toml"
	view.Entries = entries
	if len(steps) == 0 {
		return view, nil
	}

	operation, err := s.rt.Queue().Enqueue(ops.Request{
		Kind:  "project-save",
		Title: fmt.Sprintf("Save %s", filepath.Base(target)),
		Dir:   resolved,
		Steps: steps,
	})
	if err != nil {
		return ProjectView{}, err
	}
	_ = s.rt.SettingsStore().AddRecentProject(resolved)
	view.FileExists = true
	view.Queued = true
	view.Operation = &operation
	return view, nil
}

func cleanEntries(entries []asdf.ToolVersionEntry) ([]asdf.ToolVersionEntry, error) {
	seen := map[string]bool{}
	clean := make([]asdf.ToolVersionEntry, 0, len(entries))
	for _, entry := range entries {
		tool := strings.TrimSpace(entry.Tool)
		if err := asdf.ValidateName("tool", tool); err != nil {
			return nil, err
		}
		if seen[tool] {
			return nil, fmt.Errorf("tool %q appears more than once", tool)
		}
		seen[tool] = true
		if len(entry.Versions) == 0 {
			return nil, fmt.Errorf("tool %q has no version", tool)
		}
		for _, version := range entry.Versions {
			if err := asdf.ValidateVersion(version); err != nil {
				return nil, err
			}
		}
		clean = append(clean, asdf.ToolVersionEntry{Tool: tool, Versions: entry.Versions})
	}
	return clean, nil
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
		configFile := ""
		if exists {
			if s.rt.IsMise() {
				if candidate, found := asdf.MiseEffectiveConfigPath(path); found {
					configFile = candidate
				}
			} else {
				candidate := filepath.Join(path, asdf.ToolVersionsFilename)
				if _, statErr := os.Stat(candidate); statErr == nil {
					configFile = candidate
				}
			}
		}
		out = append(out, RecentProject{
			Path:            path,
			Name:            filepath.Base(path),
			Exists:          exists,
			HasToolVersions: configFile != "",
			ConfigFile:      configFile,
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

func missingTools(current []asdf.CurrentTool) []string {
	var missing []string
	for _, tool := range current {
		// `asdf current` lists every plugin, including ones that are not
		// pinned anywhere. Those have no resolved versions and are not
		// missing, so only flag tools that resolve to a real version.
		if len(tool.Versions) > 0 && !tool.Installed {
			missing = append(missing, tool.Name)
		}
	}
	return uniqueSorted(missing)
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
