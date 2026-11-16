package asdf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// mise is a separate version manager, not a drop-in replacement for asdf. It
// reads .tool-versions for compatibility but writes mise.toml by default and
// exposes JSON output for most read operations, which is what this package
// parses instead of the human-oriented tables.

// MiseSource identifies the config file that requested a tool version.
type MiseSource struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// MiseToolVersion is one row of `mise ls --json`.
type MiseToolVersion struct {
	Version          string      `json:"version"`
	RequestedVersion string      `json:"requested_version"`
	InstallPath      string      `json:"install_path"`
	Source           *MiseSource `json:"source"`
	Installed        bool        `json:"installed"`
	Active           bool        `json:"active"`
}

// Requested returns the version as written in config, falling back to the
// resolved version for installed-only rows.
func (m MiseToolVersion) Requested() string {
	if strings.TrimSpace(m.RequestedVersion) != "" {
		return m.RequestedVersion
	}
	return m.Version
}

// MiseList is the top-level map returned by `mise ls --json`.
type MiseList map[string][]MiseToolVersion

// MiseConfigFile is one entry of `mise config ls --json`.
type MiseConfigFile struct {
	Path     string   `json:"path"`
	Tools    []string `json:"tools"`
	Override bool     `json:"override"`
}

// MiseOutdated is one entry of `mise outdated --json`.
type MiseOutdated struct {
	Name      string      `json:"name"`
	Requested string      `json:"requested"`
	Current   string      `json:"current"`
	Bump      string      `json:"bump"`
	Latest    string      `json:"latest"`
	Source    *MiseSource `json:"source"`
}

// detectMise locates and validates the mise binary.
func detectMise(binaryOverride, dataDirOverride string) Detection {
	d := Detection{
		Engine:     EngineMise,
		RunnerMode: RunnerNative,
		HomeDir:    homeDir(),
		ConfigDir:  MiseConfigDir(),
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	candidates := candidateBinaries("mise", binaryOverride)
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
		d.Error = "mise was not found. Install it, or set the binary path in Settings."
		if firstErr != "" {
			d.Error = firstErr
		}
		return d
	}

	if engineBinaryBase(d.BinaryPath) == EngineASDF {
		d.Error = fmt.Sprintf("%s is an asdf binary, not mise. Set the engine to asdf or choose the mise binary.", d.BinaryPath)
		return d
	}

	d.DataDir = resolveMiseDataDir(dataDirOverride)
	if d.DataDir == "" {
		d.Error = "could not determine the mise data directory"
		return d
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, d.BinaryPath, "--version")
	cmd.Env = BuildEnvForEngine(EngineMise, d.BinaryPath, d.DataDir)
	cmd.Dir = homeDir()
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		d.Error = text
		if d.Error == "" {
			d.Error = err.Error()
		}
		d.Error = "mise --version failed: " + d.Error
		return d
	}

	d.Version = parseMiseVersion(text)
	d.Found = d.Version != ""
	if !d.Found {
		d.Error = fmt.Sprintf("mise --version did not report a mise version: %q", firstOutputLine(text))
	}
	return d
}

// miseVersionPattern matches mise's calendar versioning (2026.11.4). asdf uses
// semver (0.20.2), so this also keeps an asdf binary from being accepted when
// the engine is mise.
var miseVersionPattern = regexp.MustCompile(`^\d{4}\.\d+\.\d+`)

func parseMiseVersion(output string) string {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		candidate := fields[0]
		if strings.EqualFold(candidate, "mise") && len(fields) > 1 {
			candidate = fields[1]
		}
		candidate = strings.TrimPrefix(candidate, "v")
		if miseVersionPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func resolveMiseDataDir(override string) string {
	if override != "" {
		return filepath.Clean(expandHome(override))
	}
	if env := os.Getenv("MISE_DATA_DIR"); env != "" {
		return filepath.Clean(expandHome(env))
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "mise")
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "mise")
}

