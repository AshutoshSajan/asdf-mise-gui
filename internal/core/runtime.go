// Package core wires settings, asdf discovery, and the operation queue into a
// single runtime shared by the Wails services.
package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-gui/internal/ops"
	"github.com/AshutoshSajan/asdf-gui/internal/settings"
)

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
	r.detection = asdf.Detect(store.Get().BinaryPath, store.Get().DataDir)
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
	detection := asdf.Detect(current.BinaryPath, current.DataDir)

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

func (r *Runtime) runnerFactory() (asdf.Runner, error) {
	detection := r.EnsureDetection()
	if !detection.Found {
		message := detection.Error
		if message == "" {
			message = "asdf is not available"
		}
		return nil, errors.New(message)
	}
	return asdf.NewLocalRunner(detection.BinaryPath, detection.DataDir), nil
}

// Runner returns a runner for the current detection.
func (r *Runtime) Runner() (asdf.Runner, asdf.Detection, error) {
	detection := r.EnsureDetection()
	if !detection.Found {
		message := detection.Error
		if message == "" {
			message = "asdf is not available"
		}
		return nil, detection, errors.New(message)
	}
	return asdf.NewLocalRunner(detection.BinaryPath, detection.DataDir), detection, nil
}

// Run executes a short read-only asdf command.
func (r *Runtime) Run(ctx context.Context, dir string, args ...string) (asdf.Result, error) {
	runner, _, err := r.Runner()
	if err != nil {
		return asdf.Result{}, err
	}
	return runner.Run(ctx, dir, nil, nil, args...)
}

// PluginCatalog returns the registry from `asdf plugin list all`, cached for a
// day. The catalog changes rarely and shipping a network call on every keypress
// would be wasteful.
func (r *Runtime) PluginCatalog(ctx context.Context) ([]asdf.AvailablePlugin, error) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	if len(r.pluginCatalog) > 0 && time.Since(r.catalogAt) < 24*time.Hour {
		return r.pluginCatalog, nil
	}
	if r.catalogErr != nil && time.Since(r.catalogAt) < 5*time.Minute {
		return nil, r.catalogErr
	}

	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	result, err := r.Run(runCtx, "", "plugin", "list", "all")
	if err != nil {
		r.catalogAt = time.Now()
		r.catalogErr = err
		return nil, err
	}
	catalog := asdf.ParseAvailablePlugins(result.Stdout)
	r.pluginCatalog = catalog
	r.catalogAt = time.Now()
	r.catalogErr = nil
	return catalog, nil
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
