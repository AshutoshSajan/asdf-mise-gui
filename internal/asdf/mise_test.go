package asdf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const miseListFixture = `{
  "go": [
    {
      "version": "1.27.4",
      "requested_version": "1.27.4",
      "install_path": "/data/go/1.27.4",
      "source": null,
      "installed": true,
      "active": false
    }
  ],
  "node": [
    {
      "version": "24.20.0",
      "requested_version": "24",
      "install_path": "/data/node/24.20.0",
      "source": { "type": "mise.toml", "path": "/proj/mise.toml" },
      "installed": true,
      "active": true
    },
    {
      "version": "26.13.0",
      "requested_version": "26",
      "install_path": "/data/node/26.13.0",
      "source": { "type": "tool-versions", "path": "/proj/.tool-versions" },
      "installed": true,
      "active": false
    }
  ],
  "python": [
    {
      "version": "3.12.8",
      "requested_version": "3.12.8",
      "install_path": "",
      "source": { "type": "tool-versions", "path": "/proj/.tool-versions" },
      "installed": false,
      "active": false
    }
  ],
  "ruby": [
    {
      "version": "system",
      "requested_version": "system",
      "install_path": "",
      "source": null,
      "installed": false,
      "active": true
    }
  ]
}`

func TestParseMiseListJSON(t *testing.T) {
	list, err := ParseMiseListJSON(miseListFixture)
	if err != nil {
		t.Fatalf("ParseMiseListJSON: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(list))
	}
	node := list["node"]
	if len(node) != 2 {
		t.Fatalf("expected 2 node rows, got %d", len(node))
	}
	if node[0].Requested() != "24" || !node[0].Installed || !node[0].Active {
		t.Fatalf("unexpected node row: %+v", node[0])
	}
	if node[0].Source == nil || node[0].Source.Path != "/proj/mise.toml" {
		t.Fatalf("unexpected node source: %+v", node[0].Source)
	}
	if node[1].Requested() != "26" || node[1].Active {
		t.Fatalf("unexpected inactive node row: %+v", node[1])
	}
	if python := list["python"]; len(python) != 1 || python[0].Installed {
		t.Fatalf("unexpected python rows: %+v", python)
	}
}

func TestParseMiseListJSONEmpty(t *testing.T) {
	list, err := ParseMiseListJSON("  \n")
	if err != nil {
		t.Fatalf("expected empty output to parse: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestParseMiseListJSONInvalid(t *testing.T) {
	if _, err := ParseMiseListJSON("{not json"); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestMiseInstalledTools(t *testing.T) {
	list, err := ParseMiseListJSON(miseListFixture)
	if err != nil {
		t.Fatal(err)
	}
	tools := MiseInstalledTools(list)
	want := []ToolVersions{
		{Tool: "go", Versions: []string{"1.27.4"}},
		{Tool: "node", Versions: []string{"24.20.0", "26.13.0"}},
	}
	if !reflect.DeepEqual(tools, want) {
		t.Fatalf("unexpected installed tools:\n got %+v\nwant %+v", tools, want)
	}
}

func TestMiseInstalledToolsDeduplicates(t *testing.T) {
	list := MiseList{
		"node": {
			{Version: "24.20.0", Installed: true},
			{Version: "24.20.0", Installed: true},
			{Version: "26.13.0", Installed: false},
		},
	}
	tools := MiseInstalledTools(list)
	want := []ToolVersions{{Tool: "node", Versions: []string{"24.20.0"}}}
	if !reflect.DeepEqual(tools, want) {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestMiseCurrentTools(t *testing.T) {
	list, err := ParseMiseListJSON(miseListFixture)
	if err != nil {
		t.Fatal(err)
	}
	tools := MiseCurrentTools(list)
	if len(tools) != 3 {
		t.Fatalf("expected 3 current tools, got %d: %+v", len(tools), tools)
	}

	byName := map[string]CurrentTool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	node := byName["node"]
	if !reflect.DeepEqual(node.Versions, []string{"24"}) {
		t.Fatalf("unexpected node versions: %+v", node.Versions)
	}
	if !node.Installed || node.Source != "/proj/mise.toml" {
		t.Fatalf("unexpected node resolution: %+v", node)
	}

	python := byName["python"]
	if !reflect.DeepEqual(python.Versions, []string{"3.12.8"}) {
		t.Fatalf("unexpected python versions: %+v", python.Versions)
	}
	if python.Installed || python.Message != "not installed" {
		t.Fatalf("unexpected python resolution: %+v", python)
	}
	if python.Source != "/proj/.tool-versions" {
		t.Fatalf("unexpected python source: %q", python.Source)
	}

	ruby := byName["ruby"]
	if !ruby.System || !ruby.Installed {
		t.Fatalf("unexpected system resolution: %+v", ruby)
	}
}

func TestMiseCurrentToolsIgnoresInactiveUnconfiguredRows(t *testing.T) {
	list := MiseList{
		"go": {{Version: "1.27.4", Installed: true}},
	}
	if tools := MiseCurrentTools(list); len(tools) != 0 {
		t.Fatalf("expected unconfigured inactive rows to be ignored: %+v", tools)
	}
}

func TestMiseEntriesForSource(t *testing.T) {
	list, err := ParseMiseListJSON(miseListFixture)
	if err != nil {
		t.Fatal(err)
	}
	entries := MiseEntriesForSource(list, "/proj/../proj/mise.toml")
	want := []ToolVersionEntry{{Tool: "node", Versions: []string{"24"}}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected entries:\n got %+v\nwant %+v", entries, want)
	}

	if entries := MiseEntriesForSource(list, ""); entries != nil {
		t.Fatalf("expected nil for empty path, got %+v", entries)
	}
	if entries := MiseEntriesForSource(list, "/elsewhere/mise.toml"); len(entries) != 0 {
		t.Fatalf("expected no entries for unrelated path, got %+v", entries)
	}
}

func TestMiseEntriesForSourceDeduplicatesVersions(t *testing.T) {
	list := MiseList{
		"node": {
			{Version: "24.20.0", RequestedVersion: "24", Source: &MiseSource{Path: "/proj/mise.toml"}},
			{Version: "24.20.0", RequestedVersion: "24", Source: &MiseSource{Path: "/proj/mise.toml"}},
			{Version: "26.13.0", RequestedVersion: "26", Source: &MiseSource{Path: "/other/mise.toml"}},
		},
	}
	entries := MiseEntriesForSource(list, "/proj/mise.toml")
	want := []ToolVersionEntry{{Tool: "node", Versions: []string{"24"}}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestParseMisePluginList(t *testing.T) {
	output := "node\t\thttps://github.com/asdf-vm/asdf-nodejs.git\tabc123\n" +
		"* python\t\thttps://github.com/asdf-community/asdf-python.git\tdef456\n" +
		"No plugins installed\n"
	plugins := ParseMisePluginList(output)
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d: %+v", len(plugins), plugins)
	}
	if plugins[0].Name != "node" || plugins[0].URL != "https://github.com/asdf-vm/asdf-nodejs.git" || plugins[0].Ref != "abc123" {
		t.Fatalf("unexpected first plugin: %+v", plugins[0])
	}
	if plugins[1].Name != "python" || plugins[1].URL != "https://github.com/asdf-community/asdf-python.git" {
		t.Fatalf("unexpected second plugin: %+v", plugins[1])
	}
}

func TestParseMisePluginListEmpty(t *testing.T) {
	if plugins := ParseMisePluginList(""); len(plugins) != 0 {
		t.Fatalf("expected no plugins, got %+v", plugins)
	}
}

func TestParseMisePluginCatalog(t *testing.T) {
	output := "node                          asdf:mise-plugins/mise-node\n" +
		"1password                     vfox:jdx/vfox-1password\n"
	plugins := ParseMisePluginCatalog(output)
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d: %+v", len(plugins), plugins)
	}
	if plugins[0].Name != "1password" || plugins[0].URL != "vfox:jdx/vfox-1password" {
		t.Fatalf("unexpected first plugin: %+v", plugins[0])
	}
	if plugins[1].Name != "node" || plugins[1].URL != "asdf:mise-plugins/mise-node" {
		t.Fatalf("unexpected second plugin: %+v", plugins[1])
	}
}

func TestParseMisePluginCatalogDeduplicatesBackends(t *testing.T) {
	output := "ag                            asdf:asdf-community/asdf-silver-searcher\n" +
		"ag                            vfox:jdx/vfox-ag\n" +
		"node                          asdf:mise-plugins/mise-node\n"
	plugins := ParseMisePluginCatalog(output)
	if len(plugins) != 2 {
		t.Fatalf("expected 2 deduplicated plugins, got %d: %+v", len(plugins), plugins)
	}
	if plugins[0].Name != "ag" || plugins[0].URL != "asdf:asdf-community/asdf-silver-searcher" {
		t.Fatalf("unexpected first plugin: %+v", plugins[0])
	}
	if plugins[1].Name != "node" {
		t.Fatalf("unexpected second plugin: %+v", plugins[1])
	}
}

func TestParseMiseAvailableVersionsJSON(t *testing.T) {
	output := `[{"version":"24.20.0","created_at":"2026-01-01T00:00:00Z"},{"version":"26.13.0","created_at":"2026-02-01T00:00:00Z"}]`
	versions, err := ParseMiseAvailableVersionsJSON(output)
	if err != nil {
		t.Fatalf("ParseMiseAvailableVersionsJSON: %v", err)
	}
	if !reflect.DeepEqual(versions, []string{"24.20.0", "26.13.0"}) {
		t.Fatalf("unexpected versions: %+v", versions)
	}
	if versions, err := ParseMiseAvailableVersionsJSON(""); err != nil || len(versions) != 0 {
		t.Fatalf("unexpected empty result: %+v, %v", versions, err)
	}
	if _, err := ParseMiseAvailableVersionsJSON("{broken"); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestParseMiseOutdatedJSON(t *testing.T) {
	output := `{
	  "fzf": {
	    "name": "fzf",
	    "requested": "latest",
	    "current": "0.74.5",
	    "bump": null,
	    "latest": "0.75.0",
	    "source": {"type": "mise.toml", "path": "/home/me/.config/mise/config.toml"}
	  },
	  "node": {
	    "name": "node",
	    "requested": "24",
	    "current": "24.20.0",
	    "bump": "24.21.0",
	    "latest": "26.13.0",
	    "source": {"type": "mise.toml", "path": "/proj/mise.toml"}
	  },
	  "python": {
	    "name": "python",
	    "requested": "3.12",
	    "current": "3.12.8",
	    "bump": null,
	    "latest": null
	  }
	}`
	latest, err := ParseMiseOutdatedJSON(output)
	if err != nil {
		t.Fatalf("ParseMiseOutdatedJSON: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(latest), latest)
	}
	if latest[0].Tool != "fzf" || latest[0].Version != "0.75.0" || latest[0].Installed {
		t.Fatalf("unexpected fzf row: %+v", latest[0])
	}
	if latest[1].Tool != "node" || latest[1].Version != "26.13.0" || !latest[1].Known {
		t.Fatalf("unexpected node row: %+v", latest[1])
	}
	if latest, err := ParseMiseOutdatedJSON(""); err != nil || len(latest) != 0 {
		t.Fatalf("unexpected empty result: %+v, %v", latest, err)
	}
	if _, err := ParseMiseOutdatedJSON("[]"); err == nil {
		t.Fatal("expected array-shaped output to fail")
	}
}

func TestParseMiseConfigListJSON(t *testing.T) {
	output := `[
	  {"path": "/proj/mise.toml", "tools": ["node", "python"], "override": false},
	  {"path": "/home/me/.config/mise/config.toml", "tools": ["go"], "override": true}
	]`
	files, err := ParseMiseConfigListJSON(output)
	if err != nil {
		t.Fatalf("ParseMiseConfigListJSON: %v", err)
	}
	if len(files) != 2 || files[0].Path != "/proj/mise.toml" || !reflect.DeepEqual(files[0].Tools, []string{"node", "python"}) {
		t.Fatalf("unexpected config files: %+v", files)
	}
	if !files[1].Override {
		t.Fatalf("expected second config to be an override: %+v", files[1])
	}
	if files, err := ParseMiseConfigListJSON(""); err != nil || len(files) != 0 {
		t.Fatalf("unexpected empty result: %+v, %v", files, err)
	}
	if _, err := ParseMiseConfigListJSON("{}"); err == nil {
		t.Fatal("expected object-shaped output to fail")
	}
}

func TestReadMiseToolsTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mise.toml")
	content := `# project tools
[tools]
node = "24.20.0"
python = ["3.12.8", "3.13.0"] # fallback list
ruby = '3.3.0'

[env]
FOO = "bar"

[tools.go]
version = "1.27.4"

[tools.rust]
version = "1.92.0"
components = ["clippy"]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadMiseToolsTOML(path)
	if err != nil {
		t.Fatalf("ReadMiseToolsTOML: %v", err)
	}
	want := []ToolVersionEntry{
		{Tool: "go", Versions: []string{"1.27.4"}},
		{Tool: "node", Versions: []string{"24.20.0"}},
		{Tool: "python", Versions: []string{"3.12.8", "3.13.0"}},
		{Tool: "ruby", Versions: []string{"3.3.0"}},
		{Tool: "rust", Versions: []string{"1.92.0"}},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected entries:\n got %+v\nwant %+v", entries, want)
	}
}

func TestReadMiseToolsTOMLMissingFile(t *testing.T) {
	entries, err := ReadMiseToolsTOML(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("expected missing file to be ignored: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected no entries, got %+v", entries)
	}
}

func TestMiseWriteTarget(t *testing.T) {
	dir := t.TempDir()
	if got := MiseWriteTarget(dir); got != filepath.Join(dir, "mise.toml") {
		t.Fatalf("expected new mise.toml, got %q", got)
	}
	toolVersions := filepath.Join(dir, ToolVersionsFilename)
	if err := os.WriteFile(toolVersions, []byte("nodejs 22.14.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := MiseWriteTarget(dir); got != toolVersions {
		t.Fatalf("expected existing .tool-versions, got %q", got)
	}
}
