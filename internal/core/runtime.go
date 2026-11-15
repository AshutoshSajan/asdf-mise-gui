// Package core wires settings, asdf discovery, and the operation queue into a
// single runtime shared by the Wails services.
package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/ops"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/settings"
)

// homeDir is the deterministic working directory for engine-wide reads. An
// empty dir would inherit the process working directory, which may be a
// project with its own config.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

type Runtime struct {
	mu        sync.RWMutex
	store     *settings.Store
	detection asdf.Detection
	queue     *ops.Queue

	cacheMu       sync.Mutex
	pluginCatalog []asdf.AvailablePlugin
	catalogAt     time.Time
	catalogErr    error

	emitter func(name string, data any)
	onEvent func(name string, data any)
}

func New() (*Runtime, error) {
	store, err := settings.Load()
	if err != nil {
		return nil, err
	}
	logDir, err := settings.LogsDir()
	if err != nil {
		return nil, err
	}

	r := &Runtime{store: store}
	appSettings := store.Get()
	r.detection = asdf.DetectEngine(appSettings.Engine, appSettings.BinaryPath, appSettings.DataDir)
	r.queue = ops.NewQueue(r.runnerFactory, logDir)
	r.queue.SetOnChange(func() {
		if r.onEvent != nil {
			r.onEvent("cache:invalidate", "operations")
		}
	})
	return r, nil
}

// SetEmitter installs the Wails event emitter once the app exists.
func (r *Runtime) SetEmitter(emit func(name string, data any)) {
	r.mu.Lock()
	r.emitter = emit
	r.mu.Unlock()
	r.queue.SetEmitter(emit)
}

// SetOnChange installs a callback used to invalidate cached read state.
func (r *Runtime) SetOnChange(fn func(name string, data any)) {
	r.mu.Lock()
	r.onEvent = fn
	r.mu.Unlock()
}

func (r *Runtime) Queue() *ops.Queue { return r.queue }

func (r *Runtime) SettingsStore() *settings.Store { return r.store }

func (r *Runtime) Settings() settings.Settings { return r.store.Get() }

func (r *Runtime) SaveSettings(next settings.Settings) (asdf.Detection, error) {
	if err := r.store.Save(next); err != nil {
		return r.Detection(), err
	}
	return r.Refresh(), nil
}

// Refresh re-runs discovery and returns the new state.
func (r *Runtime) Refresh() asdf.Detection {
	current := r.store.Get()
	detection := asdf.DetectEngine(current.Engine, current.BinaryPath, current.DataDir)

	r.mu.Lock()
	r.detection = detection
	r.mu.Unlock()

	r.cacheMu.Lock()
	r.pluginCatalog = nil
	r.catalogAt = time.Time{}
	r.catalogErr = nil
	r.cacheMu.Unlock()
	return detection
}

// Detection returns the cached detection result.
func (r *Runtime) Detection() asdf.Detection {
	r.mu.RLock()
	detection := r.detection
	r.mu.RUnlock()
	if detection.BinaryPath == "" {
		return r.Refresh()
	}
	return detection
}

// EnsureDetection refreshes when asdf is currently missing.
func (r *Runtime) EnsureDetection() asdf.Detection {
	detection := r.Detection()
	if !detection.Found {
		detection = r.Refresh()
	}
	return detection
}

// Engine returns the active version manager: "asdf" or "mise".
func (r *Runtime) Engine() string {
	engine := r.Detection().Engine
	if engine == "" || engine == asdf.EngineAuto {
		return asdf.EngineASDF
	}
	return engine
}

// IsMise reports whether the app is driving mise.
func (r *Runtime) IsMise() bool { return r.Engine() == asdf.EngineMise }

