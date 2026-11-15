# Agent Instructions

## Branch and pull request policy

- **Never push directly to `master`.** Every change, including docs, version
  bumps, and release fixes, goes through a pull request.
- Create a topic branch from an up-to-date `master`, push the branch, and open a
  PR with `gh pr create`.
- Wait for CI to pass, then merge the PR (squash or merge commit, matching the
  repository's existing history). Do not fast-forward `master` locally and push.
- Tags and releases are cut from `master` only after the PR is merged and CI is
  green.

## Local verification

Run the checks that CI runs before opening a PR:

```bash
gofmt -l internal main.go
go vet -tags gtk3 ./internal/... .
go test -tags gtk3 -race ./internal/... .
npm --prefix frontend run build
```

Local machines may only have GTK3 headers; release CI builds against GTK4.

## Version bumps

Use the single version script so Go, npm, Linux packaging, and macOS plists stay
in sync:

```bash
node scripts/set-version.mjs <semver>
```

## Release assets

The updater matches AppImage assets by platform and architecture. Keep the
`asdf-mise-gui-linux-<arch>.AppImage` naming and the `checksums.txt` sidecar in
`.github/workflows/release.yml`, or AppImage self-update stops working.
