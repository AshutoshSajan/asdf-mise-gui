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
)

// ToolsService reads installed tools and queues install/uninstall/set actions.
type ToolsService struct {
	rt *core.Runtime
}

func NewToolsService(rt *core.Runtime) *ToolsService {
	return &ToolsService{rt: rt}
}

type ToolView struct {
	Name             string   `json:"name"`
	Installed        []string `json:"installed"`
	Current          []string `json:"current"`
	Source           string   `json:"source"`
	CurrentInstalled bool     `json:"currentInstalled"`
	System           bool     `json:"system"`
	Message          string   `json:"message,omitempty"`
}

func (s *ToolsService) List() ([]ToolView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plugins, err := s.rt.PluginNames(ctx, homeDir())
	if err != nil {
		return nil, err
	}
	installed, err := s.rt.Tools(ctx, homeDir())
	if err != nil {
		return nil, err
	}
	current, _ := s.rt.Current(ctx, homeDir())

	installedMap := map[string][]string{}
	for _, tool := range installed {
		installedMap[tool.Tool] = tool.Versions
	}
	currentMap := map[string]asdf.CurrentTool{}
	for _, tool := range current {
		currentMap[tool.Name] = tool
	}

	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, name := range plugins {
		add(name)
	}
	for _, tool := range installed {
		add(tool.Tool)
	}
	for _, tool := range current {
		add(tool.Name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })

	views := make([]ToolView, 0, len(names))
	for _, name := range names {
		view := ToolView{Name: name, Installed: installedMap[name]}
		if view.Installed == nil {
			view.Installed = []string{}
		}
		if item, ok := currentMap[name]; ok {
			view.Current = item.Versions
			view.Source = item.Source
			view.CurrentInstalled = item.Installed
			view.System = item.System
			view.Message = item.Message
		}
		if view.Current == nil {
			view.Current = []string{}
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *ToolsService) Available(tool string) ([]string, error) {
	if err := asdf.ValidateName("tool", tool); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return s.rt.AvailableVersions(ctx, tool)
}

func (s *ToolsService) LatestAll() ([]asdf.LatestVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return s.rt.LatestAll(ctx)
}

func (s *ToolsService) Where(tool, version string) (string, error) {
	if err := asdf.ValidateName("tool", tool); err != nil {
		return "", err
	}
	if err := asdf.ValidateVersion(version); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	path, err := s.rt.Where(ctx, tool, version)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

func (s *ToolsService) Install(tool, version string) (ops.Operation, error) {
	if err := asdf.ValidateName("tool", tool); err != nil {
		return ops.Operation{}, err
	}
	if err := asdf.ValidateVersion(version); err != nil {
		return ops.Operation{}, err
	}
	args := []string{"install", tool, version}
	if s.rt.IsMise() {
		args = []string{"install", tool + "@" + version}
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "install",
		Title: fmt.Sprintf("Install %s %s", tool, version),
		Dir:   homeDir(),
		Args:  args,
	})
}

func (s *ToolsService) InstallAll(dir string) (ops.Operation, error) {
	dir, err := s.validProjectDir(dir)
	if err != nil {
		return ops.Operation{}, err
	}
	title := "Install tools from .tool-versions"
	if s.rt.IsMise() {
		title = "Install tools from mise config"
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "install-all",
		Title: title,
		Dir:   dir,
		Args:  []string{"install"},
	})
}

func (s *ToolsService) Uninstall(tool, version string) (ops.Operation, error) {
	if err := asdf.ValidateName("tool", tool); err != nil {
		return ops.Operation{}, err
	}
	if err := asdf.ValidateVersion(version); err != nil {
		return ops.Operation{}, err
	}
	args := []string{"uninstall", tool, version}
	if s.rt.IsMise() {
		args = []string{"uninstall", tool + "@" + version}
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "uninstall",
		Title: fmt.Sprintf("Uninstall %s %s", tool, version),
		Dir:   homeDir(),
		Args:  args,
	})
}

// SetVersion writes a version with the engine's own CLI, preserving its
// parsing and comment handling. scope is "local" or "global".
func (s *ToolsService) SetVersion(tool string, versions []string, scope, dir string) (ops.Operation, error) {
	if err := asdf.ValidateName("tool", tool); err != nil {
		return ops.Operation{}, err
	}
	if len(versions) == 0 {
		return ops.Operation{}, fmt.Errorf("at least one version is required")
	}
	for _, version := range versions {
		if err := asdf.ValidateVersion(version); err != nil {
			return ops.Operation{}, err
		}
	}
	if s.rt.IsMise() {
		return s.setMiseVersion(tool, versions, scope, dir)
	}

	args := []string{"set"}
	title := fmt.Sprintf("Set %s to %s (global)", tool, strings.Join(versions, ", "))
	workDir := homeDir()
	switch scope {
	case "global":
		args = append(args, "-u")
	case "local", "":
		validDir, err := s.validProjectDir(dir)
		if err != nil {
			return ops.Operation{}, err
		}
		workDir = validDir
		title = fmt.Sprintf("Set %s to %s (local)", tool, strings.Join(versions, ", "))
	default:
		return ops.Operation{}, fmt.Errorf("unknown scope %q", scope)
	}
	args = append(args, tool)
	args = append(args, versions...)
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "set",
		Title: title,
		Dir:   workDir,
		Args:  args,
	})
}

// setMiseVersion uses `mise use`, which is the tool-pin command. `mise set`
// only manages environment variables.
func (s *ToolsService) setMiseVersion(tool string, versions []string, scope, dir string) (ops.Operation, error) {
	targets := make([]string, 0, len(versions))
	for _, version := range versions {
		targets = append(targets, tool+"@"+version)
	}
	args := []string{"use"}
	title := fmt.Sprintf("Set %s to %s (global)", tool, strings.Join(versions, ", "))
	workDir := homeDir()
	switch scope {
	case "global":
		args = append(args, "--global")
	case "local", "":
		validDir, err := s.validProjectDir(dir)
		if err != nil {
			return ops.Operation{}, err
		}
		workDir = validDir
		target := asdf.MiseWriteTarget(validDir)
		args = append(args, "--path", target)
		title = fmt.Sprintf("Set %s to %s (local)", tool, strings.Join(versions, ", "))
	default:
		return ops.Operation{}, fmt.Errorf("unknown scope %q", scope)
	}
	args = append(args, targets...)
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "set",
		Title: title,
		Dir:   workDir,
		Args:  args,
	})
}

func (s *ToolsService) validProjectDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("choose a project directory first")
	}
	abs, err := filepath.Abs(expandHome(dir))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project directory does not exist: %s", abs)
	}
	return abs, nil
}
