package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/ops"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/settings"
)

const maxWorkspaceProjects = 40

// WorkspaceService manages named groups of project directories and scans them
// together, so one screen can report on several projects at once.
type WorkspaceService struct {
	rt       *core.Runtime
	projects *ProjectService
}

func NewWorkspaceService(rt *core.Runtime, projects *ProjectService) *WorkspaceService {
	return &WorkspaceService{rt: rt, projects: projects}
}

type WorkspaceInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"`
}

type WorkspaceProjectView struct {
	Dir           string                  `json:"dir"`
	Name          string                  `json:"name"`
	Exists        bool                    `json:"exists"`
	Engine        string                  `json:"engine"`
	ConfigFile    string                  `json:"configFile"`
	ConfigFormat  string                  `json:"configFormat"`
	FileExists    bool                    `json:"fileExists"`
	FileInProject bool                    `json:"fileInProject"`
	UsingParent   bool                    `json:"usingParent"`
	Entries       []asdf.ToolVersionEntry `json:"entries"`
	Current       []asdf.CurrentTool      `json:"current"`
	Missing       []string                `json:"missing"`
	Warnings      []string                `json:"warnings"`
	PinnedCount   int                     `json:"pinnedCount"`
}

type WorkspaceView struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Projects        []WorkspaceProjectView `json:"projects"`
	Missing         []string               `json:"missing"`
	PinnedCount     int                    `json:"pinnedCount"`
	MissingProjects int                    `json:"missingProjects"`
	ScannedAt       string                 `json:"scannedAt"`
}

func (s *WorkspaceService) List() []settings.Workspace {
	return s.rt.SettingsStore().Workspaces()
}

// Save creates a workspace when ID is empty and updates an existing one
// otherwise. Project paths are resolved through the active runner so WSL and
// host paths are stored consistently.
func (s *WorkspaceService) Save(input WorkspaceInput) (settings.Workspace, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return settings.Workspace{}, fmt.Errorf("workspace name is required")
	}
	if utf8.RuneCountInString(name) > 80 {
		return settings.Workspace{}, fmt.Errorf("workspace name is too long")
	}
	if len(input.Projects) > maxWorkspaceProjects {
		return settings.Workspace{}, fmt.Errorf("a workspace can hold at most %d projects", maxWorkspaceProjects)
	}

	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newWorkspaceID()
	} else if _, ok := s.rt.SettingsStore().Workspace(id); !ok {
		return settings.Workspace{}, fmt.Errorf("workspace not found: %s", id)
	}

	workspace := settings.Workspace{
		ID:       id,
		Name:     name,
		Projects: s.resolveProjects(input.Projects),
	}
	if _, err := s.rt.SettingsStore().UpsertWorkspace(workspace); err != nil {
		return settings.Workspace{}, err
	}
	return workspace, nil
}

func (s *WorkspaceService) Delete(id string) ([]settings.Workspace, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("workspace id is required")
	}
	return s.rt.SettingsStore().DeleteWorkspace(id)
}

// Scan opens every project in the workspace and returns its config, current
// versions, and missing tools. Missing directories and engine errors are
// reported per project instead of failing the whole scan.
func (s *WorkspaceService) Scan(id string) (WorkspaceView, error) {
	workspace, ok := s.rt.SettingsStore().Workspace(strings.TrimSpace(id))
	if !ok {
		return WorkspaceView{}, fmt.Errorf("workspace not found: %s", id)
	}
	return scanWorkspace(workspace, s.projects.Open, s.rt.Engine()), nil
}

// InstallMissing queues one install operation per workspace project that has
// unresolved pins. The operation queue still runs them one at a time.
func (s *WorkspaceService) InstallMissing(id string) ([]ops.Operation, error) {
	view, err := s.Scan(id)
	if err != nil {
		return nil, err
	}
	operations := make([]ops.Operation, 0)
	for _, project := range view.Projects {
		if !project.Exists || len(project.Missing) == 0 {
			continue
		}
		operation, err := s.projects.InstallMissing(project.Dir)
		if err != nil {
			return operations, err
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

func scanWorkspace(workspace settings.Workspace, open func(string, bool) (ProjectView, error), engine string) WorkspaceView {
	view := WorkspaceView{
		ID:        workspace.ID,
		Name:      workspace.Name,
		Projects:  make([]WorkspaceProjectView, 0, len(workspace.Projects)),
		Missing:   []string{},
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
	}
	missing := map[string]bool{}

	for _, dir := range workspace.Projects {
		project := WorkspaceProjectView{
			Dir:      dir,
			Name:     filepath.Base(dir),
			Engine:   engine,
			Entries:  []asdf.ToolVersionEntry{},
			Current:  []asdf.CurrentTool{},
			Missing:  []string{},
			Warnings: []string{},
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			project.Warnings = append(project.Warnings, "Project directory is missing.")
			view.Projects = append(view.Projects, project)
			continue
		}
		project.Exists = true

		opened, err := open(dir, false)
		if err != nil {
			project.Warnings = append(project.Warnings, err.Error())
			view.Projects = append(view.Projects, project)
			continue
		}
		project.Engine = opened.Engine
		project.ConfigFile = opened.FilePath
		project.ConfigFormat = opened.ConfigFormat
		project.FileExists = opened.FileExists
		project.FileInProject = opened.FileInProject
		project.UsingParent = opened.UsingParent
		project.Entries = opened.Entries
		project.Current = opened.Current
		project.Missing = opened.Missing
		project.Warnings = opened.Warnings
		project.PinnedCount = len(opened.Entries)
		if project.Entries == nil {
			project.Entries = []asdf.ToolVersionEntry{}
		}
		if project.Current == nil {
			project.Current = []asdf.CurrentTool{}
		}
		if project.Missing == nil {
			project.Missing = []string{}
		}
		if project.Warnings == nil {
			project.Warnings = []string{}
		}

		view.PinnedCount += project.PinnedCount
		if len(project.Missing) > 0 {
			view.MissingProjects++
		}
		for _, tool := range project.Missing {
			missing[tool] = true
		}
		view.Projects = append(view.Projects, project)
	}

	view.Missing = sortedKeys(missing)
	return view
}

func (s *WorkspaceService) resolveProjects(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		resolved := filepath.Clean(s.rt.ResolvePath(raw))
		if abs, err := filepath.Abs(resolved); err == nil {
			resolved = abs
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
	}
	return out
}

func newWorkspaceID() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("ws-%d", time.Now().UnixNano())
	}
	return "ws-" + hex.EncodeToString(raw[:])
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
