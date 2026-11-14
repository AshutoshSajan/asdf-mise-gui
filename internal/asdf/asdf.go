// Package asdf provides detection of the asdf binary and typed access to its
// command line interface. The GUI never reimplements asdf's resolution logic:
// every mutation and every resolved version comes from the real CLI.
package asdf

import (
	"fmt"
	"regexp"
	"strings"
)

// ToolVersionsFilename is the configuration file asdf reads in a project.
const ToolVersionsFilename = ".tool-versions"

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	versionPattern = regexp.MustCompile(`^[^\s\x00-\x1f\x7f]+$`)
)

// ValidateName checks a tool or plugin name before it becomes an argv element.
// It intentionally rejects anything starting with a dash so a name can never be
// interpreted as a command flag.
func ValidateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid %s name %q: use letters, numbers, dots, dashes, plus signs or underscores", kind, name)
	}
	return nil
}

// ValidateVersion checks a single version string. Versions are passed as
// separate argv elements, never through a shell.
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version is required")
	}
	if strings.HasPrefix(version, "-") {
		return fmt.Errorf("invalid version %q: must not start with a dash", version)
	}
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	if len(version) > 512 {
		return fmt.Errorf("version is too long")
	}
	return nil
}

// ValidateURL checks a plugin repository URL.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	if strings.HasPrefix(rawURL, "-") {
		return fmt.Errorf("invalid URL: must not start with a dash")
	}
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL is too long")
	}
	for _, r := range rawURL {
		if r <= 0x20 || r == 0x7f {
			return fmt.Errorf("invalid URL: contains whitespace or control characters")
		}
	}
	return nil
}

// QuoteCommand renders an argv slice for display. It is never executed.
func QuoteCommand(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if arg == "" {
			quoted[i] = `""`
			continue
		}
		if strings.ContainsAny(arg, " \t\n\"'`$&;|<>()*?[]{}!") {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
			continue
		}
		quoted[i] = arg
	}
	return strings.Join(quoted, " ")
}
