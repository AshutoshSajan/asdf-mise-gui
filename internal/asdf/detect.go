package asdf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Detection describes the version manager the app will drive.
type Detection struct {
	Found      bool   `json:"found"`
	Engine     string `json:"engine"`
	BinaryPath string `json:"binaryPath"`
	DataDir    string `json:"dataDir"`
	Version    string `json:"version"`
	Source     string `json:"source"`
	Error      string `json:"error,omitempty"`
	CheckedAt  string `json:"checkedAt"`
}

// Engines supported by the GUI.
const (
	EngineAuto = "auto"
	EngineASDF = "asdf"
	EngineMise = "mise"
)

var versionPatternCLI = regexp.MustCompile(`(?i)asdf\s+version\s+(v?[0-9]+\.[0-9]+\.[0-9]+[^\s]*)`)

// Detect locates the asdf binary. It is kept for callers that only care about
// asdf; DetectEngine is the engine-aware entry point used by the app.
func Detect(binaryOverride, dataDirOverride string) Detection {
	return DetectEngine(EngineASDF, binaryOverride, dataDirOverride)
}

// DetectEngine locates the selected version manager and validates it by running
// --version. Overrides come from the app settings and take precedence over
// discovery. With "auto", asdf wins when both engines are installed so that
// existing users keep their current setup; mise is the fallback.
func DetectEngine(engine, binaryOverride, dataDirOverride string) Detection {
	switch engine {
	case EngineASDF:
		return detectASDF(binaryOverride, dataDirOverride)
	case EngineMise:
		return detectMise(binaryOverride, dataDirOverride)
	case "", EngineAuto:
		asdfDetection := detectASDF(binaryOverride, dataDirOverride)
		if asdfDetection.Found {
			return asdfDetection
		}
		miseDetection := detectMise(binaryOverride, dataDirOverride)
		if miseDetection.Found {
			return miseDetection
		}
		asdfDetection.Engine = EngineAuto
		asdfDetection.Error = fmt.Sprintf("asdf: %s; mise: %s", asdfDetection.Error, miseDetection.Error)
		return asdfDetection
	default:
		return Detection{
			Engine:    engine,
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			Error:     fmt.Sprintf("unknown engine %q: choose auto, asdf, or mise", engine),
		}
	}
}

func detectASDF(binaryOverride, dataDirOverride string) Detection {
	d := Detection{Engine: EngineASDF, CheckedAt: time.Now().UTC().Format(time.RFC3339)}

	candidates := candidateBinaries("asdf", binaryOverride)
	var firstErr string
	for _, candidate := range candidates {
		path, err := resolveExecutable(candidate.path)
		if err != nil {
			if candidate.source == "settings" || candidate.source == "PATH" {
				firstErr = fmt.Sprintf("%s: %v", candidate.path, err)
			}
			continue
		}
		d.BinaryPath = path
		d.Source = candidate.source
		break
	}

	if d.BinaryPath == "" {
		d.Error = "asdf was not found. Install it, or set the binary path in Settings."
		if firstErr != "" {
			d.Error = firstErr
		}
		return d
	}

	if engineBinaryBase(d.BinaryPath) == EngineMise {
		d.Error = fmt.Sprintf("%s is a mise binary, not asdf. Set the engine to mise or choose the asdf binary.", d.BinaryPath)
		return d
	}

	d.DataDir = resolveDataDir(dataDirOverride)
	if d.DataDir == "" {
		d.Error = "could not determine the asdf data directory"
		return d
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, d.BinaryPath, "--version")
	cmd.Env = BuildEnv(d.BinaryPath, d.DataDir)
	cmd.Dir = homeDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.Error = strings.TrimSpace(string(out))
		if d.Error == "" {
			d.Error = err.Error()
		}
		d.Error = "asdf --version failed: " + d.Error
		return d
	}

	text := strings.TrimSpace(string(out))
	d.Version = parseVersionLine(text)
	if d.Version == "" {
		d.Error = fmt.Sprintf("%s does not report an asdf version: %q", d.BinaryPath, firstOutputLine(text))
		return d
	}
	d.Found = true
	return d
}

