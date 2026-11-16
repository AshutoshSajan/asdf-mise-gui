package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
)

// Version comparison statuses used by the Tools screen. Missing wins over
// outdated because an uninstalled pin cannot run at all.
const (
	VersionStatusCurrent  = "current"
	VersionStatusOutdated = "outdated"
	VersionStatusMissing  = "missing"
	VersionStatusUnpinned = "unpinned"
	VersionStatusUnknown  = "unknown"
)

type VersionComparison struct {
	Tool            string   `json:"tool"`
	Pinned          []string `json:"pinned"`
	Installed       []string `json:"installed"`
	Latest          string   `json:"latest"`
	LatestInstalled bool     `json:"latestInstalled"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason"`
}

type VersionComparisonView struct {
	Items         []VersionComparison `json:"items"`
	OutdatedCount int                 `json:"outdatedCount"`
	MissingCount  int                 `json:"missingCount"`
	UnpinnedCount int                 `json:"unpinnedCount"`
	GeneratedAt   string              `json:"generatedAt"`
	Warning       string              `json:"warning,omitempty"`
}

// Compare reports how the active pins, installed versions, and latest versions
// relate for every known tool. asdf reports a latest version for every plugin;
// mise's outdated command only reports tools that can move, so an omitted tool
// is treated as current rather than unknown.
func (s *ToolsService) Compare() (VersionComparisonView, error) {
	tools, err := s.List()
	if err != nil {
		return VersionComparisonView{}, err
	}
	latest, latestErr := s.LatestAll()
	view := buildComparisons(tools, latest, s.rt.IsMise())
	if latestErr != nil {
		view.Warning = fmt.Sprintf("Latest versions are unavailable: %s", latestErr)
	}
	return view, nil
}

func buildComparisons(tools []ToolView, latest []asdf.LatestVersion, outdatedOnly bool) VersionComparisonView {
	latestByTool := map[string]asdf.LatestVersion{}
	for _, item := range latest {
		name := strings.ToLower(strings.TrimSpace(item.Tool))
		if name == "" {
			continue
		}
		latestByTool[name] = item
	}

	view := VersionComparisonView{
		Items:       make([]VersionComparison, 0, len(tools)),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, tool := range tools {
		pinned := append([]string{}, tool.Current...)
		installed := append([]string{}, tool.Installed...)
		item := VersionComparison{
			Tool:      tool.Name,
			Pinned:    pinned,
			Installed: installed,
		}

		known, hasLatest := latestByTool[strings.ToLower(tool.Name)]
		if hasLatest && known.Known && strings.TrimSpace(known.Version) != "" {
			item.Latest = strings.TrimSpace(known.Version)
			item.LatestInstalled = containsVersion(installed, item.Latest)
		}

		switch {
		case len(pinned) == 0:
			item.Status = VersionStatusUnpinned
			item.Reason = "No active pin in this context."
		case len(pinned) > 0 && !tool.CurrentInstalled && !tool.System:
			item.Status = VersionStatusMissing
			item.Reason = "The pinned version is not installed."
		case tool.System || containsVersion(pinned, "system"):
			item.Status = VersionStatusCurrent
			item.Reason = "The system version is active."
		case item.Latest != "":
			if containsVersion(pinned, item.Latest) || containsVersion(pinned, "latest") {
				item.Status = VersionStatusCurrent
				item.Reason = "The active pin is the latest version."
			} else {
				item.Status = VersionStatusOutdated
				if item.LatestInstalled {
					item.Reason = fmt.Sprintf("Latest %s is installed but the pin is %s.", item.Latest, strings.Join(pinned, " "))
				} else {
					item.Reason = fmt.Sprintf("Latest %s is available; the pin is %s.", item.Latest, strings.Join(pinned, " "))
				}
			}
		case outdatedOnly:
			item.Status = VersionStatusCurrent
			item.Reason = "No update reported by mise."
		default:
			item.Status = VersionStatusUnknown
			item.Reason = "No latest version was reported."
		}

		switch item.Status {
		case VersionStatusOutdated:
			view.OutdatedCount++
		case VersionStatusMissing:
			view.MissingCount++
		case VersionStatusUnpinned:
			view.UnpinnedCount++
		}
		view.Items = append(view.Items, item)
	}

	sort.Slice(view.Items, func(i, j int) bool {
		return strings.ToLower(view.Items[i].Tool) < strings.ToLower(view.Items[j].Tool)
	})
	return view
}

func containsVersion(versions []string, wanted string) bool {
	for _, version := range versions {
		if strings.EqualFold(version, wanted) {
			return true
		}
	}
	return false
}
