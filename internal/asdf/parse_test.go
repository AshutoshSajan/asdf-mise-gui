package asdf

import (
	"reflect"
	"testing"
)

func TestParsePluginList(t *testing.T) {
	output := "testplugin\t\t/tmp/plugins/testplugin\t05d42257f6639c3dbe1d82cc9823bc0b3b381906\n" +
		"nodejs\t\thttps://github.com/asdf-vm/asdf-nodejs.git\tabc123\n"
	plugins := ParsePluginList(output)
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name != "nodejs" || plugins[0].URL != "https://github.com/asdf-vm/asdf-nodejs.git" || plugins[0].Ref != "abc123" {
		t.Fatalf("unexpected first plugin: %+v", plugins[0])
	}
	if plugins[1].Name != "testplugin" || plugins[1].URL != "/tmp/plugins/testplugin" {
		t.Fatalf("unexpected second plugin: %+v", plugins[1])
	}
}

func TestParsePluginListEmpty(t *testing.T) {
	if plugins := ParsePluginList("No plugins installed\n"); len(plugins) != 0 {
		t.Fatalf("expected no plugins, got %+v", plugins)
	}
}

func TestParseInstalledVersions(t *testing.T) {
	output := "nodejs\n  20.11.0\n  22.0.0\nruby\n  3.2.2\n"
	tools := ParseInstalledVersions(output)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Tool != "nodejs" || !reflect.DeepEqual(tools[0].Versions, []string{"20.11.0", "22.0.0"}) {
		t.Fatalf("unexpected nodejs: %+v", tools[0])
	}
	if tools[1].Tool != "ruby" || !reflect.DeepEqual(tools[1].Versions, []string{"3.2.2"}) {
		t.Fatalf("unexpected ruby: %+v", tools[1])
	}
}

func TestParseCurrent(t *testing.T) {
	output := "testplugin      1.0.0 2.0.0     /tmp/opencode/asdf-project/.tool-versions false - Run `asdf install testplugin 1.0.0`\n" +
		"nodejs      22.0.0           /tmp/opencode/asdf-home/.tool-versions true\n" +
		"ruby      ______          ______          \n" +
		"python    system           /home/me/.tool-versions true\n"
	tools := ParseCurrent(output)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d: %+v", len(tools), tools)
	}
	byName := map[string]CurrentTool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	testplugin := byName["testplugin"]
	if !reflect.DeepEqual(testplugin.Versions, []string{"1.0.0", "2.0.0"}) {
		t.Fatalf("unexpected versions: %+v", testplugin.Versions)
	}
	if testplugin.Installed {
		t.Fatalf("expected testplugin to be missing")
	}
	if testplugin.Source != "/tmp/opencode/asdf-project/.tool-versions" {
		t.Fatalf("unexpected source: %q", testplugin.Source)
	}
	nodejs := byName["nodejs"]
	if !nodejs.Installed || nodejs.Source != "/tmp/opencode/asdf-home/.tool-versions" {
		t.Fatalf("unexpected nodejs: %+v", nodejs)
	}
	ruby := byName["ruby"]
	if len(ruby.Versions) != 0 || ruby.Source != "" || ruby.Installed {
		t.Fatalf("unexpected ruby: %+v", ruby)
	}
	python := byName["python"]
	if !python.System || !python.Installed {
		t.Fatalf("expected system to be treated as installed: %+v", python)
	}
}

func TestParseCurrentWithSpacesInPath(t *testing.T) {
	output := "nodejs      22.0.0           /home/me/My Projects/.tool-versions true\n"
	tools := ParseCurrent(output)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Source != "/home/me/My Projects/.tool-versions" {
		t.Fatalf("unexpected source: %q", tools[0].Source)
	}
	if !tools[0].Installed {
		t.Fatalf("expected installed")
	}
}

func TestParseCurrentNoPlugins(t *testing.T) {
	if tools := ParseCurrent("No plugins installed\n"); len(tools) != 0 {
		t.Fatalf("expected no tools, got %+v", tools)
	}
}

func TestParseLatestAll(t *testing.T) {
	output := "nodejs\t22.0.0\tinstalled\nruby\t3.3.0\tmissing\n"
	latest := ParseLatestAll(output)
	if len(latest) != 2 {
		t.Fatalf("expected 2, got %d", len(latest))
	}
	if latest[0].Tool != "nodejs" || latest[0].Version != "22.0.0" || !latest[0].Installed || !latest[0].Known {
		t.Fatalf("unexpected latest: %+v", latest[0])
	}
	if latest[1].Installed {
		t.Fatalf("expected ruby missing")
	}
}

func TestParseAvailableVersions(t *testing.T) {
	versions := ParseAvailableVersions("22.0.0\n20.11.0\n22.0.0\n")
	if !reflect.DeepEqual(versions, []string{"22.0.0", "20.11.0"}) {
		t.Fatalf("unexpected versions: %+v", versions)
	}
}

func TestParseAvailablePlugins(t *testing.T) {
	output := "1password-cli                               https://github.com/NeoHsu/asdf-1password-cli.git\n" +
		"R                                           https://github.com/asdf-community/asdf-r.git\n"
	plugins := ParseAvailablePlugins(output)
	if len(plugins) != 2 {
		t.Fatalf("expected 2, got %d", len(plugins))
	}
	if plugins[0].Name != "1password-cli" || plugins[0].URL == "" {
		t.Fatalf("unexpected plugin: %+v", plugins[0])
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"nodejs", "ruby", "test-plugin", "foo.bar", "foo_bar", "foo+bar"}
	for _, name := range valid {
		if err := ValidateName("tool", name); err != nil {
			t.Errorf("%q should be valid: %v", name, err)
		}
	}
	invalid := []string{"", "-rf", "--help", "a b", "../x", "a;b", "$(x)"}
	for _, name := range invalid {
		if err := ValidateName("tool", name); err == nil {
			t.Errorf("%q should be invalid", name)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	if err := ValidateVersion("--help"); err == nil {
		t.Fatal("flag-like version should be rejected")
	}
	if err := ValidateVersion("22.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateVersion("ref:abc123"); err != nil {
		t.Fatal(err)
	}
}

func TestQuoteCommand(t *testing.T) {
	got := QuoteCommand([]string{"install", "my tool", "1.0.0"})
	if got != "install 'my tool' 1.0.0" {
		t.Fatalf("unexpected quote: %s", got)
	}
}
