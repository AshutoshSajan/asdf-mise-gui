// Package version exposes the application version to the backend and the
// update checker. Keep the value in sync with the release tag and the build
// metadata; scripts/set-version.mjs updates every copy.
package version

// Version is the application version without a leading "v". It must match the
// tag the release workflow publishes (for example, "0.2.0" for v0.2.0).
var Version = "0.4.0"
