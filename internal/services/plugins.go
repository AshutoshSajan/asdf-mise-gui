package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-gui/internal/core"
	"github.com/AshutoshSajan/asdf-gui/internal/ops"
)

// PluginsService manages asdf plugins.
type PluginsService struct {
	rt *core.Runtime
}

func NewPluginsService(rt *core.Runtime) *PluginsService {
	return &PluginsService{rt: rt}
}

func (s *PluginsService) List() ([]asdf.Plugin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := s.rt.Run(ctx, homeDir(), "plugin", "list", "--urls", "--refs")
	if err != nil {
		return nil, err
	}
	return asdf.ParsePluginList(result.Stdout), nil
}

func (s *PluginsService) Available() ([]asdf.AvailablePlugin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return s.rt.PluginCatalog(ctx)
}

func (s *PluginsService) Add(name, url string) (ops.Operation, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" && url != "" {
		name = inferPluginName(url)
	}
	if err := asdf.ValidateName("plugin", name); err != nil {
		return ops.Operation{}, err
	}
	if err := asdf.ValidateURL(url); err != nil {
		return ops.Operation{}, err
	}
	args := []string{"plugin", "add", name}
	if url != "" {
		args = append(args, url)
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "plugin-add",
		Title: fmt.Sprintf("Add plugin %s", name),
		Dir:   homeDir(),
		Args:  args,
	})
}

func (s *PluginsService) Remove(name string) (ops.Operation, error) {
	if err := asdf.ValidateName("plugin", name); err != nil {
		return ops.Operation{}, err
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "plugin-remove",
		Title: fmt.Sprintf("Remove plugin %s", name),
		Dir:   homeDir(),
		Args:  []string{"plugin", "remove", name},
	})
}

func (s *PluginsService) Update(name string) (ops.Operation, error) {
	args := []string{"plugin", "update"}
	title := "Update all plugins"
	if strings.TrimSpace(name) != "" {
		if err := asdf.ValidateName("plugin", name); err != nil {
			return ops.Operation{}, err
		}
		args = append(args, name)
		title = fmt.Sprintf("Update plugin %s", name)
	}
	return s.rt.Queue().Enqueue(ops.Request{
		Kind:  "plugin-update",
		Title: title,
		Dir:   homeDir(),
		Args:  args,
	})
}

func inferPluginName(rawURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(rawURL), "/"), ".git")
	base := filepath.Base(trimmed)
	base = strings.TrimPrefix(base, "asdf-")
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}