// MiseConfigDir returns the global mise configuration directory.
func MiseConfigDir() string {
	if env := os.Getenv("MISE_CONFIG_DIR"); env != "" {
		return filepath.Clean(expandHome(env))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mise")
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "mise")
}

// MiseGlobalConfigPath is the file mise writes for global tool requests.
func MiseGlobalConfigPath() string {
	dir := MiseConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// MiseDataDir returns the data directory for the detected mise binary.
func MiseDataDir() string {
	return resolveMiseDataDir("")
}

// MiseConfigNames are the project files mise reads, in the order the GUI uses
// when choosing a single write target.
var MiseConfigNames = []string{"mise.toml", "mise.local.toml", ".mise.toml", ToolVersionsFilename}

// FindMiseConfig walks up from start and returns the nearest mise config file.
func FindMiseConfig(start string) (string, bool) {
	dir := start
	for {
		for _, name := range MiseConfigNames {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// MiseProjectConfigPath returns the file the GUI should read for a project.
// Existing files keep their format; new projects get mise's canonical
// mise.toml.
func MiseProjectConfigPath(dir string) (path string, exists bool) {
	if local, found := MiseLocalConfigPath(dir); found {
		return local, true
	}
	nearest, found := FindMiseConfig(dir)
	if found {
		return nearest, true
	}
	return filepath.Join(dir, "mise.toml"), false
}

// MiseLocalConfigPath returns a config file directly inside dir, if any.
func MiseLocalConfigPath(dir string) (string, bool) {
	for _, name := range MiseConfigNames {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// MiseEffectiveConfigPath returns the config file that applies in dir: a
// project-local file first, then the nearest parent config.
func MiseEffectiveConfigPath(dir string) (string, bool) {
	if local, found := MiseLocalConfigPath(dir); found {
		return local, true
	}
	return FindMiseConfig(dir)
}

// MiseWriteTarget is where a project save should write. Existing local files
// keep their format; otherwise a new mise.toml is created in the project.
func MiseWriteTarget(dir string) string {
	if local, found := MiseLocalConfigPath(dir); found {
		return local
	}
	return filepath.Join(dir, "mise.toml")
}

// ParseMiseListJSON parses `mise ls --json`.
func ParseMiseListJSON(output string) (MiseList, error) {
	var list MiseList
	if strings.TrimSpace(output) == "" {
		return MiseList{}, nil
	}
	if err := json.Unmarshal([]byte(output), &list); err != nil {
		return nil, fmt.Errorf("parse mise ls --json: %w", err)
	}
	if list == nil {
		list = MiseList{}
	}
	return list, nil
}

// MiseInstalledTools converts `mise ls --json` into the tool/version shape the
// rest of the GUI already understands.
func MiseInstalledTools(list MiseList) []ToolVersions {
	tools := make([]ToolVersions, 0, len(list))
	for name, rows := range list {
		var versions []string
		seen := map[string]bool{}
		for _, row := range rows {
			if !row.Installed || row.Version == "" || seen[row.Version] {
				continue
			}
			seen[row.Version] = true
			versions = append(versions, row.Version)
		}
		if len(versions) == 0 {
			continue
		}
		tools = append(tools, ToolVersions{Tool: name, Versions: versions})
	}
	sort.Slice(tools, func(i, j int) bool { return strings.ToLower(tools[i].Tool) < strings.ToLower(tools[j].Tool) })
	return tools
}

// MiseCurrentTools converts `mise ls --json` into active/config-requested
// tools. Rows with a source are included even when mise marks them inactive:
// a missing version requested by a project config is exactly what the UI needs
// to show as "missing".
func MiseCurrentTools(list MiseList) []CurrentTool {
	tools := make([]CurrentTool, 0, len(list))
	for name, rows := range list {
		var chosen []MiseToolVersion
		for _, row := range rows {
			if row.Active {
				chosen = append(chosen, row)
			}
		}
		if len(chosen) == 0 {
			for _, row := range rows {
				if row.Source != nil && row.Source.Path != "" {
					chosen = append(chosen, row)
				}
			}
		}
		if len(chosen) == 0 {
			continue
		}

		item := CurrentTool{Name: name, Installed: true}
		seen := map[string]bool{}
		for _, row := range chosen {
			version := row.Requested()
			if version != "" && !seen[version] {
				seen[version] = true
				item.Versions = append(item.Versions, version)
			}
			if !row.Installed {
				item.Installed = false
			}
			if row.Source != nil && row.Source.Path != "" && item.Source == "" {
				item.Source = row.Source.Path
			}
			if version == "system" {
				item.System = true
				item.Installed = true
			}
		}
		if !item.Installed && len(item.Versions) > 0 {
			item.Message = "not installed"
		}
		if len(item.Versions) == 0 {
			continue
		}
		tools = append(tools, item)
	}
	sort.Slice(tools, func(i, j int) bool { return strings.ToLower(tools[i].Name) < strings.ToLower(tools[j].Name) })
	return tools
}

// MiseEntriesForSource returns the requests that came from one config file.
func MiseEntriesForSource(list MiseList, path string) []ToolVersionEntry {
	if path == "" {
		return nil
	}
	path = filepath.Clean(path)
	var entries []ToolVersionEntry
	for name, rows := range list {
		var versions []string
		seen := map[string]bool{}
		for _, row := range rows {
			if row.Source == nil || filepath.Clean(row.Source.Path) != path {
				continue
			}
			version := row.Requested()
			if version == "" || seen[version] {
				continue
			}
			seen[version] = true
			versions = append(versions, version)
		}
		if len(versions) == 0 {
			continue
		}
		entries = append(entries, ToolVersionEntry{Tool: name, Versions: versions})
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Tool) < strings.ToLower(entries[j].Tool) })
	return entries
}

// ParseMisePluginList parses `mise plugins ls -u`.
func ParseMisePluginList(output string) []Plugin {
	var plugins []Plugin
	for _, line := range nonEmptyLines(output) {
		if strings.Contains(strings.ToLower(line), "no plugins installed") {
			continue
		}
		parts := splitColumns(line)
		if len(parts) == 0 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "*"))
		if name == "" {
			continue
		}
		plugin := Plugin{Name: name}
		if len(parts) > 1 {
			plugin.URL = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			plugin.Ref = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 && plugin.Ref == "" {
			plugin.Ref = strings.TrimSpace(parts[3])
		}
		plugins = append(plugins, plugin)
	}
	sort.Slice(plugins, func(i, j int) bool { return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name) })
	return plugins
}

// ParseMisePluginCatalog parses `mise plugins ls-remote -u`, which is the mise
// equivalent of asdf's plugin index.
func ParseMisePluginCatalog(output string) []AvailablePlugin {
	// mise lists the same tool once per backend, so "ag" appears for both the
	// asdf and vfox registries. Keep the first entry per name: installing uses
	// the name, and duplicate rows would collide as UI list keys.
	seen := map[string]bool{}
	plugins := make([]AvailablePlugin, 0, 128)
	for _, line := range nonEmptyLines(output) {
		parts := splitColumns(line)
		if len(parts) == 0 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "*"))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		plugin := AvailablePlugin{Name: name}
		if len(parts) > 1 {
			plugin.URL = strings.TrimSpace(parts[1])
		}
		plugins = append(plugins, plugin)
	}
	sort.Slice(plugins, func(i, j int) bool { return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name) })
	return plugins
}