func (r *Runtime) runnerFactory() (asdf.Runner, error) {
	detection := r.EnsureDetection()
	if !detection.Found {
		message := detection.Error
		if message == "" {
			message = "the version manager is not available"
		}
		return nil, errors.New(message)
	}
	return asdf.NewLocalRunnerForEngine(detection.Engine, detection.BinaryPath, detection.DataDir), nil
}

// Runner returns a runner for the current detection.
func (r *Runtime) Runner() (asdf.Runner, asdf.Detection, error) {
	detection := r.EnsureDetection()
	if !detection.Found {
		message := detection.Error
		if message == "" {
			message = "the version manager is not available"
		}
		return nil, detection, errors.New(message)
	}
	return asdf.NewLocalRunnerForEngine(detection.Engine, detection.BinaryPath, detection.DataDir), detection, nil
}

// Run executes a short read-only asdf command.
func (r *Runtime) Run(ctx context.Context, dir string, args ...string) (asdf.Result, error) {
	runner, _, err := r.Runner()
	if err != nil {
		return asdf.Result{}, err
	}
	return runner.Run(ctx, dir, nil, nil, args...)
}

// PluginCatalog returns the registry from `asdf plugin list all` or mise's
// plugin registry, cached for a day. The catalog changes rarely and shipping a
// network call on every keypress would be wasteful.
func (r *Runtime) PluginCatalog(ctx context.Context) ([]asdf.AvailablePlugin, error) {
	// Read the cache without holding the lock across detection or the child
	// process: Refresh and PluginCatalog both touch cacheMu, and a missing
	// engine makes detection call Refresh.
	r.cacheMu.Lock()
	cached := r.pluginCatalog
	catalogAt := r.catalogAt
	cachedErr := r.catalogErr
	r.cacheMu.Unlock()

	if len(cached) > 0 && time.Since(catalogAt) < 24*time.Hour {
		return cached, nil
	}
	if cachedErr != nil && time.Since(catalogAt) < 5*time.Minute {
		return nil, cachedErr
	}

	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var catalog []asdf.AvailablePlugin
	var err error
	if r.IsMise() {
		result, runErr := r.Run(runCtx, homeDir(), "plugins", "ls-remote", "-u")
		if runErr != nil {
			err = runErr
		} else {
			catalog = asdf.ParseMisePluginCatalog(result.Stdout)
		}
	} else {
		result, runErr := r.Run(runCtx, homeDir(), "plugin", "list", "all")
		if runErr != nil {
			err = runErr
		} else {
			catalog = asdf.ParseAvailablePlugins(result.Stdout)
		}
	}

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	if err != nil {
		r.catalogAt = time.Now()
		r.catalogErr = err
		return nil, err
	}
	r.pluginCatalog = catalog
	r.catalogAt = time.Now()
	r.catalogErr = nil
	return catalog, nil
}

// Plugins lists installed plugins for the active engine.
func (r *Runtime) Plugins(ctx context.Context, dir string) ([]asdf.Plugin, error) {
	if r.IsMise() {
		result, err := r.Run(ctx, dir, "plugins", "ls", "-u")
		if err != nil {
			return nil, err
		}
		return asdf.ParseMisePluginList(result.Stdout), nil
	}
	result, err := r.Run(ctx, dir, "plugin", "list", "--urls", "--refs")
	if err != nil {
		return nil, err
	}
	return asdf.ParsePluginList(result.Stdout), nil
}

// PluginNames lists installed plugin names for the active engine.
func (r *Runtime) PluginNames(ctx context.Context, dir string) ([]string, error) {
	plugins, err := r.Plugins(ctx, dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name)
	}
	return names, nil
}

// Tools lists installed tool versions for the active engine.
func (r *Runtime) Tools(ctx context.Context, dir string) ([]asdf.ToolVersions, error) {
	if r.IsMise() {
		list, err := r.MiseList(ctx, dir)
		if err != nil {
			return nil, err
		}
		return asdf.MiseInstalledTools(list), nil
	}
	result, err := r.Run(ctx, dir, "list")
	if err != nil {
		return nil, err
	}
	return asdf.ParseInstalledVersions(result.Stdout), nil
}