// engineBinaryBase returns the binary file name without a Windows extension,
// lowercased so engine names compare consistently.
func engineBinaryBase(path string) string {
	base := strings.ToLower(filepath.Base(path))
	return strings.TrimSuffix(base, ".exe")
}

// firstOutputLine keeps detection errors readable when a binary prints a banner
// before its version (mise prints update warnings on stderr).
func firstOutputLine(output string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	if len(line) > 160 {
		line = line[:160] + "…"
	}
	return line
}

type binaryCandidate struct {
	path   string
	source string
}

func candidateBinaries(name, override string) []binaryCandidate {
	var out []binaryCandidate
	seen := map[string]bool{}
	add := func(path, source string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, binaryCandidate{path: path, source: source})
	}

	if override != "" {
		add(expandHome(override), "settings")
		return out
	}

	if path, err := exec.LookPath(name); err == nil {
		add(path, "PATH")
	}

	home := homeDir()
	add(filepath.Join(home, ".asdf", "bin", name), "~/.asdf/bin")
	add(filepath.Join(home, ".local", "bin", name), "~/.local/bin")
	if name == "mise" {
		add(filepath.Join(home, ".cargo", "bin", name), "~/.cargo/bin")
		add(filepath.Join(home, ".local", "share", "mise", "bin", name), "~/.local/share/mise/bin")
	}
	if runtime.GOOS == "darwin" {
		add("/opt/homebrew/bin/"+name, "Homebrew")
		add("/usr/local/bin/"+name, "Homebrew")
	}
	add("/home/linuxbrew/.linuxbrew/bin/"+name, "Homebrew")
	add("/usr/local/bin/"+name, "common location")
	add("/usr/bin/"+name, "common location")
	return out
}

func resolveExecutable(path string) (string, error) {
	path = expandHome(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("is a directory")
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("is not executable")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func resolveDataDir(override string) string {
	if override != "" {
		return filepath.Clean(expandHome(override))
	}
	if env := os.Getenv("ASDF_DATA_DIR"); env != "" {
		return filepath.Clean(expandHome(env))
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".asdf")
}

func expandHome(path string) string {
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return home
}

func parseVersionLine(line string) string {
	match := versionPatternCLI.FindStringSubmatch(line)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

// BuildEnv returns the environment used for every asdf invocation.
func BuildEnv(binaryPath, dataDir string) []string {
	return BuildEnvForEngine(EngineASDF, binaryPath, dataDir)
}

// BuildEnvForEngine returns the environment used for every invocation of the
// selected version manager. GUI apps launched from a desktop environment often
// do not inherit the user's shell PATH, so the discovered binary directory is
// prepended explicitly.
func BuildEnvForEngine(engine, binaryPath, dataDir string) []string {
	if engine != EngineMise {
		engine = EngineASDF
	}
	binDir := filepath.Dir(binaryPath)

	base := os.Environ()
	env := make(map[string]string, len(base)+6)
	order := make([]string, 0, len(base)+6)
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := env[key]; !exists {
			order = append(order, key)
		}
		env[key] = value
	}
	set := func(key, value string) {
		if _, exists := env[key]; !exists {
			order = append(order, key)
		}
		env[key] = value
	}

	currentPath := env["PATH"]
	if currentPath == "" {
		currentPath = "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	}
	set("PATH", binDir+string(os.PathListSeparator)+currentPath)
	if engine == EngineMise {
		set("MISE_DATA_DIR", dataDir)
	} else {
		set("ASDF_DATA_DIR", dataDir)
		if parent := filepath.Dir(binDir); parent != "" && filepath.Base(binDir) == "bin" {
			set("ASDF_DIR", parent)
		}
	}
	set("NO_COLOR", "1")
	set("TERM", "dumb")
	set("GIT_TERMINAL_PROMPT", "0")
	set("GIT_ASKPASS", "true")
	if env["HOME"] == "" {
		set("HOME", homeDir())
	}

	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+env[key])
	}
	return result
}
