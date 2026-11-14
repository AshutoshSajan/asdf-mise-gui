package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
	"github.com/AshutoshSajan/asdf-gui/internal/core"
	"github.com/AshutoshSajan/asdf-gui/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ManagerService exposes app-level status and diagnostics.
type ManagerService struct {
	rt *core.Runtime
}

func NewManagerService(rt *core.Runtime) *ManagerService {
	return &ManagerService{rt: rt}
}

type Status struct {
	Detection      asdf.Detection    `json:"detection"`
	Settings       settings.Settings `json:"settings"`
	OS             string            `json:"os"`
	Arch           string            `json:"arch"`
	Environment    string            `json:"environment"`
	WSL            bool              `json:"wsl"`
	PluginCount    int               `json:"pluginCount"`
	ToolCount      int               `json:"toolCount"`
	VersionCount   int               `json:"versionCount"`
	LogsDir        string            `json:"logsDir"`
	SettingsPath   string            `json:"settingsPath"`
	AsdfConfigFile string            `json:"asdfConfigFile"`
	NeedsSetup     bool              `json:"needsSetup"`
	QueuePending   int               `json:"queuePending"`
}

func (s *ManagerService) Status(refresh bool) Status {
	if refresh {
		s.rt.Refresh()
	}
	detection := s.rt.Detection()
	appSettings := s.rt.Settings()
	env, wsl := environment()
	logsDir, _ := settings.LogsDir()

	status := Status{
		Detection:      detection,
		Settings:       appSettings,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Environment:    env,
		WSL:            wsl,
		LogsDir:        logsDir,
		SettingsPath:   s.rt.SettingsStore().Path(),
		AsdfConfigFile: filepath.Join(homeDir(), ".asdfrc"),
		NeedsSetup:     !detection.Found,
		QueuePending:   s.rt.Queue().RunningCount(),
	}
	if detection.Found {
		plugins := s.readPlugins()
		tools := s.readTools()
		status.PluginCount = len(plugins)
		status.ToolCount = len(tools)
		for _, tool := range tools {
			status.VersionCount += len(tool.Versions)
		}
	}
	return status
}

func (s *ManagerService) readPlugins() []asdf.Plugin {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := s.rt.Run(ctx, "", "plugin", "list", "--urls", "--refs")
	if err != nil {
		return nil
	}
	return asdf.ParsePluginList(result.Stdout)
}