// ParseMiseAvailableVersionsJSON parses `mise ls-remote TOOL --json`.
func ParseMiseAvailableVersionsJSON(output string) ([]string, error) {
	if strings.TrimSpace(output) == "" {
		return []string{}, nil
	}
	var rows []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(output), &rows); err == nil {
		versions := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.Version != "" {
				versions = append(versions, row.Version)
			}
		}
		return versions, nil
	}
	var plain []string
	if err := json.Unmarshal([]byte(output), &plain); err == nil {
		return plain, nil
	}
	return nil, fmt.Errorf("parse mise ls-remote --json: unexpected output")
}

// ParseMiseOutdatedJSON parses `mise outdated --json` into latest-version rows.
func ParseMiseOutdatedJSON(output string) ([]LatestVersion, error) {
	if strings.TrimSpace(output) == "" {
		return []LatestVersion{}, nil
	}
	var rows map[string]MiseOutdated
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return nil, fmt.Errorf("parse mise outdated --json: %w", err)
	}
	latest := make([]LatestVersion, 0, len(rows))
	for name, row := range rows {
		version := row.Latest
		if version == "" {
			version = row.Bump
		}
		if version == "" {
			continue
		}
		if name == "" {
			name = row.Name
		}
		latest = append(latest, LatestVersion{
			Tool:      name,
			Version:   version,
			Installed: row.Current == version,
			Known:     true,
		})
	}
	sort.Slice(latest, func(i, j int) bool { return strings.ToLower(latest[i].Tool) < strings.ToLower(latest[j].Tool) })
	return latest, nil
}

