package services

import (
	"reflect"
	"testing"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
)

func TestMissingTools(t *testing.T) {
	current := []asdf.CurrentTool{
		{Name: "nodejs", Versions: []string{"22.14.0"}, Installed: true},
		{Name: "python", Versions: []string{"3.12.8"}, Installed: false},
		{Name: "ruby"}, // unpinned plugin: no resolved version
		{Name: "terraform", Versions: []string{}},
		{Name: "golang", Versions: []string{"system"}, Installed: true, System: true},
		{Name: "python", Versions: []string{"3.11.9"}, Installed: false},
		{Name: "elixir", Versions: []string{"1.18.0"}, Installed: false},
	}
	want := []string{"elixir", "python"}
	if got := missingTools(current); !reflect.DeepEqual(got, want) {
		t.Fatalf("missingTools() = %v, want %v", got, want)
	}
}
