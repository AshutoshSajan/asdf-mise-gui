package version

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVersionIsSemantic(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?$`)
	if !pattern.MatchString(Version) {
		t.Fatalf("Version %q is not a semantic version", Version)
	}
}

// TestBuildMetadataMatchesVersion keeps the source constant, the Wails build
// metadata, the Linux package metadata, and the frontend package in sync.
// Release builds re-run scripts/set-version.mjs, so local drift would only be
// caught here before the tag is pushed.
func TestBuildMetadataMatchesVersion(t *testing.T) {
	checks := []struct {
		path   string
		prefix string
	}{
		// config.yml also has a top-level `version: '3'` for the config
		// schema, so match the indented application version only.
		{"../../build/config.yml", "  version:"},
		{"../../build/linux/nfpm/nfpm.yaml", "version:"},
	}
	for _, check := range checks {
		if got := quotedVersion(t, check.path, check.prefix); got != Version {
			t.Errorf("%s version = %q, want %q", check.path, got, Version)
		}
	}

	for _, plist := range []string{"../../build/darwin/Info.plist", "../../build/darwin/Info.dev.plist"} {
		if got := plistVersion(t, plist, "CFBundleShortVersionString"); got != Version {
			t.Errorf("%s CFBundleShortVersionString = %q, want %q", plist, got, Version)
		}
		if got := plistVersion(t, plist, "CFBundleVersion"); got != Version {
			t.Errorf("%s CFBundleVersion = %q, want %q", plist, got, Version)
		}
	}

	raw, err := os.ReadFile("../../frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != Version {
		t.Errorf("frontend/package.json version = %q, want %q", pkg.Version, Version)
	}
}

func plistVersion(t *testing.T, path, key string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`<key>` + regexp.QuoteMeta(key) + `</key>\s*<string>([^<]+)</string>`)
	match := pattern.FindStringSubmatch(string(raw))
	if len(match) != 2 {
		t.Fatalf("no %s entry found in %s", key, path)
	}
	return match[1]
}

func quotedVersion(t *testing.T, path, prefix string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		match := regexp.MustCompile(`^[^"']*["']([^"']+)["']`).FindStringSubmatch(line)
		if len(match) == 2 {
			return match[1]
		}
	}
	t.Fatalf("no %s entry found in %s", prefix, path)
	return ""
}