// ParseMiseConfigListJSON parses `mise config ls --json`.
func ParseMiseConfigListJSON(output string) ([]MiseConfigFile, error) {
	if strings.TrimSpace(output) == "" {
		return []MiseConfigFile{}, nil
	}
	var files []MiseConfigFile
	if err := json.Unmarshal([]byte(output), &files); err != nil {
		return nil, fmt.Errorf("parse mise config ls --json: %w", err)
	}
	return files, nil
}

// ReadsMiseToolVersions reports whether a config file uses asdf's plain text
// format rather than TOML.
func ReadsMiseToolVersions(path string) bool {
	return filepath.Base(path) == ToolVersionsFilename
}

// ReadMiseToolsTOML extracts simple [tools] entries from a mise TOML file. It
// is a fallback for display when mise's JSON output has no rows for a file;
// writes always go through the mise CLI so the real parser stays authoritative.
func ReadMiseToolsTOML(path string) ([]ToolVersionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []ToolVersionEntry
	index := map[string]int{}
	inTools := false
	tableTool := ""
	add := func(tool string, versions []string) {
		tool = strings.TrimSpace(unquoteTOML(tool))
		if tool == "" {
			return
		}
		if existing, ok := index[tool]; ok {
			if len(versions) > 0 {
				entries[existing].Versions = versions
			}
			return
		}
		index[tool] = len(entries)
		entries = append(entries, ToolVersionEntry{Tool: tool, Versions: versions})
	}

	for _, raw := range strings.Split(normalizeNewlines(string(data)), "\n") {
		line := strings.TrimSpace(stripTOMLComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(strings.Trim(line, "[]"))
			switch {
			case section == "tools":
				inTools = true
				tableTool = ""
			case strings.HasPrefix(section, "tools."):
				inTools = true
				tableTool = strings.TrimSpace(strings.TrimPrefix(section, "tools."))
				add(tableTool, nil)
			default:
				inTools = false
				tableTool = ""
			}
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(unquoteTOML(strings.TrimSpace(key)))
		if tableTool != "" {
			if key == "version" {
				add(tableTool, parseTOMLVersions(value))
			}
			continue
		}
		add(key, parseTOMLVersions(value))
	}

	kept := entries[:0]
	for _, entry := range entries {
		if len(entry.Versions) > 0 {
			kept = append(kept, entry)
		}
	}
	entries = kept
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Tool) < strings.ToLower(entries[j].Tool) })
	return entries, nil
}

func stripTOMLComment(line string) string {
	inString := false
	escaped := false
	for index, r := range line {
		if escaped {
			escaped = false
			continue
		}
		switch r {
		case '\\':
			if inString {
				escaped = true
			}
		case '"':
			inString = !inString
		case '#':
			if !inString {
				return line[:index]
			}
		}
	}
	return line
}

func unquoteTOML(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseTOMLVersions(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	var versions []string
	for _, part := range strings.Split(value, ",") {
		item := unquoteTOML(part)
		if item != "" {
			versions = append(versions, item)
		}
	}
	return versions
}
