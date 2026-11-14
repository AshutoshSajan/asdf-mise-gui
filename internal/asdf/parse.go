package asdf

import (
	"regexp"
	"sort"
	"strings"
)

// Plugin is an installed asdf plugin.
type Plugin struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Ref  string `json:"ref"`
}

// ToolVersions is one tool with its installed versions.
type ToolVersions struct {
	Tool     string   `json:"tool"`
	Versions []string `json:"versions"`
}

// CurrentTool is a tool as resolved by `asdf current`.
type CurrentTool struct {
	Name      string   `json:"name"`
	Versions  []string `json:"versions"`
	Source    string   `json:"source"`
	Installed bool     `json:"installed"`
	System    bool     `json:"system"`
	Message   string   `json:"message,omitempty"`
}

// LatestVersion is a row from `asdf latest --all`.
type LatestVersion struct {
	Tool      string `json:"tool"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Known     bool   `json:"known"`
}

// AvailablePlugin is an entry from the asdf plugin registry.
type AvailablePlugin struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var multiSpace = regexp.MustCompile(`\s{2,}`)

// installedSuffix matches the final column of `asdf current`. The installed
// column can start immediately after the source path with a single space, so it
// must be peeled off before splitting the aligned columns.
var installedSuffix = regexp.MustCompile(`(?i)\s+(true|false)(\s+-\s+.*)?$`)

// ParsePluginList parses `asdf plugin list` (plain or with --urls/--refs).
func ParsePluginList(output string) []Plugin {
	var plugins []Plugin
	for _, line := range nonEmptyLines(output) {
		if strings.Contains(strings.ToLower(line), "no plugins installed") {
			continue
		}
		parts := splitColumns(line)
		plugin := Plugin{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			plugin.URL = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			plugin.Ref = strings.TrimSpace(parts[2])
		}
		if plugin.Name == "" || plugin.Name == "No plugins installed" {
			continue
		}
		plugins = append(plugins, plugin)
	}
	sort.Slice(plugins, func(i, j int) bool { return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name) })
	return plugins
}

// ParsePluginNames parses the newline-separated output of `asdf plugin list`.
func ParsePluginNames(output string) []string {
	plugins := ParsePluginList(output)
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		names = append(names, plugin.Name)
	}
	return names
}

// ParseInstalledVersions parses `asdf list`, which prints a tool name followed
// by two-space indented versions.
func ParseInstalledVersions(output string) []ToolVersions {
	var tools []ToolVersions
	index := map[string]int{}
	for _, line := range strings.Split(normalizeNewlines(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), "no versions installed") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if line == trimmed {
			fields := strings.Fields(trimmed)
			if len(fields) == 0 {
				continue
			}
			name := fields[0]
			if _, ok := index[name]; !ok {
				index[name] = len(tools)
				tools = append(tools, ToolVersions{Tool: name})
			}
			if len(fields) > 1 {
				tools[index[name]].Versions = append(tools[index[name]].Versions, fields[1:]...)
			}
			continue
		}
		if len(tools) == 0 {
			continue
		}
		tools[len(tools)-1].Versions = append(tools[len(tools)-1].Versions, strings.Fields(trimmed)...)
	}
	sort.Slice(tools, func(i, j int) bool { return strings.ToLower(tools[i].Tool) < strings.ToLower(tools[j].Tool) })
	return tools
}

// ParseCurrent parses `asdf current --no-header`.
func ParseCurrent(output string) []CurrentTool {
	var tools []CurrentTool
	for _, line := range nonEmptyLines(output) {
		if strings.Contains(strings.ToLower(line), "no plugins installed") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "Name ") {
			continue
		}
		line = strings.TrimRight(line, " \t\r")

		installed := false
		message := ""
		if match := installedSuffix.FindStringSubmatchIndex(line); match != nil {
			message = strings.TrimSpace(line[match[0]:])
			installed = strings.EqualFold(line[match[2]:match[3]], "true")
			line = strings.TrimRight(line[:match[0]], " \t")
		}

		parts := splitColumns(line)
		if len(parts) == 0 {
			continue
		}
		if strings.EqualFold(parts[0], "Name") {
			continue
		}

		item := CurrentTool{Name: strings.TrimSpace(parts[0]), Installed: installed, Message: message}
		if len(parts) > 1 {
			for _, version := range strings.Fields(parts[1]) {
				if version == "______" {
					continue
				}
				item.Versions = append(item.Versions, version)
			}
		}
		if len(parts) > 2 {
			item.Source = cleanPlaceholder(strings.Join(parts[2:], " "))
		}

		for _, version := range item.Versions {
			if version == "system" {
				item.System = true
				item.Installed = true
			}
		}
		if item.Name == "" {
			continue
		}
		tools = append(tools, item)
	}
	sort.Slice(tools, func(i, j int) bool { return strings.ToLower(tools[i].Name) < strings.ToLower(tools[j].Name) })
	return tools
}

// ParseLatestAll parses `asdf latest --all` (tab separated).
func ParseLatestAll(output string) []LatestVersion {
	var latest []LatestVersion
	for _, line := range nonEmptyLines(output) {
		parts := splitColumns(line)
		if len(parts) == 0 {
			continue
		}
		item := LatestVersion{Tool: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			item.Version = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			status := strings.ToLower(strings.TrimSpace(parts[2]))
			item.Known = true
			item.Installed = strings.HasPrefix(status, "installed")
		}
		if item.Tool == "" {
			continue
		}
		latest = append(latest, item)
	}
	return latest
}

// ParseAvailableVersions parses `asdf list all <tool>`.
func ParseAvailableVersions(output string) []string {
	seen := map[string]bool{}
	var versions []string
	for _, line := range nonEmptyLines(output) {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "no versions") || strings.Contains(lower, "unknown") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if seen[field] {
				continue
			}
			seen[field] = true
			versions = append(versions, field)
		}
	}
	return versions
}

// ParseAvailablePlugins parses `asdf plugin list all`.
func ParseAvailablePlugins(output string) []AvailablePlugin {
	var plugins []AvailablePlugin
	for _, line := range nonEmptyLines(output) {
		parts := splitColumns(line)
		if len(parts) == 0 {
			continue
		}
		plugin := AvailablePlugin{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			plugin.URL = strings.TrimSpace(parts[1])
		}
		if plugin.Name == "" {
			continue
		}
		plugins = append(plugins, plugin)
	}
	sort.Slice(plugins, func(i, j int) bool { return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name) })
	return plugins
}

func splitColumns(line string) []string {
	line = strings.TrimRight(line, " \t\r")
	if strings.Contains(line, "\t") {
		parts := regexp.MustCompile(`\t+`).Split(line, -1)
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			out = append(out, part)
		}
		return out
	}
	if multiSpace.MatchString(line) {
		return multiSpace.Split(line, -1)
	}
	return strings.Fields(line)
}

func cleanPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if value == "______" {
		return ""
	}
	return value
}

func nonEmptyLines(output string) []string {
	output = normalizeNewlines(output)
	lines := strings.Split(output, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, strings.TrimRight(line, " \t\r"))
	}
	return out
}

func normalizeNewlines(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}
