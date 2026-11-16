package core

import (
	"path/filepath"
	"testing"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
)

func TestHostPathNativePassesThrough(t *testing.T) {
	r := &Runtime{detection: asdf.Detection{
		Found:      true,
		BinaryPath: "/usr/bin/asdf",
		RunnerMode: asdf.RunnerNative,
	}}
	if got, err := r.HostPath("/home/ada/app"); err != nil || got != "/home/ada/app" {
		t.Fatalf("HostPath() = %q, %v", got, err)
	}
	if got := r.RunnerPath("/home/ada/app"); got != "/home/ada/app" {
		t.Fatalf("RunnerPath() = %q", got)
	}
}

func TestHostPathWSLMapsLinuxPaths(t *testing.T) {
	r := &Runtime{detection: asdf.Detection{
		Found:      true,
		BinaryPath: "/home/ada/.asdf/bin/asdf",
		RunnerMode: asdf.RunnerWSL,
		WSLDistro:  "Ubuntu",
		WSLHome:    "/home/ada",
	}}

	want := `\\wsl.localhost\Ubuntu\home\ada\app`
	if got, err := r.HostPath("/home/ada/app"); err != nil || got != want {
		t.Fatalf("HostPath() = %q, %v; want %q", got, err, want)
	}
	if got := r.ResolvePath("~/app"); got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
	if got := r.ResolvePath("~"); got != `\\wsl.localhost\Ubuntu\home\ada` {
		t.Fatalf("ResolvePath(~) = %q", got)
	}
	if got := r.RunnerPath(want); got != "/home/ada/app" {
		t.Fatalf("RunnerPath() = %q", got)
	}
	if got := r.RunnerPath(`D:\code\app`); got != "/mnt/d/code/app" {
		t.Fatalf("RunnerPath(drive) = %q", got)
	}
}

func TestResolvePathWSLKeepsHostPaths(t *testing.T) {
	r := &Runtime{detection: asdf.Detection{
		Found:      true,
		BinaryPath: "/home/ada/.asdf/bin/asdf",
		RunnerMode: asdf.RunnerWSL,
		WSLDistro:  "Ubuntu",
		WSLHome:    "/home/ada",
	}}
	host := `C:\Users\Ada\project`
	if got := r.ResolvePath(host); got != filepath.Clean(host) {
		t.Fatalf("ResolvePath(%q) = %q", host, got)
	}
}
