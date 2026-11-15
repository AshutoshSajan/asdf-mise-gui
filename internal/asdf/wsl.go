package asdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Runner modes select where the version manager executes. WSL is only
// available on Windows, but the constants and runners compile everywhere so
// the UI and tests do not need platform-specific build tags.
const (
	RunnerAuto   = "auto"
	RunnerNative = "native"
	RunnerWSL    = "wsl"
)

// DetectOptions carries the settings that affect engine discovery.
type DetectOptions struct {
	Engine          string
	BinaryOverride  string
	DataDirOverride string
	RunnerMode      string
	WSLDistro       string
}

// NormalizeRunnerMode keeps unknown or empty values on the safe auto path.
func NormalizeRunnerMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case RunnerNative:
		return RunnerNative
	case RunnerWSL:
		return RunnerWSL
	default:
		return RunnerAuto
	}
}

// wslInfo is the subset of the Linux environment the bridge needs.
type wslInfo struct {
	distro    string
	engine    string
	binary    string
	home      string
	dataDir   string
	configDir string
	version   string
}

// WSLDistros lists installed WSL distributions. wsl.exe emits UTF-16LE for
// this command, unlike commands executed inside the distribution.
func WSLDistros() ([]string, error) {
	if runtime.GOOS != "windows" {
		return nil, errors.New("WSL is only available on Windows")
	}
	path, err := wslExecutable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-l", "-q").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(strings.TrimSpace(decodeWSLText(exit.Stderr))) > 0 {
			return nil, fmt.Errorf("list WSL distributions: %s", strings.TrimSpace(decodeWSLText(exit.Stderr)))
		}
		return nil, fmt.Errorf("list WSL distributions: %w", err)
	}
	distros := parseWSLDistros(out)
	if len(distros) == 0 {
		return nil, errors.New("no WSL distributions are installed")
	}
	return distros, nil
}

