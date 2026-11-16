package asdf

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestNormalizeRunnerMode(t *testing.T) {
	cases := map[string]string{
		"":        RunnerAuto,
		"auto":    RunnerAuto,
		"native":  RunnerNative,
		"wsl":     RunnerWSL,
		" WSL ":   RunnerWSL,
		"unknown": RunnerAuto,
	}
	for input, want := range cases {
		if got := NormalizeRunnerMode(input); got != want {
			t.Errorf("NormalizeRunnerMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWindowsPathToWSL(t *testing.T) {
	cases := []struct {
		path    string
		want    string
		wantErr bool
	}{
		{`C:\Users\Ada\project`, "/mnt/c/Users/Ada/project", false},
		{`D:/code/app`, "/mnt/d/code/app", false},
		{`C:\`, "/mnt/c", false},
		{`\\wsl.localhost\Ubuntu\home\ada\app`, "/home/ada/app", false},
		{`\\wsl$\Ubuntu\home\ada\app`, "/home/ada/app", false},
		{`/home/ada/app`, "/home/ada/app", false},
		{`relative\path`, "", true},
		{`\\server\share\project`, "", true},
		{``, "", true},
	}
	for _, test := range cases {
		got, err := windowsPathToWSL(test.path)
		if test.wantErr {
			if err == nil {
				t.Errorf("windowsPathToWSL(%q) = %q, want error", test.path, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("windowsPathToWSL(%q) returned error: %v", test.path, err)
			continue
		}
		if got != test.want {
			t.Errorf("windowsPathToWSL(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestWSLPathToWindows(t *testing.T) {
	cases := []struct {
		prefix string
		path   string
		want   string
	}{
		{`\\wsl.localhost\Ubuntu`, "/home/ada/app", `\\wsl.localhost\Ubuntu\home\ada\app`},
		{`\\wsl$\Ubuntu`, "/", `\\wsl$\Ubuntu\`},
		{`\\wsl$\Ubuntu`, "", `\\wsl$\Ubuntu\`},
	}
	for _, test := range cases {
		if got := wslPathToWindows(test.prefix, test.path); got != test.want {
			t.Errorf("wslPathToWindows(%q, %q) = %q, want %q", test.prefix, test.path, got, test.want)
		}
	}
}

func TestResolveWSLCandidate(t *testing.T) {
	cases := map[string]string{
		"~":                          "/home/ada",
		"~/bin/asdf":                 "/home/ada/bin/asdf",
		`C:\tools\asdf.exe`:          "/mnt/c/tools/asdf.exe",
		`\\wsl.localhost\Ubuntu\opt`: "/opt",
		"/usr/local/bin/mise":        "/usr/local/bin/mise",
	}
	for input, want := range cases {
		if got := resolveWSLCandidate(input, "/home/ada"); got != want {
			t.Errorf("resolveWSLCandidate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseWSLDistrosUTF16(t *testing.T) {
	text := "Ubuntu\r\nDebian\r\n"
	words := utf16.Encode([]rune(text))
	raw := []byte{0xff, 0xfe}
	for _, word := range words {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], word)
		raw = append(raw, buf[:]...)
	}
	got := parseWSLDistros(raw)
	want := []string{"Ubuntu", "Debian"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWSLDistros() = %#v, want %#v", got, want)
	}
}

func TestParseWSLDistrosUTF8(t *testing.T) {
	got := parseWSLDistros([]byte("\ufeffUbuntu\n\nDebian\n"))
	want := []string{"Ubuntu", "Debian"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWSLDistros() = %#v, want %#v", got, want)
	}
}

func TestParseWSLInfoIgnoresProfileNoise(t *testing.T) {
	output := strings.Join([]string{
		"Welcome to Ubuntu",
		"__HOME__/home/ada",
		"__XDG_CONFIG_HOME__/home/ada/.config",
		"__XDG_DATA_HOME__",
		"__ASDF_BIN__/home/ada/.asdf/bin/asdf",
		"__ASDF_DATA__/home/ada/.asdf",
		"__MISE_BIN__",
		"__MISE_DATA__/home/ada/.local/share/mise",
		"__MISE_CONFIG__/home/ada/.config/mise",
	}, "\n")
	got := parseWSLInfo(output)
	want := map[string]string{
		"HOME":            "/home/ada",
		"XDG_CONFIG_HOME": "/home/ada/.config",
		"XDG_DATA_HOME":   "",
		"ASDF_BIN":        "/home/ada/.asdf/bin/asdf",
		"ASDF_DATA":       "/home/ada/.asdf",
		"MISE_BIN":        "",
		"MISE_DATA":       "/home/ada/.local/share/mise",
		"MISE_CONFIG":     "/home/ada/.config/mise",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWSLInfo() = %#v, want %#v", got, want)
	}
}

func TestBuildWSLEnv(t *testing.T) {
	env := BuildWSLEnv(EngineASDF, "/home/ada/.asdf/bin/asdf", "/home/ada/.asdf", "/home/ada")
	values := envMap(env)
	for key, want := range map[string]string{
		"HOME":          "/home/ada",
		"ASDF_DATA_DIR": "/home/ada/.asdf",
		"ASDF_DIR":      "/home/ada/.asdf",
		"NO_COLOR":      "1",
	} {
		if values[key] != want {
			t.Errorf("%s = %q, want %q", key, values[key], want)
		}
	}

	mise := envMap(BuildWSLEnv(EngineMise, "/usr/local/bin/mise", "/home/ada/.local/share/mise", "/home/ada"))
	if mise["MISE_DATA_DIR"] != "/home/ada/.local/share/mise" {
		t.Errorf("MISE_DATA_DIR = %q", mise["MISE_DATA_DIR"])
	}
	if _, ok := mise["ASDF_DATA_DIR"]; ok {
		t.Error("mise environment must not set ASDF_DATA_DIR")
	}
}

func TestWSLRunnerCommand(t *testing.T) {
	runner := NewWSLRunner("Ubuntu", EngineASDF, "/home/ada/.asdf/bin/asdf", "/home/ada/.asdf", "/home/ada")
	cmd := runner.command(t.Context(), "/home/ada/app", nil, "current", "--no-header")
	wantPrefix := []string{"wsl.exe", "-d", "Ubuntu", "--cd", "/home/ada/app", "--", "env"}
	if !reflect.DeepEqual(cmd.Args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("command prefix = %#v, want %#v", cmd.Args[:len(wantPrefix)], wantPrefix)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"ASDF_DATA_DIR=/home/ada/.asdf",
		"/home/ada/.asdf/bin/asdf current --no-header",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("command %q does not contain %q", joined, want)
		}
	}
}

func TestWSLRunnerCommandWithoutDirectory(t *testing.T) {
	runner := NewWSLRunner("Ubuntu", EngineMise, "/usr/bin/mise", "/home/ada/.local/share/mise", "/home/ada")
	cmd := runner.command(t.Context(), "", nil, "ls", "--json")
	if strings.Contains(strings.Join(cmd.Args, " "), "--cd") {
		t.Fatalf("command should not set --cd without a directory: %#v", cmd.Args)
	}
}

func envMap(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	return values
}
