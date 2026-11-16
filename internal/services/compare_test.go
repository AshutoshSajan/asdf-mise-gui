package services

import (
	"strings"
	"testing"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
)

func TestBuildComparisons(t *testing.T) {
	tools := []ToolView{
		{Name: "node", Installed: []string{"22.0.0", "22.1.0"}, Current: []string{"22.0.0"}, CurrentInstalled: true},
		{Name: "python", Installed: []string{"3.11.0"}, Current: []string{"3.12.0"}},
		{Name: "ruby", Installed: []string{"3.3.0"}, Current: []string{"3.3.0"}, CurrentInstalled: true},
		{Name: "go", Installed: []string{"1.22.0"}},
		{Name: "terraform", Installed: []string{"1.8.0"}, Current: []string{"1.8.0"}, CurrentInstalled: true},
		{Name: "deno", Installed: []string{"1.40.0"}, Current: []string{"1.40.0"}, CurrentInstalled: true},
	}
	latest := []asdf.LatestVersion{
		{Tool: "node", Version: "22.1.0", Installed: true, Known: true},
		{Tool: "python", Version: "3.12.0", Known: true},
		{Tool: "ruby", Version: "3.3.0", Installed: true, Known: true},
		{Tool: "go", Version: "1.22.0", Installed: true, Known: true},
	}

	view := buildComparisons(tools, latest, false)
	byTool := map[string]VersionComparison{}
	for _, item := range view.Items {
		byTool[item.Tool] = item
	}

	if got := byTool["node"].Status; got != VersionStatusOutdated {
		t.Fatalf("node status = %q, want outdated", got)
	}
	if !strings.Contains(byTool["node"].Reason, "installed but the pin") {
		t.Fatalf("node reason should note the installed latest: %q", byTool["node"].Reason)
	}
	if !byTool["node"].LatestInstalled {
		t.Fatal("node latest should be reported as installed")
	}
	if got := byTool["python"].Status; got != VersionStatusMissing {
		t.Fatalf("python status = %q, want missing", got)
	}
	if got := byTool["ruby"].Status; got != VersionStatusCurrent {
		t.Fatalf("ruby status = %q, want current", got)
	}
	if got := byTool["go"].Status; got != VersionStatusUnpinned {
		t.Fatalf("go status = %q, want unpinned", got)
	}
	if got := byTool["terraform"].Status; got != VersionStatusUnknown {
		t.Fatalf("terraform status = %q, want unknown", got)
	}
	if got := byTool["deno"].Status; got != VersionStatusUnknown {
		t.Fatalf("deno status = %q, want unknown without outdated-only mode", got)
	}
	if view.OutdatedCount != 1 || view.MissingCount != 1 || view.UnpinnedCount != 1 {
		t.Fatalf("unexpected counts: %+v", view)
	}
}

func TestBuildComparisonsOutdatedOnlyTreatsOmittedAsCurrent(t *testing.T) {
	view := buildComparisons(
		[]ToolView{{Name: "deno", Installed: []string{"1.40.0"}, Current: []string{"1.40.0"}, CurrentInstalled: true}},
		nil,
		true,
	)
	if len(view.Items) != 1 || view.Items[0].Status != VersionStatusCurrent {
		t.Fatalf("unexpected comparison: %+v", view.Items)
	}
	if !strings.Contains(view.Items[0].Reason, "mise") {
		t.Fatalf("reason should mention mise: %q", view.Items[0].Reason)
	}
}

func TestBuildComparisonsSystemAndLatestKeyword(t *testing.T) {
	view := buildComparisons(
		[]ToolView{
			{Name: "python", Current: []string{"system"}, CurrentInstalled: true, System: true},
			{Name: "node", Installed: []string{"22.1.0"}, Current: []string{"latest"}, CurrentInstalled: true},
		},
		[]asdf.LatestVersion{
			{Tool: "python", Version: "3.13.0", Known: true},
			{Tool: "node", Version: "22.1.0", Installed: true, Known: true},
		},
		false,
	)
	for _, item := range view.Items {
		if item.Status != VersionStatusCurrent {
			t.Fatalf("%s status = %q, want current", item.Tool, item.Status)
		}
	}
}
