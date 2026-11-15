package main

import (
	"embed"
	"log"
	"log/slog"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/ops"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/services"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails embeds the production frontend into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Registering event payload types gives the TypeScript bindings a typed
	// event map. Every asdf invocation reports its lifecycle here.
	application.RegisterEvent[ops.Operation](ops.EventQueued)
	application.RegisterEvent[ops.Operation](ops.EventStarted)
	application.RegisterEvent[ops.Operation](ops.EventUpdated)
	application.RegisterEvent[ops.OutputBatch](ops.EventOutput)
	application.RegisterEvent[ops.Operation](ops.EventFinished)
	application.RegisterEvent[string](ops.EventState)
	application.RegisterEvent[string]("cache:invalidate")
}

func main() {
	runtime, err := core.New()
	if err != nil {
		log.Fatalf("asdf-mise-gui: initialise runtime: %v", err)
	}
	defer runtime.Close()

	managerService := services.NewManagerService(runtime)
	toolsService := services.NewToolsService(runtime)
	pluginsService := services.NewPluginsService(runtime)
	projectService := services.NewProjectService(runtime, toolsService)
	operationService := services.NewOperationService(runtime)
	settingsService := services.NewSettingsService(runtime)

	app := application.New(application.Options{
		Name:        "asdf-mise-gui",
		Description: "A desktop GUI for asdf and mise",
		LogLevel:    slog.LevelInfo,
		Services: []application.Service{
			application.NewService(managerService),
			application.NewService(toolsService),
			application.NewService(pluginsService),
			application.NewService(projectService),
			application.NewService(operationService),
			application.NewService(settingsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	runtime.SetEmitter(func(name string, data any) {
		app.Event.Emit(name, data)
	})
	runtime.SetOnChange(func(name string, data any) {
		app.Event.Emit(name, data)
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "asdf mise GUI",
		Width:            1280,
		Height:           840,
		MinWidth:         980,
		MinHeight:        640,
		BackgroundColour: application.NewRGB(11, 13, 12),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})

	if err := app.Run(); err != nil {
		log.Fatalf("asdf-mise-gui: %v", err)
	}
}