// parseWSLDistros tolerates UTF-16LE (the normal wsl -l output), a UTF-8 BOM,
// and CRLF line endings.
func parseWSLDistros(raw []byte) []string {
	text := decodeWSLText(raw)
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\x00"))
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// decodeWSLText detects UTF-16LE by its BOM or by the NUL bytes that UTF-16LE
// ASCII text produces. Everything else is already UTF-8.
func decodeWSLText(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xff && raw[1] == 0xfe {
		return decodeUTF16LE(raw[2:])
	}
	if len(raw) >= 2 && raw[1] == 0x00 {
		return decodeUTF16LE(raw)
	}
	return strings.TrimPrefix(string(raw), "\ufeff")
}

func decodeUTF16LE(raw []byte) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	words := make([]uint16, len(raw)/2)
	for i := range words {
		words[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	return string(utf16.Decode(words))
}

func wslExecutable() (string, error) {
	for _, name := range []string{"wsl.exe", "wsl"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("wsl.exe was not found. Enable WSL2 and install a distribution")
}

// windowsPathToWSL converts a Windows drive or WSL UNC path into the path the
// distribution sees. Linux paths pass through unchanged so a user can paste a
// path copied from a WSL shell.
func windowsPathToWSL(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}
	normalized := strings.ReplaceAll(path, `\`, "/")
	if strings.HasPrefix(normalized, "/") && !strings.HasPrefix(normalized, "//") {
		return filepath.ToSlash(path), nil
	}
	lower := strings.ToLower(normalized)
	for _, prefix := range []string{"//wsl.localhost/", "//wsl$/"} {
		if strings.HasPrefix(lower, prefix) {
			rest := normalized[len(prefix):]
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) < 2 || parts[1] == "" {
				return "/", nil
			}
			return "/" + strings.TrimPrefix(parts[1], "/"), nil
		}
	}
	if len(normalized) >= 2 && normalized[1] == ':' && isDriveLetter(normalized[0]) {
		drive := strings.ToLower(string(normalized[0]))
		rest := strings.TrimLeft(normalized[2:], "/")
		if rest == "" {
			return "/mnt/" + drive, nil
		}
		return "/mnt/" + drive + "/" + rest, nil
	}
	if strings.HasPrefix(normalized, "//") {
		return "", fmt.Errorf("%s is a UNC path WSL cannot reach; use \\\\wsl.localhost\\<distro>\\... or a drive path", path)
	}
	return "", fmt.Errorf("%s is not an absolute Windows or WSL path", path)
}

func isDriveLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

// wslPathToWindows maps a Linux path to a host-visible path under the chosen
// distributions. The caller supplies \\wsl.localhost or \\wsl$ so tests do not
// depend on a live WSL installation.
func wslPathToWindows(prefix, linuxPath string) string {
	linuxPath = strings.TrimSpace(linuxPath)
	if linuxPath == "" || linuxPath == "/" {
		return prefix + `\`
	}
	return prefix + `\` + strings.ReplaceAll(strings.TrimPrefix(linuxPath, "/"), "/", `\`)
}

// WSLHostPath exposes a Linux path through the distribution's Windows share.
func WSLHostPath(distro, linuxPath string) string {
	return wslPathToWindows(wslHostPrefix(distro), linuxPath)
}

// WSLPathFromHost maps a Windows drive or WSL UNC path back to the path the
// distribution sees. Relative paths and unreachable shares return an error.
func WSLPathFromHost(hostPath string) (string, error) {
	return windowsPathToWSL(hostPath)
}

func wslHostPrefix(distro string) string {
	for _, base := range []string{`\\wsl.localhost`, `\\wsl$`} {
		prefix := base + `\` + distro
		if _, err := os.Stat(prefix); err == nil {
			return prefix
		}
	}
	return `\\wsl.localhost\` + distro
}

// WSLRunner executes the version manager inside a WSL distribution.
type WSLRunner struct {
	wslPath    string
	distro     string
	engine     string
	binaryPath string
	dataDir    string
	homeDir    string
}

// NewWSLRunner builds a runner for an already detected Linux-side binary.
func NewWSLRunner(distro, engine, binaryPath, dataDir, homeDir string) *WSLRunner {
	if engine != EngineMise {
		engine = EngineASDF
	}
	return &WSLRunner{
		wslPath:    "wsl.exe",
		distro:     distro,
		engine:     engine,
		binaryPath: binaryPath,
		dataDir:    dataDir,
		homeDir:    homeDir,
	}
}

func (r *WSLRunner) Engine() string     { return r.engine }
func (r *WSLRunner) BinaryPath() string { return r.binaryPath }
func (r *WSLRunner) DataDir() string    { return r.dataDir }
func (r *WSLRunner) Distro() string     { return r.distro }

// Command renders the WSL invocation without executing it.
func (r *WSLRunner) Command(args ...string) string {
	parts := []string{r.wslPath, "-d", r.distro, "--", r.binaryPath}
	parts = append(parts, args...)
	return QuoteCommand(parts)
}

func (r *WSLRunner) Run(ctx context.Context, dir string, extraEnv []string, stream StreamFunc, args ...string) (Result, error) {
	result := Result{Args: args, Dir: dir}
	started := time.Now()
	defer func() { result.Duration = time.Since(started).Milliseconds() }()

	if r.binaryPath == "" {
		return result, errors.New("version manager binary is not configured")
	}
	linuxDir := ""
	if strings.TrimSpace(dir) != "" {
		var err error
		linuxDir, err = windowsPathToWSL(dir)
		if err != nil {
			return result, err
		}
	}
	cmd := r.command(ctx, linuxDir, extraEnv, args...)
	return runStreamingCommand(ctx, cmd, r.engine, result, stream)
}

func (r *WSLRunner) command(ctx context.Context, linuxDir string, extraEnv []string, args ...string) *exec.Cmd {
	command := []string{r.wslPath, "-d", r.distro}
	if linuxDir != "" {
		command = append(command, "--cd", linuxDir)
	}
	command = append(command, "--", "env")
	command = append(command, BuildWSLEnv(r.engine, r.binaryPath, r.dataDir, r.homeDir)...)
	command = append(command, extraEnv...)
	command = append(command, r.binaryPath)
	command = append(command, args...)
	return exec.CommandContext(ctx, command[0], command[1:]...)
}

// BuildWSLEnv is the WSL counterpart to BuildEnvForEngine. The values are
// passed through `env` inside the distribution because wsl.exe does not import
// the Windows environment automatically.
func BuildWSLEnv(engine, binaryPath, dataDir, homeDir string) []string {
	if engine != EngineMise {
		engine = EngineASDF
	}
	env := []string{
		"NO_COLOR=1",
		"TERM=dumb",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
	}
	if homeDir != "" {
		env = append(env, "HOME="+homeDir)
	}
	if engine == EngineMise {
		env = append(env, "MISE_DATA_DIR="+dataDir)
	} else {
		env = append(env, "ASDF_DATA_DIR="+dataDir)
		binDir := path.Dir(binaryPath)
		if path.Base(binDir) == "bin" {
			env = append(env, "ASDF_DIR="+path.Dir(binDir))
		}
	}
	return env
}

// detectWSL locates and validates a version manager inside WSL.
func detectWSL(opts DetectOptions) Detection {
	d := Detection{
		Engine:     opts.Engine,
		RunnerMode: RunnerWSL,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if runtime.GOOS != "windows" {
		d.Error = "the WSL runner is only available on Windows"
		return d
	}
	path, err := wslExecutable()
	if err != nil {
		d.Error = err.Error()
		return d
	}
	distro, err := resolveWSLDistro(path, opts.WSLDistro)
	if err != nil {
		d.Error = err.Error()
		return d
	}
	d.WSLDistro = distro
	d.Source = "WSL"
	info, err := queryWSLInfo(path, distro, opts)
	if err != nil {
		d.Error = err.Error()
		return d
	}
	d.BinaryPath = info.binary
	d.DataDir = info.dataDir
	d.HomeDir = wslPathToWindows(wslHostPrefix(distro), info.home)
	d.WSLHome = info.home
	d.ConfigDir = wslPathToWindows(wslHostPrefix(distro), info.configDir)
	d.Version = info.version
	d.Found = info.binary != "" && info.version != ""
	if info.engine != "" {
		d.Engine = info.engine
	}
	if !d.Found {
		label := engineLabel(d.Engine)
		if d.Engine == EngineAuto || d.Engine == "" {
			label = "asdf or mise"
		}
		d.Error = fmt.Sprintf("%s was not found inside the WSL distribution %q. Install it there or switch the runner.", label, distro)
	}
	return d
}

func engineLabel(engine string) string {
	if engine == EngineMise {
		return "mise"
	}
	return "asdf"
}

// resolveWSLCandidate turns a settings override into a Linux path. Tilde
// expands against the distribution's home; Windows and WSL UNC paths are
// translated; anything else is treated as an already valid Linux path.
func resolveWSLCandidate(value, home string) string {
	value = strings.TrimSpace(value)
	home = strings.TrimSpace(home)
	if home != "" {
		if value == "~" {
			return home
		}
		if strings.HasPrefix(value, "~/") {
			return path.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if mapped, err := windowsPathToWSL(value); err == nil {
		return mapped
	}
	return value
}

func resolveWSLDistro(path, preferred string) (string, error) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		return preferred, nil
	}
	distros, err := wslDistroList(path)
	if err != nil {
		return "", err
	}
	if len(distros) == 0 {
		return "", errors.New("no WSL distributions are installed")
	}
	return distros[0], nil
}

func wslDistroList(path string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-l", "-q").Output()
	if err != nil {
		return nil, fmt.Errorf("list WSL distributions: %w", err)
	}
	return parseWSLDistros(out), nil
}

func queryWSLInfo(wslPath, distro string, opts DetectOptions) (wslInfo, error) {
	script := strings.Join([]string{
		`printf '__HOME__%s\n' "$HOME"`,
		`printf '__XDG_CONFIG_HOME__%s\n' "${XDG_CONFIG_HOME:-}"`,
		`printf '__XDG_DATA_HOME__%s\n' "${XDG_DATA_HOME:-}"`,
		`printf '__ASDF_BIN__%s\n' "$(command -v asdf 2>/dev/null || true)"`,
		`printf '__ASDF_DATA__%s\n' "${ASDF_DATA_DIR:-$HOME/.asdf}"`,
		`printf '__MISE_BIN__%s\n' "$(command -v mise 2>/dev/null || true)"`,
		`printf '__MISE_DATA__%s\n' "${MISE_DATA_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/mise}"`,
		`printf '__MISE_CONFIG__%s\n' "${MISE_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/mise}"`,
	}, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, wslPath, "-d", distro, "--", "bash", "-lc", script).CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(decodeWSLText(out))
		if text == "" {
			text = err.Error()
		}
		return wslInfo{}, fmt.Errorf("inspect WSL distribution %q: %s", distro, firstOutputLine(text))
	}
	values := parseWSLInfo(string(out))

	info := wslInfo{
		distro:    distro,
		home:      values["HOME"],
		dataDir:   values["ASDF_DATA"],
		configDir: values["MISE_CONFIG"],
	}
	if opts.Engine == EngineMise {
		info.binary = values["MISE_BIN"]
		info.dataDir = values["MISE_DATA"]
	} else if opts.Engine == EngineASDF {
		info.binary = values["ASDF_BIN"]
	} else {
		// Auto keeps the native preference order: asdf first, then mise.
		if values["ASDF_BIN"] != "" {
			info.binary = values["ASDF_BIN"]
		} else {
			info.binary = values["MISE_BIN"]
			info.dataDir = values["MISE_DATA"]
		}
	}
	if override := strings.TrimSpace(opts.BinaryOverride); override != "" {
		info.binary = resolveWSLCandidate(override, info.home)
	}
	if override := strings.TrimSpace(opts.DataDirOverride); override != "" {
		info.dataDir = resolveWSLCandidate(override, info.home)
	}
	if info.binary == "" {
		return info, nil
	}

	engine := opts.Engine
	if engine == EngineAuto || engine == "" {
		if strings.Contains(path.Base(info.binary), EngineMise) {
			engine = EngineMise
		} else {
			engine = EngineASDF
		}
	}
	info.engine = engine
	env := BuildWSLEnv(engine, info.binary, info.dataDir, info.home)
	versionArgs := []string{"-d", distro, "--", "env"}
	versionArgs = append(versionArgs, env...)
	versionArgs = append(versionArgs, info.binary, "--version")
	versionCtx, versionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer versionCancel()
	versionOut, versionErr := exec.CommandContext(versionCtx, wslPath, versionArgs...).CombinedOutput()
	text := strings.TrimSpace(decodeWSLText(versionOut))
	if versionErr != nil {
		return info, fmt.Errorf("%s --version failed inside WSL: %s", engine, firstOutputLine(text))
	}
	if engine == EngineMise {
		info.version = parseMiseVersion(text)
	} else {
		info.version = parseVersionLine(text)
	}
	return info, nil
}

// parseWSLInfo extracts sentinel-prefixed values from the shell probe. Profile
// banners and other output are ignored.
func parseWSLInfo(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "__") {
			continue
		}
		rest := line[2:]
		idx := strings.Index(rest, "__")
		if idx < 0 {
			continue
		}
		key := rest[:idx]
		value := strings.TrimPrefix(rest[idx+2:], "=")
		values[key] = strings.TrimSpace(value)
	}
	return values
}
