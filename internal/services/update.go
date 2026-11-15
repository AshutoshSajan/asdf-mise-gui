package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const (
	updateRepository     = "AshutoshSajan/asdf-mise-gui"
	updateChecksumAsset  = "checksums.txt"
	latestReleasePageURL = "https://github.com/" + updateRepository + "/releases/latest"
)

// UpdateService checks GitHub releases and, when running from an AppImage,
// installs a verified update in place and restarts the application.
type UpdateService struct {
	rt *core.Runtime

	mu            sync.RWMutex
	app           *application.App
	configured    bool
	configuredErr string
	status        UpdateStatus
	pending       *updater.Release

	// opMu serializes Check and Install. The Wails updater has one pending
	// release slot and one staging directory, so two flows must never overlap.
	opMu sync.Mutex
}

// UpdateStatus is the single snapshot the frontend renders. It deliberately
// avoids leaking updater.Release internals such as raw digests.
type UpdateStatus struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	ReleaseURL     string `json:"releaseURL"`
	PublishedAt    string `json:"publishedAt"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	Checksummed    bool   `json:"checksummed"`
	State          string `json:"state"`
	Error          string `json:"error"`
	CheckedAt      string `json:"checkedAt"`
	CheckEnabled   bool   `json:"checkEnabled"`
	CanSelfUpdate  bool   `json:"canSelfUpdate"`
	Installable    bool   `json:"installable"`
}

func NewUpdateService(rt *core.Runtime) *UpdateService {
	return &UpdateService{
		rt: rt,
		status: UpdateStatus{
			CurrentVersion: version.Version,
			ReleaseURL:     latestReleasePageURL,
			State:          "idle",
		},
	}
}

// ServiceStartup wires the Wails updater to the public GitHub release feed.
// Wails excludes this lifecycle method from generated bindings, so the setup
// stays backend-only. A failure is recorded for the UI but never aborts app
// startup.
func (s *UpdateService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	app := application.Get()
	if app == nil {
		s.setConfiguredError(errors.New("application is not ready"))
		return nil
	}
	s.mu.Lock()
	s.app = app
	s.configured = false
	s.configuredErr = ""
	s.mu.Unlock()

	provider, err := github.New(github.Config{
		Repository:    updateRepository,
		ChecksumAsset: updateChecksumAsset,
		AssetMatcher:  updateAssetMatcher,
	})
	if err != nil {
		s.setConfiguredError(err)
		return nil
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: version.Version,
		Providers:      []updater.Provider{provider},
		// The app renders its own banner, update card, and progress dialog.
		Window: updater.WindowNone,
	}); err != nil {
		s.setConfiguredError(err)
		return nil
	}

	s.mu.Lock()
	s.configured = true
	s.configuredErr = ""
	s.mu.Unlock()
	return nil
}

func (s *UpdateService) setConfiguredError(err error) {
	s.mu.Lock()
	s.configuredErr = err.Error()
	s.mu.Unlock()
}

// Status returns the last known update state without touching the network.
func (s *UpdateService) Status() UpdateStatus {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()
	return s.decorate(status)
}

// Check asks GitHub for the latest release. It never downloads an artifact.
func (s *UpdateService) Check() UpdateStatus {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	app := s.app
	configured := s.configured
	configuredErr := s.configuredErr
	s.pending = nil
	s.status.State = "checking"
	s.status.Error = ""
	s.mu.Unlock()

	if app == nil || !configured {
		message := configuredErr
		if message == "" {
			message = "update checker is not configured"
		}
		return s.fail(message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	release, err := app.Updater.Check(ctx)
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		return s.failWithTime(checkedAt, err.Error())
	}
	if release == nil {
		s.mu.Lock()
		s.pending = nil
		s.status.LatestVersion = ""
		s.status.ReleaseName = ""
		s.status.ReleaseNotes = ""
		s.status.ReleaseURL = latestReleasePageURL
		s.status.PublishedAt = ""
		s.status.AssetName = ""
		s.status.AssetSize = 0
		s.status.Checksummed = false
		s.status.State = "up-to-date"
		s.status.Error = ""
		s.status.CheckedAt = checkedAt
		s.mu.Unlock()
		return s.Status()
	}

	s.mu.Lock()
	s.pending = release
	s.status.LatestVersion = strings.TrimPrefix(release.Version, "v")
	s.status.ReleaseName = release.Name
	s.status.ReleaseNotes = release.Notes
	s.status.ReleaseURL = releasePageURL(release)
	s.status.PublishedAt = ""
	if !release.PublishedAt.IsZero() {
		s.status.PublishedAt = release.PublishedAt.UTC().Format(time.RFC3339)
	}
	s.status.AssetName = release.Artifact.Filename
	s.status.AssetSize = release.Artifact.Size
	s.status.Checksummed = hasChecksum(release)
	s.status.State = "available"
	s.status.Error = ""
	s.status.CheckedAt = checkedAt
	s.mu.Unlock()
	return s.Status()
}

// Install downloads, verifies, swaps, and restarts. It is only supported for
// AppImages; every other build gets a release-page link from the frontend.
func (s *UpdateService) Install() (UpdateStatus, error) {
	if !canSelfUpdate() {
		err := errors.New("self-update is only available in the AppImage build; download the new release from GitHub")
		return s.Status(), err
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.RLock()
	app := s.app
	configured := s.configured
	pending := s.pending
	s.mu.RUnlock()

	if app == nil || !configured {
		err := errors.New("update checker is not configured")
		return s.fail(err.Error()), err
	}
	if pending == nil {
		err := errors.New("no update is ready to install; check for updates first")
		return s.Status(), err
	}
	if !installable(pending) {
		err := fmt.Errorf("release %s does not publish an installable AppImage with a checksum", pending.Version)
		return s.fail(err.Error()), err
	}

	s.mu.Lock()
	s.status.State = "downloading"
	s.status.Error = ""
	s.mu.Unlock()

	// Downloads are verified while streaming; ten minutes leaves room for slow
	// links without holding the updater's operation lock forever.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := app.Updater.DownloadAndInstall(ctx); err != nil {
		return s.fail(err.Error()), err
	}

	s.mu.Lock()
	s.status.State = "ready"
	s.status.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Unlock()

	if err := app.Updater.Restart(ctx); err != nil {
		return s.fail(err.Error()), err
	}
	return s.Status(), nil
}

func (s *UpdateService) fail(message string) UpdateStatus {
	return s.failWithTime(time.Now().UTC().Format(time.RFC3339), message)
}

func (s *UpdateService) failWithTime(checkedAt, message string) UpdateStatus {
	s.mu.Lock()
	s.status.LatestVersion = ""
	s.status.ReleaseName = ""
	s.status.ReleaseNotes = ""
	s.status.ReleaseURL = latestReleasePageURL
	s.status.PublishedAt = ""
	s.status.AssetName = ""
	s.status.AssetSize = 0
	s.status.Checksummed = false
	s.status.State = "error"
	s.status.Error = message
	s.status.CheckedAt = checkedAt
	s.mu.Unlock()
	return s.Status()
}

func (s *UpdateService) decorate(status UpdateStatus) UpdateStatus {
	status.CurrentVersion = version.Version
	status.CheckEnabled = s.rt.Settings().CheckForUpdates
	status.CanSelfUpdate = canSelfUpdate()
	status.Installable = status.CanSelfUpdate &&
		status.LatestVersion != "" &&
		strings.HasSuffix(strings.ToLower(status.AssetName), ".appimage") &&
		status.Checksummed
	if status.ReleaseURL == "" {
		status.ReleaseURL = latestReleasePageURL
	}
	return status
}

func canSelfUpdate() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	image := os.Getenv("APPIMAGE")
	mount := os.Getenv("APPDIR")
	if image == "" || mount == "" {
		return false
	}
	info, err := os.Stat(image)
	return err == nil && info.Mode().IsRegular()
}

func installable(release *updater.Release) bool {
	return release != nil &&
		strings.HasSuffix(strings.ToLower(release.Artifact.Filename), ".appimage") &&
		hasChecksum(release)
}

func hasChecksum(release *updater.Release) bool {
	return release != nil &&
		release.Verification != nil &&
		len(release.Verification.Digest) > 0 &&
		strings.EqualFold(release.Verification.DigestAlgo, "sha256")
}

func releasePageURL(release *updater.Release) string {
	if release != nil && release.Metadata != nil {
		if page, ok := release.Metadata["github.release.htmlURL"].(string); ok && page != "" {
			return page
		}
	}
	if release == nil || release.Version == "" {
		return latestReleasePageURL
	}
	return fmt.Sprintf("https://github.com/%s/releases/tag/v%s", updateRepository, strings.TrimPrefix(release.Version, "v"))
}

// updateAssetMatcher prefers a native Linux AppImage. Other platforms fall
// back to the framework's matcher, and finally to any primary asset so a build
// without a native artifact can still show release notes and link out.
func updateAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	platform := strings.ToLower(req.Platform)
	arch := strings.ToLower(req.Arch)

	if platform == "linux" {
		for i, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.HasSuffix(name, ".appimage") &&
				strings.Contains(name, "linux") &&
				matchesArch(name, arch) {
				return i
			}
		}
	}

	if index := github.DefaultAssetMatcher(req, assets); index >= 0 {
		return index
	}

	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".asc") {
			continue
		}
		if name == updateChecksumAsset ||
			strings.HasPrefix(name, "sha256sums") ||
			strings.HasPrefix(name, "sha512sums") {
			continue
		}
		return i
	}
	return -1
}

func matchesArch(name, arch string) bool {
	if strings.Contains(name, arch) {
		return true
	}
	switch arch {
	case "amd64":
		return strings.Contains(name, "x86_64") || strings.Contains(name, "x64")
	case "arm64":
		return strings.Contains(name, "aarch64")
	case "386":
		if strings.Contains(name, "x86_64") || strings.Contains(name, "x64") || strings.Contains(name, "amd64") {
			return false
		}
		return strings.Contains(name, "i386") || strings.Contains(name, "ia32") || strings.Contains(name, "x86")
	}
	return false
}