// Current lists resolved/requested tools for the active engine in dir.
func (r *Runtime) Current(ctx context.Context, dir string) ([]asdf.CurrentTool, error) {
	if r.IsMise() {
		list, err := r.MiseList(ctx, dir)
		if err != nil {
			return nil, err
		}
		return asdf.MiseCurrentTools(list), nil
	}
	result, err := r.Run(ctx, dir, "current", "--no-header")
	if err != nil {
		return nil, err
	}
	return asdf.ParseCurrent(result.Stdout), nil
}

// MiseList returns mise's structured view in dir.
func (r *Runtime) MiseList(ctx context.Context, dir string) (asdf.MiseList, error) {
	result, err := r.Run(ctx, dir, "ls", "--json")
	if err != nil {
		return nil, err
	}
	return asdf.ParseMiseListJSON(result.Stdout)
}

// AvailableVersions lists installable versions for a tool.
func (r *Runtime) AvailableVersions(ctx context.Context, tool string) ([]string, error) {
	if r.IsMise() {
		result, err := r.Run(ctx, homeDir(), "ls-remote", tool, "--json")
		if err != nil {
			return nil, err
		}
		return asdf.ParseMiseAvailableVersionsJSON(result.Stdout)
	}
	result, err := r.Run(ctx, homeDir(), "list", "all", tool)
	if err != nil {
		return nil, err
	}
	return asdf.ParseAvailableVersions(result.Stdout), nil
}

// LatestAll returns latest-version hints for the active engine.
func (r *Runtime) LatestAll(ctx context.Context) ([]asdf.LatestVersion, error) {
	if r.IsMise() {
		result, err := r.Run(ctx, homeDir(), "outdated", "--json", "--bump", "--inactive")
		if err != nil {
			return nil, err
		}
		return asdf.ParseMiseOutdatedJSON(result.Stdout)
	}
	result, err := r.Run(ctx, homeDir(), "latest", "--all")
	if err != nil {
		return nil, err
	}
	return asdf.ParseLatestAll(result.Stdout), nil
}

// Where resolves the install directory for a tool version.
func (r *Runtime) Where(ctx context.Context, tool, version string) (string, error) {
	if r.IsMise() {
		result, err := r.Run(ctx, homeDir(), "where", tool+"@"+version)
		if err != nil {
			return "", err
		}
		return result.Stdout, nil
	}
	result, err := r.Run(ctx, homeDir(), "where", tool, version)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// EngineInfo returns the diagnostic text for the active engine.
func (r *Runtime) EngineInfo(ctx context.Context, dir string) (string, error) {
	if r.IsMise() {
		result, err := r.Run(ctx, dir, "doctor")
		text := result.Stdout
		if strings.TrimSpace(result.Stderr) != "" {
			if strings.TrimSpace(text) != "" {
				text += "\n"
			}
			text += result.Stderr
		}
		if err != nil {
			return strings.TrimSpace(text), err
		}
		return strings.TrimSpace(text), nil
	}
	result, err := r.Run(ctx, dir, "info")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// MiseConfigFiles returns the config files mise loads in dir.
func (r *Runtime) MiseConfigFiles(ctx context.Context, dir string) ([]asdf.MiseConfigFile, error) {
	if !r.IsMise() {
		return nil, nil
	}
	result, err := r.Run(ctx, dir, "config", "ls", "--json")
	if err != nil {
		return nil, err
	}
	return asdf.ParseMiseConfigListJSON(result.Stdout)
}

func (r *Runtime) Emit(name string, data any) {
	r.mu.RLock()
	emit := r.emitter
	r.mu.RUnlock()
	if emit != nil {
		emit(name, data)
	}
}

func (r *Runtime) Close() {
	if r.queue != nil {
		r.queue.Close()
	}
}