func (s *ManagerService) readTools() []asdf.ToolVersions {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := s.rt.Run(ctx, "", "list")
	if err != nil {
		return nil
	}
	return asdf.ParseInstalledVersions(result.Stdout)
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

type Diagnostics struct {
	Detection    asdf.Detection `json:"detection"`
	Environment  string         `json:"environment"`
	WSL          bool           `json:"wsl"`
	OS           string         `json:"os"`
	Arch         string         `json:"arch"`
	AsdfInfo     string         `json:"asdfInfo"`
	BashVersion  string         `json:"bashVersion"`
	GitVersion   string         `json:"gitVersion"`
	Shell        string         `json:"shell"`
	PathEnv      string         `json:"pathEnv"`
	LogsDir      string         `json:"logsDir"`
	SettingsPath string         `json:"settingsPath"`
	Checks       []Check        `json:"checks"`
	GeneratedAt  string         `json:"generatedAt"`
}

func (s *ManagerService) Diagnostics() Diagnostics {
	detection := s.rt.Refresh()
	env, wsl := environment()
	logsDir, _ := settings.LogsDir()
	diag := Diagnostics{
		Detection:    detection,
		Environment:  env,
		WSL:          wsl,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Shell:        os.Getenv("SHELL"),
		PathEnv:      os.Getenv("PATH"),
		LogsDir:      logsDir,
		SettingsPath: s.rt.SettingsStore().Path(),
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	if detection.Found {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		if result, err := s.rt.Run(ctx, "", "info"); err == nil {
			diag.AsdfInfo = strings.TrimSpace(result.Stdout)
		} else {
			diag.AsdfInfo = err.Error()
		}
		cancel()
	}

	diag.BashVersion, _ = versionCommand("bash", "--version")
	diag.GitVersion, _ = versionCommand("git", "--version")

	diag.Checks = append(diag.Checks, Check{
		Name:   "asdf binary",
		Status: boolStatus(detection.Found),
		Detail: detailOr(detection.BinaryPath, detection.Error),
	})
	if detection.Found {
		diag.Checks = append(diag.Checks, Check{
			Name:   "asdf version",
			Status: "ok",
			Detail: detailOr(detection.Version, "unknown"),
		})
		dataDirStatus := "warn"
		dataDirDetail := detection.DataDir
		if info, err := os.Stat(detection.DataDir); err == nil && info.IsDir() {
			dataDirStatus = "ok"
		} else if parent, err := os.Stat(filepath.Dir(detection.DataDir)); err == nil && parent.IsDir() {
			dataDirDetail += " (will be created on first install)"
			dataDirStatus = "ok"
		}
		diag.Checks = append(diag.Checks, Check{Name: "asdf data directory", Status: dataDirStatus, Detail: dataDirDetail})
	}

	bashStatus := "fail"
	bashDetail := "bash is required by asdf plugins"
	if diag.BashVersion != "" {
		bashStatus = "ok"
		bashDetail = firstLine(diag.BashVersion)
	}
	diag.Checks = append(diag.Checks, Check{Name: "bash", Status: bashStatus, Detail: bashDetail})

	gitStatus := "fail"
	gitDetail := "git is required to install plugins"
	if diag.GitVersion != "" {
		gitStatus = "ok"
		gitDetail = firstLine(diag.GitVersion)
	}
	diag.Checks = append(diag.Checks, Check{Name: "git", Status: gitStatus, Detail: gitDetail})

	shimsPath := filepath.Join(homeDir(), ".asdf", "shims")
	shimsStatus := "warn"
	shimsDetail := shimsPath + " is not on PATH; the shell will not see asdf-managed tools"
	if pathContains(os.Getenv("PATH"), shimsPath) {
		shimsStatus = "ok"
		shimsDetail = "asdf shims are on PATH"
	}
	diag.Checks = append(diag.Checks, Check{Name: "shell integration", Status: shimsStatus, Detail: shimsDetail})

	platformStatus := "ok"
	platformDetail := env
	if runtime.GOOS == "windows" {
		platformStatus = "fail"
		platformDetail = "native Windows is not supported; run the Linux build inside WSL2"
	}
	diag.Checks = append(diag.Checks, Check{Name: "platform", Status: platformStatus, Detail: platformDetail})

	return diag
}

func (s *ManagerService) OpenLogsDir() error {
	dir, err := settings.LogsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openInFileManager(dir, false)
}

func (s *ManagerService) OpenConfigDir() error {
	dir, err := settings.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openInFileManager(dir, false)
}

// OpenURL opens an http(s) documentation link in the default browser.
func (s *ManagerService) OpenURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return fmt.Errorf("only http(s) links can be opened")
	}
	app := application.Get()
	if app == nil {
		return fmt.Errorf("application is not ready")
	}
	return app.Browser.OpenURL(rawURL)
}

func environment() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		return "macOS", false
	case "linux":
		raw, err := os.ReadFile("/proc/version")
		if err == nil {
			lower := strings.ToLower(string(raw))
			if strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl") {
				return "WSL2", true
			}
		}
		return "Linux", false
	case "windows":
		return "Windows (unsupported)", false
	default:
		return runtime.GOOS, false
	}
}

func versionCommand(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func openInFileManager(path string, selectFile bool) error {
	app := application.Get()
	if app == nil {
		return fmt.Errorf("application is not ready")
	}
	return app.Env.OpenFileManager(path, selectFile)
}

func boolStatus(value bool) string {
	if value {
		return "ok"
	}
	return "fail"
}

func detailOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if fallback != "" {
		return fallback
	}
	return "not available"
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func pathContains(pathValue, entry string) bool {
	for _, part := range filepath.SplitList(pathValue) {
		if part == entry {
			return true
		}
	}
	return false
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return home
}
