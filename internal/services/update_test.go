package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/version"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestUpdateAssetMatcherPrefersLinuxAppImage(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "checksums.txt"},
		{Name: "asdf-mise-gui-linux-amd64.deb"},
		{Name: "asdf-mise-gui-linux-x86_64.AppImage"},
	}
	index := updateAssetMatcher(updater.CheckRequest{Platform: "linux", Arch: "amd64"}, assets)
	if index != 2 {
		t.Fatalf("matcher chose %d (%q), want the AppImage at 2", index, assetName(assets, index))
	}
}

func TestUpdateAssetMatcherChoosesMatchingArchitecture(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "asdf-mise-gui-linux-x86_64.AppImage"},
		{Name: "asdf-mise-gui-linux-aarch64.AppImage"},
	}
	index := updateAssetMatcher(updater.CheckRequest{Platform: "linux", Arch: "arm64"}, assets)
	if index != 1 {
		t.Fatalf("matcher chose %d (%q), want the aarch64 AppImage at 1", index, assetName(assets, index))
	}
}

func TestUpdateAssetMatcherFallsBackForNotificationOnlyPlatforms(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "checksums.txt"},
		{Name: "asdf-mise-gui-linux-x86_64.AppImage"},
	}
	index := updateAssetMatcher(updater.CheckRequest{Platform: "darwin", Arch: "arm64"}, assets)
	if index != 1 {
		t.Fatalf("matcher chose %d, want the first primary asset at 1", index)
	}
}

func TestUpdateAssetMatcherRejectsChecksumOnlyRelease(t *testing.T) {
	assets := []github.ReleaseAsset{{Name: "checksums.txt"}}
	if index := updateAssetMatcher(updater.CheckRequest{Platform: "linux", Arch: "amd64"}, assets); index != -1 {
		t.Fatalf("matcher chose %d, want -1", index)
	}
}

func TestCanSelfUpdateRequiresAppImageEnvironment(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("AppImage self-update is Linux-only")
	}
	image := filepath.Join(t.TempDir(), "asdf-mise-gui-linux-x86_64.AppImage")
	if err := os.WriteFile(image, []byte("appimage"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("APPIMAGE", "")
	t.Setenv("APPDIR", "")
	if canSelfUpdate() {
		t.Fatal("canSelfUpdate should be false outside an AppImage")
	}

	t.Setenv("APPIMAGE", image)
	t.Setenv("APPDIR", t.TempDir())
	if !canSelfUpdate() {
		t.Fatal("canSelfUpdate should be true when the AppImage environment is present")
	}
}

func TestInstallableRequiresAppImageChecksumAndSignature(t *testing.T) {
	release := &updater.Release{
		Artifact: updater.Artifact{Filename: "asdf-mise-gui-linux-x86_64.AppImage"},
		Verification: &updater.Verification{
			DigestAlgo:    "sha256",
			Digest:        []byte{0x01},
			SignatureAlgo: "ed25519",
			Signature:     []byte{0x02},
		},
	}
	if !installable(release) {
		t.Fatal("expected a checksummed and signed AppImage to be installable")
	}

	release.Verification = nil
	if installable(release) {
		t.Fatal("an unchecksummed release must not be installable")
	}

	release.Verification = &updater.Verification{DigestAlgo: "sha256", Digest: []byte{0x01}}
	if installable(release) {
		t.Fatal("an unsigned release must not be installable")
	}

	release.Verification = &updater.Verification{
		DigestAlgo:    "sha256",
		Digest:        []byte{0x01},
		SignatureAlgo: "ed25519",
		Signature:     []byte{0x02},
	}
	release.Artifact.Filename = "asdf-mise-gui-linux-x86_64.deb"
	if installable(release) {
		t.Fatal("a non-AppImage artifact must not be installable")
	}
}

func TestUpToDateReportsRunningVersionAsLatest(t *testing.T) {
	service := NewUpdateService(nil)
	service.markUpToDate("2026-11-16T10:00:00Z")

	if service.status.State != "up-to-date" {
		t.Fatalf("state = %q, want up-to-date", service.status.State)
	}
	if service.status.LatestVersion != version.Version {
		t.Fatalf("latestVersion = %q, want running version %q", service.status.LatestVersion, version.Version)
	}
	if service.status.CheckedAt != "2026-11-16T10:00:00Z" {
		t.Fatalf("checkedAt = %q", service.status.CheckedAt)
	}
	if service.status.Error != "" {
		t.Fatalf("error = %q, want empty", service.status.Error)
	}
}

func TestReleasePageURL(t *testing.T) {
	release := &updater.Release{
		Version: "0.2.0",
		Metadata: map[string]any{
			"github.release.htmlURL": "https://github.com/AshutoshSajan/asdf-mise-gui/releases/tag/v0.2.0",
		},
	}
	if got := releasePageURL(release); got != "https://github.com/AshutoshSajan/asdf-mise-gui/releases/tag/v0.2.0" {
		t.Fatalf("releasePageURL = %q", got)
	}

	release.Metadata = nil
	if got := releasePageURL(release); got != "https://github.com/AshutoshSajan/asdf-mise-gui/releases/tag/v0.2.0" {
		t.Fatalf("fallback releasePageURL = %q", got)
	}
}

func assetName(assets []github.ReleaseAsset, index int) string {
	if index < 0 || index >= len(assets) {
		return ""
	}
	return assets[index].Name
}
