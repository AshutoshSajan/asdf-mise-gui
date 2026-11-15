package asdf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFakeEngine(t *testing.T, name, output string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake engine scripts require a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), name)
	script := "#!/bin/sh\nprintf '%s\\n' " + shellQuote(output) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func TestDetectEngineFindsASDF(t *testing.T) {
	path := writeFakeEngine(t, "asdf", "asdf version 0.20.2 (revision unknown)")
	got := DetectEngine(EngineASDF, path, t.TempDir())
	if !got.Found {
		t.Fatalf("expected asdf detection to succeed: %+v", got)
	}
	if got.Engine != EngineASDF || got.Version != "0.20.2" {
		t.Fatalf("unexpected detection: %+v", got)
	}
}

func TestDetectEngineFindsMise(t *testing.T) {
	path := writeFakeEngine(t, "mise", "2026.11.4 linux-x64 (2026-11-07)")
	got := DetectEngine(EngineMise, path, t.TempDir())
	if !got.Found {
		t.Fatalf("expected mise detection to succeed: %+v", got)
	}
	if got.Engine != EngineMise || got.Version != "2026.11.4" {
		t.Fatalf("unexpected detection: %+v", got)
	}
}

func TestDetectEngineRejectsCrossEngineBinaries(t *testing.T) {
	misePath := writeFakeEngine(t, "mise", "2026.11.4 linux-x64 (2026-11-07)")
	asdfPath := writeFakeEngine(t, "asdf", "asdf version 0.20.2 (revision unknown)")

	got := DetectEngine(EngineASDF, misePath, t.TempDir())
	if got.Found {
		t.Fatalf("mise binary was accepted as asdf: %+v", got)
	}
	if !strings.Contains(got.Error, "mise binary") {
		t.Fatalf("expected a mise-binary error, got %q", got.Error)
	}

	got = DetectEngine(EngineMise, asdfPath, t.TempDir())
	if got.Found {
		t.Fatalf("asdf binary was accepted as mise: %+v", got)
	}
	if !strings.Contains(got.Error, "asdf binary") {
		t.Fatalf("expected an asdf-binary error, got %q", got.Error)
	}
}

func TestDetectEngineRejectsWrongVersionOutput(t *testing.T) {
	// A renamed or symlinked binary must still be identified by its output.
	miseOutputAsASDF := writeFakeEngine(t, "asdf", "2026.11.4 linux-x64 (2026-11-07)")
	got := DetectEngine(EngineASDF, miseOutputAsASDF, t.TempDir())
	if got.Found {
		t.Fatalf("mise version output was accepted as asdf: %+v", got)
	}
	if !strings.Contains(got.Error, "does not report an asdf version") {
		t.Fatalf("unexpected error: %q", got.Error)
	}

	asdfOutputAsMise := writeFakeEngine(t, "mise", "asdf version 0.20.2 (revision unknown)")
	got = DetectEngine(EngineMise, asdfOutputAsMise, t.TempDir())
	if got.Found {
		t.Fatalf("asdf version output was accepted as mise: %+v", got)
	}
	if !strings.Contains(got.Error, "did not report a mise version") {
		t.Fatalf("unexpected error: %q", got.Error)
	}
}

func TestDetectEngineAutoUsesSelectedBinary(t *testing.T) {
	asdfPath := writeFakeEngine(t, "asdf", "asdf version 0.20.2 (revision unknown)")
	got := DetectEngine(EngineAuto, asdfPath, t.TempDir())
	if !got.Found || got.Engine != EngineASDF {
		t.Fatalf("auto should prefer asdf: %+v", got)
	}

	misePath := writeFakeEngine(t, "mise", "2026.11.4 linux-x64 (2026-11-07)")
	got = DetectEngine(EngineAuto, misePath, t.TempDir())
	if !got.Found || got.Engine != EngineMise {
		t.Fatalf("auto should fall back to mise: %+v", got)
	}
}

func TestParseMiseVersion(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{"release", "2026.11.4 linux-x64 (2026-11-07)", "2026.11.4"},
		{"prefixed", "v2026.11.4", "2026.11.4"},
		{"named", "mise 2026.11.4", "2026.11.4"},
		{"warning before version", "mise WARN update available\n2026.11.4 linux-x64\n", "2026.11.4"},
		{"asdf output", "asdf version 0.20.2 (revision unknown)", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseMiseVersion(tc.output); got != tc.want {
				t.Fatalf("parseMiseVersion(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}
